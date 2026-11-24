package metrics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if err := collector.reconcileStateSnapshot(ctx, store, ScopeAll, nil, known, false); err != nil {
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
