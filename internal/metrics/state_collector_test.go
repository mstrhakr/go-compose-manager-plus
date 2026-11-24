package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestStateHistoryRunsWithoutNumericStats(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	settingsStore := NewSettingsStore(db.SQL)
	stats, err := settingsStore.Save(ctx, Settings{
		Profile: ProfileLow, Scope: ScopeActivated, Interval: 15 * time.Second,
		StorageLimit: 10 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	sources, err := settingsStore.LoadSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources.StateEnabled = true
	sources.StateScope = ScopeAll
	if _, _, err := settingsStore.SaveConfiguration(ctx, stats, sources); err != nil {
		t.Fatal(err)
	}

	id := strings.Repeat("d", 64)
	listener, socket := newDockerSocket(t)
	eventTime := time.Now().UTC()
	var statsRequested atomic.Bool
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/web"],"State":"running","Labels":{}}]`, id)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["CMD","true"]}},"State":{"Status":"running","Health":{"Status":"healthy"},"StartedAt":%q}}`,
				id, eventTime.Add(-time.Minute).Format(time.RFC3339Nano))
		case "/v1.40/containers/" + id + "/stats":
			statsRequested.Store(true)
			http.Error(w, "not expected", http.StatusInternalServerError)
		case "/v1.40/events":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("event response does not support streaming")
				return
			}
			fmt.Fprintf(w, `{"Type":"container","Action":"start","Actor":{"ID":%q,"Attributes":{}},"timeNano":%d}`+"\n", id, eventTime.UnixNano())
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	collector, err := NewCollector(t.TempDir(), settingsStore, discovery.NewStore(db.SQL), runtime.NewClient(socket),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- collector.Run(runCtx) }()
	defer func() {
		cancel()
		if err := <-done; err != context.Canceled {
			t.Errorf("collector shutdown = %v", err)
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		states, stateErr := collector.SelectStates(ctx, id, eventTime.Add(-time.Hour), time.Now().UTC(), 10)
		events, eventErr := collector.SelectTimelineEvents(ctx, id, eventTime.Add(-time.Hour), time.Now().UTC(), 10)
		if stateErr != nil || eventErr != nil {
			t.Fatalf("read state/event history: %v / %v", stateErr, eventErr)
		}
		if len(states) != 0 && len(events) == 1 {
			if len(states) != 1 || states[0].State != "running" || states[0].Health != HealthHealthy {
				t.Fatalf("state observations=%#v", states)
			}
			if statsRequested.Load() {
				t.Fatal("state-only collection requested numeric Docker stats")
			}
			if !collector.Status().EventConnected {
				t.Fatalf("Stats Off reset discarded the active state event connection: %#v", collector.Status())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("independent state capture did not persist: %#v", collector.Status())
}

func TestStateReconnectRecordsGapAndObservedSnapshot(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("e", 64)
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/web"],"State":"running","Labels":{}}]`, id)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["CMD","true"]}},"State":{"Status":"running","Health":{"Status":"healthy"}}}`, id)
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Second)
	initial := State{ContainerID: id, Time: base, State: "running", Health: HealthHealthy}
	if err := store.InsertStates(ctx, time.Now().UTC(), []State{initial}); err != nil {
		t.Fatal(err)
	}
	known := map[string]State{id: initial}
	gapStart := time.Now().UTC()
	if err := store.StartStateGaps(ctx, []string{id}, gapStart); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, known); err != nil {
		t.Fatal(err)
	}
	history, err := store.SelectStateHistory(ctx, id, base.Add(-time.Second), time.Now().UTC().Add(time.Second), 10)
	if err != nil || len(history.States) != 2 || len(history.Gaps) != 1 {
		t.Fatalf("state reconnect history = %#v, error %v", history, err)
	}
	last := history.States[len(history.States)-1]
	if !last.ReconnectObserved || last.Health != HealthHealthy ||
		!history.Gaps[0].End.After(history.Gaps[0].Start) {
		t.Fatalf("reconnect observation/gap = %#v", history)
	}
}

func TestStateReconciliationKeepsSharedGapUntilFailureFreeRestartSnapshot(t *testing.T) {
	ctx := context.Background()
	firstID, secondID := strings.Repeat("b", 64), strings.Repeat("c", 64)
	firstOK, secondOK := atomic.Bool{}, atomic.Bool{}
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"State":"running","Labels":{}},{"Id":%q,"State":"paused","Labels":{}}]`, firstID, secondID)
		case "/v1.40/containers/" + firstID + "/json":
			if !firstOK.Load() {
				http.Error(w, "inspection unavailable", http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"running"}}`, firstID)
		case "/v1.40/containers/" + secondID + "/json":
			if !secondOK.Load() {
				http.Error(w, "inspection unavailable", http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"paused"}}`, secondID)
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Minute)
	if err := store.InsertStates(ctx, time.Now().UTC(), []State{
		{ContainerID: firstID, Time: base, State: "running", Health: HealthNone},
		{ContainerID: secondID, Time: base, State: "paused", Health: HealthNone},
	}); err != nil {
		t.Fatal(err)
	}
	gapStart := time.Now().UTC().Add(-time.Second)
	if err := store.StartGlobalStateGap(ctx, gapStart); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	known := make(map[string]State)
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, known); err == nil {
		t.Fatal("all-failed inspection reconciliation reported success")
	}
	if open, err := store.HasOpenGlobalStateGap(ctx); err != nil || !open {
		t.Fatalf("all-failed reconciliation closed shared gap: open=%t err=%v", open, err)
	}
	for _, id := range []string{firstID, secondID} {
		if open, err := store.HasOpenStateGap(ctx, id); err != nil || !open {
			t.Fatalf("restart did not preserve failed-target gap for %s: open=%t err=%v", id, open, err)
		}
	}

	firstOK.Store(true)
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, known); err == nil {
		t.Fatal("partially successful inspection reconciliation reported success")
	}
	if open, err := store.HasOpenGlobalStateGap(ctx); err != nil || !open {
		t.Fatalf("first successful target closed shared gap: open=%t err=%v", open, err)
	}
	if open, err := store.HasOpenStateGap(ctx, firstID); err != nil || open {
		t.Fatalf("successful target retained its local gap: open=%t err=%v", open, err)
	}
	firstHistory, err := store.SelectStates(ctx, firstID, base.Add(-time.Second), time.Now().UTC().Add(time.Second), 10)
	if err != nil || len(firstHistory) != 2 || !firstHistory[1].ReconnectObserved {
		t.Fatalf("first reconnect observation = %#v, error %v", firstHistory, err)
	}

	secondOK.Store(true)
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, known); err != nil {
		t.Fatal(err)
	}
	if open, err := store.HasOpenGlobalStateGap(ctx); err != nil || open {
		t.Fatalf("failure-free reconciliation did not close shared gap: open=%t err=%v", open, err)
	}
	if open, err := store.HasOpenStateGap(ctx, secondID); err != nil || open {
		t.Fatalf("successful target retained its local gap: open=%t err=%v", open, err)
	}
	firstHistory, err = store.SelectStates(ctx, firstID, base.Add(-time.Second), time.Now().UTC().Add(time.Second), 10)
	if err != nil || len(firstHistory) != 2 {
		t.Fatalf("repeated refresh duplicated successful reconnect state: %#v, error %v", firstHistory, err)
	}
	secondHistory, err := store.SelectStates(ctx, secondID, base.Add(-time.Second), time.Now().UTC().Add(time.Second), 10)
	if err != nil || len(secondHistory) != 2 || !secondHistory[1].ReconnectObserved {
		t.Fatalf("failed target reconnect observation = %#v, error %v", secondHistory, err)
	}
}

func TestEmptySuccessfulStateReconciliationClosesSharedGap(t *testing.T) {
	ctx := context.Background()
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.40/containers/json" {
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		http.NotFound(w, r)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := store.StartGlobalStateGap(ctx, time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, map[string]State{}); err != nil {
		t.Fatal(err)
	}
	if open, err := store.HasOpenGlobalStateGap(ctx); err != nil || open {
		t.Fatalf("failure-free empty reconciliation did not close shared gap: open=%t err=%v", open, err)
	}
}

func TestStateOnlyLifecycleEventsCaptureCreatedAndPausedObservations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id := strings.Repeat("a", 64)
	var inspectionState atomic.Int64
	var inspections atomic.Int64
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			_, _ = fmt.Fprint(w, `[]`)
		case "/v1.40/containers/" + id + "/json":
			state := "created"
			if inspectionState.Load() != 0 {
				state = "paused"
			}
			inspections.Add(1)
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":%q}}`, id, state)
		case "/v1.40/events":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("Docker event response does not support streaming")
				return
			}
			writeEvent := func(action string, at time.Time) {
				fmt.Fprintf(w, `{"Type":"container","Action":%q,"Actor":{"ID":%q},"timeNano":%d}`+"\n", action, id, at.UnixNano())
				flusher.Flush()
			}
			writeEvent("create", time.Now().UTC().Add(-time.Second))
			if !waitForAtomicValue(r.Context(), &inspections, 1) {
				return
			}
			inspectionState.Store(1)
			writeEvent("pause", time.Now().UTC().Add(-500*time.Millisecond))
			if !waitForAtomicValue(r.Context(), &inspections, 2) {
				return
			}
			writeEvent("destroy", time.Now().UTC().Add(-250*time.Millisecond))
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	done := make(chan struct{})
	go func() {
		collector.watchStateEvents(ctx, store, ScopeAll)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		windowEnd := time.Now().UTC().Add(time.Second)
		windowStart := windowEnd.Add(-time.Hour)
		history, historyErr := store.SelectStateHistory(ctx, id, windowStart, windowEnd, 10)
		events, eventErr := store.SelectEvents(ctx, id, windowStart, windowEnd, 10)
		if historyErr != nil || eventErr != nil {
			t.Fatalf("read lifecycle history: state=%v events=%v", historyErr, eventErr)
		}
		if len(history.States) == 2 && len(events) == 3 {
			if history.States[0].State != "created" || history.States[1].State != "paused" ||
				!history.States[0].Time.After(events[0].Time) || !history.States[1].Time.After(events[1].Time) {
				t.Fatalf("lifecycle observations invented event-time states: states=%#v events=%#v", history.States, events)
			}
			if events[0].Kind != EventCreate || events[1].Kind != EventPause || events[2].Kind != EventDestroy {
				t.Fatalf("lifecycle event kinds = %#v", events)
			}
			cancel()
			<-done
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatalf("state-only lifecycle coverage incomplete: inspections=%d status=%#v",
		inspections.Load(), collector.Status())
}

func TestStateOnlyScopeResolutionFailureRemainsVisible(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	idle := make(chan struct{})
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			_, _ = fmt.Fprint(w, `[]`)
		case "/v1.40/events":
			w.(http.Flusher).Flush()
			close(idle)
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{
		docker: runtime.NewClient(socket),
		projects: projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
			return nil, fmt.Errorf("project inventory unavailable")
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	done := make(chan struct{})
	go func() {
		collector.watchStateEvents(ctx, store, ScopeActivated)
		close(done)
	}()
	select {
	case <-idle:
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("state event stream did not connect")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := collector.Status()
		if status.EventConnected && strings.Contains(status.StateProblem, "partially resolved state collection scope") {
			if status.StateCollecting {
				t.Fatalf("scope failure was reported as active collection: %#v", status)
			}
			cancel()
			<-done
			return
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	t.Fatalf("state scope failure was discarded: %#v", collector.Status())
}

func TestStateMaintenanceRecoveryDoesNotClearWatcherFailure(t *testing.T) {
	collector := &Collector{}
	collector.stateWatcherHealthy = true
	collector.status.StateCollecting = true
	collector.status.EventConnected = true
	collector.setStateWatcherProblem(errors.New("activated project scope is unavailable"))
	collector.setStateStorageProblem(errors.New("metrics storage is paused"))
	collector.beginStateOnlySession()
	collector.clearStateStorageProblem()
	status := collector.Status()
	if status.StateCollecting || status.EventConnected || !strings.Contains(status.StateProblem, "activated project scope is unavailable") ||
		strings.Contains(status.StateProblem, "storage: metrics storage is paused") {
		t.Fatalf("storage recovery masked watcher health: %#v", status)
	}
}

func TestStateWatchersRefreshActivatedScopeWithoutReconnect(t *testing.T) {
	for _, mode := range []string{"state-only", "stats-enabled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			id := strings.Repeat("d", 64)
			root := t.TempDir()
			projectPath := filepath.Join(root, "app")
			if err := os.Mkdir(projectPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectPath, "compose.yaml"), []byte("name: app\nservices:\n  web:\n    image: example\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			var active atomic.Bool
			var inspections atomic.Int64
			connected := make(chan struct{})
			var connectedOnce sync.Once
			listener, socket := newDockerSocket(t)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.40/containers/json":
					fmt.Fprintf(w, `[{"Id":%q,"Names":["/web"],"State":"running","Labels":{"com.docker.compose.project":"app"}}]`, id)
				case "/v1.40/containers/" + id + "/json":
					inspections.Add(1)
					fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"running"}}`, id)
				case "/v1.40/events":
					flusher := w.(http.Flusher)
					flusher.Flush()
					connectedOnce.Do(func() { close(connected) })
					<-r.Context().Done()
				default:
					http.NotFound(w, r)
				}
			}))
			server.Listener = listener
			server.Start()
			defer server.Close()
			store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			collector := &Collector{
				docker: runtime.NewClient(socket),
				projects: projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
					if !active.Load() {
						return nil, nil
					}
					return []discovery.Project{{Root: root, Path: projectPath, Active: true, Available: true}}, nil
				}),
				logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
				stateRefreshInterval: 10 * time.Millisecond,
			}
			done := make(chan struct{})
			go func() {
				if mode == "state-only" {
					collector.watchStateEvents(ctx, store, ScopeActivated)
				} else {
					collector.watchEvents(ctx, store, &eventContinuity{}, true, ScopeActivated)
				}
				close(done)
			}()
			select {
			case <-connected:
			case <-time.After(5 * time.Second):
				cancel()
				<-done
				t.Fatal("event stream did not connect")
			}
			active.Store(true)
			if !waitForAtomicValue(ctx, &inspections, 1) {
				cancel()
				<-done
				t.Fatal("newly activated container was not reconciled while the event stream stayed open")
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				states, err := store.SelectStates(ctx, id, time.Now().UTC().Add(-time.Minute), time.Now().UTC().Add(time.Minute), 10)
				if err != nil {
					cancel()
					<-done
					t.Fatal(err)
				}
				if len(states) != 0 {
					if !collector.Status().StateCollecting {
						t.Fatalf("successful scope reconciliation was not reported active: %#v", collector.Status())
					}
					cancel()
					<-done
					return
				}
				time.Sleep(time.Millisecond)
			}
			cancel()
			<-done
			t.Fatalf("activated scope refresh did not persist state: %#v", collector.Status())
		})
	}
}

func waitForAtomicValue(ctx context.Context, value *atomic.Int64, minimum int64) bool {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for value.Load() < minimum {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-ticker.C:
		}
	}
	return true
}
