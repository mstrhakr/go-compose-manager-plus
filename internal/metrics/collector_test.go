package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type projectReaderFunc func(context.Context) ([]discovery.Project, error)

func (f projectReaderFunc) Projects(ctx context.Context) ([]discovery.Project, error) {
	return f(ctx)
}

func newCollectorTestStore(t *testing.T) (*ManagedStore, time.Time) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	store, err := OpenManaged(context.Background(), t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store, now
}

func newDockerSocket(t *testing.T) (net.Listener, string) {
	t.Helper()
	if goruntime.GOOS == "windows" {
		t.Skip("Docker Unix socket integration test is not supported on Windows")
	}
	dir, err := os.MkdirTemp(".", ".docker-test-")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		_ = os.RemoveAll(dir)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove temporary Docker socket directory: %v", err)
		}
	})
	return listener, socket
}

func newCollectorDocker(t *testing.T, containers []runtime.Container) *runtime.Client {
	t.Helper()
	listener, socket := newDockerSocket(t)
	var statsCount atomic.Int64
	started := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprint(w, `[`)
			for i, container := range containers {
				if i != 0 {
					fmt.Fprint(w, `,`)
				}
				fmt.Fprintf(w, `{"Id":%q,"Names":[%q],"State":%q,"Labels":{"com.docker.compose.project":%q,"com.docker.compose.service":%q}}`,
					container.ID, "/"+container.Name, container.State, container.Project, container.Service)
			}
			fmt.Fprint(w, `]`)
		default:
			const pathPrefix = "/v1.40/containers/"
			if !strings.HasPrefix(r.URL.Path, pathPrefix) {
				http.NotFound(w, r)
				return
			}
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, pathPrefix), "/")
			if len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			id, action := parts[0], parts[1]
			var container *runtime.Container
			for i := range containers {
				if containers[i].ID == id {
					container = &containers[i]
					break
				}
			}
			if container == nil {
				http.NotFound(w, r)
				return
			}
			switch action {
			case "json":
				fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":%q,"StartedAt":%q}}`,
					id, container.State, started)
			case "stats":
				n := statsCount.Add(1)
				now := time.Now().UTC().Add(time.Duration(n) * time.Second)
				fmt.Fprintf(w, `{"read":%q,"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":1},"memory_stats":{"usage":100,"limit":1024},"networks":{"eth0":{"rx_bytes":%d,"tx_bytes":%d}},"blkio_stats":{"io_service_bytes_recursive":[{"major":8,"minor":0,"op":"Read","value":%d},{"major":8,"minor":0,"op":"Write","value":%d}]}}`,
					now.Format(time.RFC3339Nano), n*1000, n*10000, n*100, n*200, n*300, n*400)
			default:
				http.NotFound(w, r)
			}
		}
	}))
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return runtime.NewClient(socket)
}

func TestCollectorPersistsReadOnlySamplesAndStates(t *testing.T) {
	id := strings.Repeat("a", 64)
	stoppedID := strings.Repeat("b", 64)
	docker := newCollectorDocker(t, []runtime.Container{
		{ID: id, Name: "web", Project: "app", Service: "web", State: "running"},
		{ID: stoppedID, Name: "db", Project: "app", Service: "db", State: "exited"},
	})
	store, now := newCollectorTestStore(t)
	collector := &Collector{docker: docker, projects: projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
		return nil, nil
	}), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	continuity := &eventContinuity{}
	continuity.setConnected(true)
	_, generation := continuity.snapshot()
	settings := Settings{Enabled: true, Scope: ScopeAll, Interval: time.Second, StorageLimit: 10 << 20}
	samplers := make(map[string]*Sampler)
	previousCycleOK := false
	for range 2 {
		if err := collector.collectCycle(context.Background(), settings, store, continuity, samplers, &previousCycleOK, &generation); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := store.Select(context.Background(), id, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(samples) != 2 {
		t.Fatalf("running container samples = %d, %v", len(samples), err)
	}
	if samples[0].Observation.CPUPercent != nil || samples[1].Observation.CPUPercent == nil {
		t.Fatalf("rate baseline behavior = %#v", samples)
	}
	states, err := store.SelectStates(context.Background(), id, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(states) != 1 || states[0].State != "running" {
		t.Fatalf("running states = %#v, %v", states, err)
	}
	stoppedStates, err := store.SelectStates(context.Background(), stoppedID, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(stoppedStates) != 1 || stoppedStates[0].State != "exited" {
		t.Fatalf("all-container state source omitted stopped context: %#v, %v", stoppedStates, err)
	}
}

func TestStatsCollectionKeepsStateScopeIndependentAndOffHidesWrites(t *testing.T) {
	activeID, inactiveID := strings.Repeat("c", 64), strings.Repeat("d", 64)
	root := t.TempDir()
	activePath := filepath.Join(root, "active")
	if err := os.Mkdir(activePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activePath, "compose.yaml"), []byte("name: app\nservices:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	docker := newCollectorDocker(t, []runtime.Container{
		{ID: activeID, Name: "active", Project: "app", Service: "web", State: "running"},
		{ID: inactiveID, Name: "inactive", Project: "other", Service: "db", State: "exited"},
	})
	store, now := newCollectorTestStore(t)
	projects := projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
		return []discovery.Project{{Root: root, Path: activePath, Active: true, Available: true}}, nil
	})
	collector := &Collector{
		docker:   docker,
		projects: projects,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	continuity := &eventContinuity{}
	previousCycleOK := false
	var generation uint64
	settings := Settings{Enabled: true, Scope: ScopeActivated, Interval: 15 * time.Second, StorageLimit: 10 << 20}
	if err := collector.collectCycleWithScopes(context.Background(), settings, ScopeAll, store, continuity,
		make(map[string]*Sampler), &previousCycleOK, &generation, true); err != nil {
		t.Fatal(err)
	}
	activeStates, err := store.SelectStates(context.Background(), activeID, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(activeStates) != 1 {
		t.Fatalf("state source omitted active container: %#v, %v", activeStates, err)
	}
	inactiveStates, err := store.SelectStates(context.Background(), inactiveID, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(inactiveStates) != 1 || inactiveStates[0].State != "exited" {
		t.Fatalf("independent state scope omitted stopped container: %#v, %v", inactiveStates, err)
	}
	samples, err := store.Select(context.Background(), inactiveID, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(samples) != 0 {
		t.Fatalf("stats scope leaked to inactive project: %#v, %v", samples, err)
	}

	offStore, offNow := newCollectorTestStore(t)
	offCollector := &Collector{docker: docker, projects: projects, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	offContinuity := &eventContinuity{}
	previousCycleOK, generation = false, 0
	if err := offCollector.collectCycleWithScopes(context.Background(), settings, ScopeAll, offStore, offContinuity,
		make(map[string]*Sampler), &previousCycleOK, &generation, false); err != nil {
		t.Fatal(err)
	}
	offStates, err := offStore.SelectStates(context.Background(), activeID, offNow.Add(-time.Minute), offNow.Add(time.Minute), 10)
	if err != nil || len(offStates) != 0 {
		t.Fatalf("State Off persisted snapshots from shared stats inspections: %#v, %v", offStates, err)
	}
	var gaps int
	if err := offStore.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM container_state_gaps`).Scan(&gaps); err != nil || gaps != 0 {
		t.Fatalf("State Off persisted observation gaps: %d, %v", gaps, err)
	}
	offSamples, err := offStore.Select(context.Background(), activeID, offNow.Add(-time.Minute), offNow.Add(time.Minute), 10)
	if err != nil || len(offSamples) != 1 {
		t.Fatalf("State Off suppressed shared stats collection: %#v, %v", offSamples, err)
	}
}

func TestStatsEventWatcherAdmitsLifecycleEventsBetweenSampleCycles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id := strings.Repeat("9", 64)
	root := t.TempDir()
	projectPath := filepath.Join(root, "app")
	if err := os.Mkdir(projectPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "compose.yaml"), []byte("name: app\nservices:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, socket := newDockerSocket(t)
	eventTime := time.Now().UTC()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			_, _ = fmt.Fprint(w, `[]`)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"created"}}`, id)
		case "/v1.40/events":
			flusher := w.(http.Flusher)
			fmt.Fprintf(w, `{"Type":"container","Action":"create","Actor":{"ID":%q,"Attributes":{"com.docker.compose.project":"app"}},"timeNano":%d}`+"\n", id, eventTime.UnixNano())
			flusher.Flush()
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
			return []discovery.Project{{Root: root, Path: projectPath, Active: true, Available: true}}, nil
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	done := make(chan struct{})
	go func() {
		collector.watchEvents(ctx, store, &eventContinuity{}, true, ScopeActivated)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		states, stateErr := store.SelectStates(ctx, id, eventTime.Add(-time.Minute), time.Now().UTC().Add(time.Minute), 10)
		events, eventErr := store.SelectEvents(ctx, id, eventTime.Add(-time.Minute), time.Now().UTC().Add(time.Minute), 10)
		if stateErr != nil || eventErr != nil {
			cancel()
			<-done
			t.Fatalf("read event-driven state history: %v / %v", stateErr, eventErr)
		}
		if len(states) == 1 && len(events) == 1 {
			if states[0].State != "created" || events[0].Kind != EventCreate {
				t.Fatalf("between-cycle event history = states %#v, events %#v", states, events)
			}
			cancel()
			<-done
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatalf("event was not admitted independently of sampling snapshots: %#v", collector.Status())
}

func TestCollectorStatusTracksStoragePressureEvictions(t *testing.T) {
	ctx := context.Background()
	store, now := newCollectorTestStore(t)
	id := strings.Repeat("a", 64)
	if err := store.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}); err != nil {
		t.Fatal(err)
	}
	evicted, err := store.store.EvictOldestSamples(ctx, 1)
	if err != nil || evicted.Count != 1 {
		t.Fatalf("evict sample = %#v, %v", evicted, err)
	}
	collector := &Collector{}
	if err := collector.refreshEvictions(ctx, store); err != nil {
		t.Fatal(err)
	}
	if got := collector.Status().Evictions; got != evicted {
		t.Fatalf("collector eviction status = %#v, want %#v", got, evicted)
	}
}

func TestCollectorActivatedScopeMatchesSharedComposeName(t *testing.T) {
	root := t.TempDir()
	activePath := filepath.Join(root, "active")
	inactivePath := filepath.Join(root, "inactive")
	for _, path := range []string{activePath, inactivePath} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "compose.yaml"), []byte("name: shared\nservices:\n  web:\n    image: example\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	id := strings.Repeat("c", 64)
	otherID := strings.Repeat("d", 64)
	docker := newCollectorDocker(t, []runtime.Container{
		{ID: id, Name: "shared-web", Project: "shared", Service: "web", State: "running"},
		{ID: otherID, Name: "other-web", Project: "other", Service: "web", State: "running"},
	})
	store, now := newCollectorTestStore(t)
	collector := &Collector{
		docker: docker,
		projects: projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
			return []discovery.Project{
				{Path: activePath, Root: root, Active: true, Available: true},
				{Path: inactivePath, Root: root, Active: false, Available: true},
			}, nil
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	continuity := &eventContinuity{}
	previousCycleOK := false
	var generation uint64
	settings := Settings{Enabled: true, Scope: ScopeActivated, Interval: time.Second, StorageLimit: 10 << 20}
	if err := collector.collectCycle(context.Background(), settings, store, continuity, make(map[string]*Sampler), &previousCycleOK, &generation); err != nil {
		t.Fatal(err)
	}
	states, err := store.SelectStates(context.Background(), id, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(states) != 1 {
		t.Fatalf("shared Compose project not included: %#v, %v", states, err)
	}
	otherStates, err := store.SelectStates(context.Background(), otherID, now.Add(-time.Minute), now.Add(time.Minute), 10)
	if err != nil || len(otherStates) != 0 {
		t.Fatalf("unactivated Compose project included: %#v, %v", otherStates, err)
	}
}

func TestDisabledCollectorDoesNotOpenMetricsDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE metrics_settings (
		id INTEGER PRIMARY KEY,
		enabled INTEGER NOT NULL,
		profile TEXT NOT NULL,
		scope TEXT NOT NULL,
		interval_ns INTEGER NOT NULL,
		storage_limit_bytes INTEGER NOT NULL,
		revision INTEGER NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE state_settings (id INTEGER PRIMARY KEY, enabled INTEGER, scope TEXT, revision INTEGER)`,
		`INSERT INTO state_settings VALUES (1, 1, 'all', 1)`,
		`CREATE TABLE log_settings (
			id INTEGER PRIMARY KEY, enabled INTEGER, profile TEXT, scope TEXT,
			delivery_preset TEXT, storage_limit_bytes INTEGER, revision INTEGER, configured INTEGER
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := t.TempDir()
	collector, err := NewCollector(dataDir, NewSettingsStore(db), projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
		return nil, nil
	}), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := collector.Run(ctx); err != context.DeadlineExceeded {
		t.Fatalf("collector shutdown = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "metrics.db")); !os.IsNotExist(err) {
		t.Fatalf("disabled collector opened metrics database: %v", err)
	}
}

func TestCollectorRunOptInPersistsEventsAndStopsOnDisable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE metrics_settings (
		id INTEGER PRIMARY KEY,
		enabled INTEGER NOT NULL,
		profile TEXT NOT NULL,
		scope TEXT NOT NULL,
		interval_ns INTEGER NOT NULL,
		storage_limit_bytes INTEGER NOT NULL,
		revision INTEGER NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE state_settings (id INTEGER PRIMARY KEY, enabled INTEGER, scope TEXT, revision INTEGER)`,
		`INSERT INTO state_settings VALUES (1, 1, 'all', 1)`,
		`CREATE TABLE log_settings (
			id INTEGER PRIMARY KEY, configured INTEGER, enabled INTEGER, profile TEXT, scope TEXT,
			delivery_preset TEXT, storage_limit_bytes INTEGER, revision INTEGER
		)`,
		`CREATE TABLE collection_settings (
			id INTEGER PRIMARY KEY, profile TEXT, restore_stats INTEGER, restore_state INTEGER,
			restore_logs INTEGER, revision INTEGER
		)`,
		`INSERT INTO collection_settings VALUES (1, 'off', 0, 0, 0, 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	settingsStore := NewSettingsStore(db)
	settings, err := settingsStore.Save(context.Background(), Settings{
		Enabled: true, Profile: ProfileCustom, Scope: ScopeAll, Interval: 50 * time.Millisecond,
		StorageLimit: 10 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("e", 64)
	listener, socket := newDockerSocket(t)
	var statsCount atomic.Int64
	eventTime := time.Now().UTC()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/web"],"State":"running","Labels":{"com.docker.compose.project":"app","com.docker.compose.service":"web"}}]`, id)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"running","StartedAt":%q}}`, id, eventTime.Add(-time.Minute).Format(time.RFC3339Nano))
		case "/v1.40/containers/" + id + "/stats":
			n := statsCount.Add(1)
			fmt.Fprintf(w, `{"read":%q,"cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":1},"memory_stats":{"usage":100,"limit":1024}}`,
				time.Now().UTC().Format(time.RFC3339Nano), n*1000, n*10000)
		case "/v1.40/events":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("event response does not support streaming")
				return
			}
			fmt.Fprintln(w, `{"Type":"container","Action":"oom","Actor":{"ID":"`+id+`","Attributes":{"com.docker.compose.project":"app"}},"timeNano":`+fmt.Sprint(eventTime.UnixNano())+`}`)
			flusher.Flush()
			fmt.Fprintln(w, `{"Type":"container","Action":"oom","Actor":{"ID":"`+id+`","Attributes":{"com.docker.compose.project":"app"}},"timeNano":`+fmt.Sprint(eventTime.UnixNano())+`}`)
			flusher.Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	dataDir := t.TempDir()
	collector, err := NewCollector(dataDir, settingsStore, projectReaderFunc(func(context.Context) ([]discovery.Project, error) {
		return nil, nil
	}), runtime.NewClient(socket), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- collector.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for (statsCount.Load() < 3 || !collector.Status().EventConnected) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if statsCount.Load() < 3 || !collector.Status().EventConnected {
		cancel()
		t.Fatalf("collector did not become active: status=%#v stats=%d", collector.Status(), statsCount.Load())
	}
	settings.Enabled = false
	if _, err := settingsStore.Save(context.Background(), settings); err != nil {
		cancel()
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for collector.Status().Enabled && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if collector.Status().Enabled {
		cancel()
		t.Fatalf("collector did not stop after opt-out: %#v", collector.Status())
	}
	stoppedAt := statsCount.Load()
	time.Sleep(100 * time.Millisecond)
	if statsCount.Load() != stoppedAt {
		cancel()
		t.Fatal("collector continued sampling after opt-out")
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("collector shutdown = %v", err)
	}
	store, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	samples, err := store.Select(context.Background(), id, eventTime.Add(-time.Minute), time.Now().Add(time.Minute), 20)
	if err != nil || len(samples) < 3 {
		t.Fatalf("persisted samples = %d, %v", len(samples), err)
	}
	events, err := store.SelectEvents(context.Background(), id, eventTime.Add(-time.Second), eventTime.Add(time.Second), 20)
	if err != nil || len(events) != 1 || events[0].Kind != EventOOM {
		t.Fatalf("replayed events = %#v, %v", events, err)
	}
}

func TestCollectorChartReadsUseOnlyActiveManagedStore(t *testing.T) {
	ctx := context.Background()
	collector := &Collector{}
	now := time.Now().UTC().Truncate(time.Millisecond)
	got, err := collector.SelectChartSeries(ctx, "", now.Add(-time.Minute), now, 30*time.Second)
	if err != nil || got != nil {
		t.Fatalf("inactive chart read = %#v, error = %v", got, err)
	}

	store, err := OpenManaged(ctx, t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	cpu := 35.0
	if err := store.Insert(ctx, now, []Sample{{
		ContainerID: id, Observation: runtime.Observations{Time: now.Add(-20 * time.Second), CPUPercent: &cpu},
	}}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	collector.storeMu.Lock()
	collector.store = store
	collector.storeMu.Unlock()
	got, err = collector.SelectChartSeries(ctx, id, now.Add(-time.Minute), now, 30*time.Second)
	if err != nil || len(got) != 1 || got[0].ContainerID != id || len(got[0].Points) != 2 ||
		got[0].Points[1].CPUPercent == nil || *got[0].Points[1].CPUPercent != cpu {
		collector.storeMu.Lock()
		collector.store = nil
		collector.storeMu.Unlock()
		_ = store.Close()
		t.Fatalf("active chart read = %#v, error = %v", got, err)
	}
	collector.storeMu.Lock()
	collector.store = nil
	if err := store.Close(); err != nil {
		collector.storeMu.Unlock()
		t.Fatal(err)
	}
	collector.storeMu.Unlock()
	got, err = collector.SelectChartSeries(ctx, id, now.Add(-time.Minute), now, 30*time.Second)
	if err != nil || got != nil {
		t.Fatalf("post-close chart read = %#v, error = %v", got, err)
	}
}
