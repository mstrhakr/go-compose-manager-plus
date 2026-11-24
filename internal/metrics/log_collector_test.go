package metrics

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestCollectorStreamsAndPersistsContainerLogs(t *testing.T) {
	ctx := context.Background()
	db, err := auth.OpenDatabase(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.SQL.Close()
	settingsStore := NewSettingsStore(db.SQL)
	metricsSettings, err := settingsStore.Save(ctx, Settings{
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
	sources.Logs = LogSettings{
		Configured: true, Enabled: true, Profile: ProfileHigh, Scope: ScopeAll,
		DeliveryPreset: ProfileHigh, StorageLimit: 10 << 20, Revision: sources.Logs.Revision,
	}
	if _, _, err := settingsStore.SaveConfiguration(ctx, metricsSettings, sources); err != nil {
		t.Fatal(err)
	}

	id := strings.Repeat("c", 64)
	listener, socket := newDockerSocket(t)
	logTime := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/web"],"State":"running","Labels":{}}]`, id)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":false,"Healthcheck":{"Test":["NONE"]}},"State":{"Status":"running","StartedAt":%q}}`,
				id, logTime.Add(-time.Minute).Format(time.RFC3339Nano))
		case "/v1.40/containers/" + id + "/logs":
			if r.URL.Query().Get("follow") != "true" || r.URL.Query().Get("stdout") != "true" ||
				r.URL.Query().Get("stderr") != "true" || r.URL.Query().Get("stdin") != "" {
				t.Errorf("unexpected Docker log query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("Docker log response does not support streaming")
				return
			}
			line := []byte(logTime.Format(time.RFC3339Nano) + " original\n")
			var header [8]byte
			header[0] = 1
			binary.BigEndian.PutUint32(header[4:], uint32(len(line)))
			if _, err := w.Write(header[:]); err != nil {
				return
			}
			if _, err := w.Write(line); err != nil {
				return
			}
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
		page, err := collector.SelectLogs(ctx, id, logTime.Add(-time.Second), time.Now().UTC(), maxLogBatch)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Logs) == 1 {
			for _, log := range page.Logs {
				if log.Time != logTime || log.Message != "original\n" || log.Stream != runtime.LogStdout {
					t.Fatalf("stored Docker log = %#v", log)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("collector did not persist a single live Docker log before cancellation: %#v", collector.Status())
}

func TestPeriodicCustomScopeCapturesStoppedBacklog(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("b", 64)
	listener, socket := newDockerSocket(t)
	logTime := time.Now().UTC().Add(-30 * time.Second).Truncate(time.Second)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/stopped"],"State":"exited","Labels":{}}]`, id)
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"exited"}}`, id)
		case "/v1.40/containers/" + id + "/logs":
			if r.URL.Query().Get("follow") != "false" || r.URL.Query().Get("until") == "" {
				t.Errorf("expected a finite stopped-container backlog read: %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprintf(w, "%s from-stopped\n", logTime.Format(time.RFC3339Nano))
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	settings := LogSettings{
		Configured: true, Enabled: true, Profile: ProfileCustom, Scope: ScopeAll,
		DeliveryPreset: ProfileLow, StorageLimit: 10 << 20,
	}
	if err := collector.collectLogBacklog(ctx, settings, store, make(map[string]time.Time)); err != nil {
		t.Fatal(err)
	}
	page, err := store.SelectLogs(ctx, id, logTime.Add(-time.Second), time.Now().UTC(), 10)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].Message != "from-stopped\n" || page.Logs[0].Stream != runtime.LogTTY {
		t.Fatalf("stopped backlog=%#v error=%v", page, err)
	}
}

func TestReadAndPersistLogsAcknowledgesCommittedBatchWithoutRetry(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("f", 64)
	now := time.Now().UTC()
	logTime := now.Add(-time.Second)
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":false},"State":{"Status":"running"}}`, id)
		case "/v1.40/containers/" + id + "/logs":
			line := []byte(logTime.Format(time.RFC3339Nano) + " committed\n")
			var header [8]byte
			header[0] = 1
			binary.BigEndian.PutUint32(header[4:], uint32(len(line)))
			_, _ = w.Write(header[:])
			_, _ = w.Write(line)
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.store.db.ExecContext(ctx, `CREATE TRIGGER fail_log_cleanup BEFORE DELETE ON pending_write
		BEGIN SELECT RAISE(FAIL, 'injected cleanup failure'); END`); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket)}
	err = collector.readAndPersistLogs(ctx, store, id, runtime.LogOptions{Since: now.Add(-time.Minute), Follow: true}, nil, nil)
	if !errors.Is(err, ErrWriteCommitted) {
		t.Fatalf("committed log outcome = %v", err)
	}
	page, err := store.SelectLogs(ctx, id, now.Add(-time.Minute), now, 10)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].Message != "committed\n" {
		t.Fatalf("acknowledged committed batch was replayed: %#v, %v", page, err)
	}
}

func TestReadAndPersistLogsDropsRecordsExpiredAtFlush(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("a", 64)
	since := time.Now().UTC().Add(-time.Hour)
	until := since.Add(time.Hour)
	freshAt := time.Now().UTC().Add(-time.Minute)
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"exited"}}`, id)
		case "/v1.40/containers/" + id + "/logs":
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "%s expired\n%s fresh\n", since.Format(time.RFC3339Nano), freshAt.Format(time.RFC3339Nano))
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	collector := &Collector{docker: runtime.NewClient(socket)}
	if err := collector.readAndPersistLogs(ctx, store, id, runtime.LogOptions{Since: since, Until: until}, nil, nil); err != nil {
		t.Fatal(err)
	}
	page, err := store.SelectLogs(ctx, id, since, until, 10)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].Message != "fresh\n" {
		t.Fatalf("flush-time history window = %#v, %v", page, err)
	}
}

func TestLiveReconnectResumesFromAcknowledgedDockerTimestamp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	id := strings.Repeat("d", 64)
	logTime := time.Now().UTC().Add(-10 * time.Second)
	var requests atomic.Int64
	secondSince := make(chan string, 1)
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/" + id + "/json":
			fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"running"}}`, id)
		case "/v1.40/containers/" + id + "/logs":
			if requests.Add(1) == 1 {
				fmt.Fprintf(w, "%s acknowledged\n", logTime.Format(time.RFC3339Nano))
				return
			}
			secondSince <- r.URL.Query().Get("since")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{docker: runtime.NewClient(socket), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cursors := &logCursorMap{times: make(map[string]time.Time)}
	done := make(chan struct{})
	go func() {
		collector.followContainerLogs(ctx, store, id, cursors, &logFollowerHealth{})
		close(done)
	}()
	select {
	case since := <-secondSince:
		if since != fmt.Sprint(logTime.Unix()) {
			cancel()
			<-done
			t.Fatalf("reconnect cursor advanced beyond acknowledged Docker time: got %q want %d", since, logTime.Unix())
		}
	case <-time.After(4 * time.Second):
		cancel()
		<-done
		t.Fatal("live follower did not reconnect")
	}
	cancel()
	<-done
}

func TestPeriodicBacklogFailureDoesNotBlockOtherContainers(t *testing.T) {
	ctx := context.Background()
	failedID, readableID := strings.Repeat("b", 64), strings.Repeat("c", 64)
	now := time.Now().UTC()
	lineAt := now.Add(-30 * time.Second)
	listener, socket := newDockerSocket(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.40/containers/json":
			fmt.Fprintf(w, `[{"Id":%q,"Names":["/unreadable"],"State":"exited","Labels":{}},{"Id":%q,"Names":["/readable"],"State":"exited","Labels":{}}]`, failedID, readableID)
		case "/v1.40/containers/" + failedID + "/json", "/v1.40/containers/" + readableID + "/json":
			if strings.Contains(r.URL.Path, failedID) {
				fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"exited"}}`, failedID)
			} else {
				fmt.Fprintf(w, `{"Id":%q,"Config":{"Tty":true},"State":{"Status":"exited"}}`, readableID)
			}
		case "/v1.40/containers/" + failedID + "/logs":
			http.Error(w, "unreadable", http.StatusInternalServerError)
		case "/v1.40/containers/" + readableID + "/logs":
			fmt.Fprintf(w, "%s readable\n", lineAt.Format(time.RFC3339Nano))
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	collector := &Collector{docker: runtime.NewClient(socket)}
	cursors := make(map[string]time.Time)
	err = collector.collectLogBacklog(ctx, LogSettings{Scope: ScopeAll}, store, cursors)
	if err == nil {
		t.Fatal("unreadable container was not reported")
	}
	if cursors[failedID] != (time.Time{}) || cursors[readableID].IsZero() {
		t.Fatalf("per-container cursors = %#v", cursors)
	}
	windowEnd := time.Now().UTC()
	page, err := store.SelectLogs(ctx, readableID, windowEnd.Add(-time.Hour), windowEnd, 10)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].Message != "readable\n" {
		t.Fatalf("readable container was blocked by sibling failure: %#v, %v", page, err)
	}
	if collector.Status().LogsProblem == "" || collector.Status().LogsCollecting {
		t.Fatalf("partial failure status was success-shaped: %#v", collector.Status())
	}
}

func TestLiveLogRefreshPreservesFollowerAndStorageFailures(t *testing.T) {
	ctx := context.Background()
	store, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{}
	health := &logFollowerHealth{problem: "live follower disconnected"}
	tasks := map[string]logFollowTask{"container": {health: health}}
	if err := collector.updateLiveLogStatus(ctx, store, 1, nil, tasks, &sync.Mutex{}, time.Now()); err == nil {
		t.Fatal("follower failure was not surfaced")
	}
	if status := collector.Status(); status.LogsCollecting || status.LogsProblem != "live follower disconnected" {
		t.Fatalf("refresh erased follower failure: %#v", status)
	}
	health.set("")
	store.limit = 1
	if err := store.Maintain(ctx, time.Now().UTC()); err == nil {
		t.Fatal("tiny storage target did not pause")
	}
	if err := collector.updateLiveLogStatus(ctx, store, 1, nil, tasks, &sync.Mutex{}, time.Now()); err == nil {
		t.Fatal("storage failure was not surfaced")
	}
	if status := collector.Status(); status.LogsCollecting || status.LogsProblem == "" {
		t.Fatalf("refresh erased storage failure: %#v", status)
	}
}
