package metrics

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	docker "github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestManagedLogsSeparateDatabaseAndPreserveOriginalBytes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	legacy, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	record := Log{ContainerID: id, Time: now.Add(-time.Second), ReceivedAt: now,
		Stream: docker.LogStderr, Message: "  original\r\nmultiline\x00\xff", Truncated: true}
	if err := legacy.store.InsertLogs(ctx, now, []Log{record}); err != nil {
		t.Fatal(err)
	}
	logs, err := OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := logs.InsertLogs(ctx, now, []Log{record, record}); err != nil {
		t.Fatal(err)
	}
	if err := legacy.InsertLogs(ctx, now, []Log{record}); err == nil {
		t.Fatal("managed logs were allowed in metrics.db")
	}
	if err := logs.Insert(ctx, now, []Sample{{ContainerID: id, Observation: docker.Observations{Time: now}}}); err == nil {
		t.Fatal("metric samples were allowed in logs.db")
	}
	var otherTables int
	if err := logs.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN ('samples', 'container_states', 'container_events', 'users', 'metrics_settings')`).Scan(&otherTables); err != nil || otherTables != 0 {
		t.Fatalf("log database contains other data tables: %d, %v", otherTables, err)
	}
	if err := logs.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "logs.db"))
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("private logs.db: %v, %v", info, err)
	}
	logs, err = OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer logs.Close()
	page, err := logs.SelectLogs(ctx, id, now.Add(-time.Minute), now, 10)
	if err != nil || page.More || !reflect.DeepEqual(page.Logs, []Log{record, record}) {
		t.Fatalf("reopened original log bytes/duplicates: %#v, %v", page, err)
	}
	ids, err := logs.SelectObservedContainers(ctx, now.Add(-time.Minute), now)
	if err != nil || !reflect.DeepEqual(ids, []string{id}) {
		t.Fatalf("log-only history identities: %v, %v", ids, err)
	}
	page, err = legacy.store.SelectLogs(ctx, id, now.Add(-time.Minute), now, 10)
	if err != nil || !reflect.DeepEqual(page.Logs, []Log{record}) {
		t.Fatalf("opening logs.db changed legacy history: %#v, %v", page, err)
	}
}

func TestManagedLogWriteRollbackAndCommittedCleanupFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	record := Log{ContainerID: strings.Repeat("b", 64), Time: now.Add(-time.Second),
		ReceivedAt: now, Stream: docker.LogStdout, Message: "kept"}
	m, err := OpenManagedLogs(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER fail_log BEFORE INSERT ON container_logs
		WHEN NEW.message = 'fail' BEGIN SELECT RAISE(FAIL, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	failed := record
	failed.Message = "fail"
	if err := m.InsertLogs(ctx, now, []Log{record, failed}); !errors.Is(err, ErrHistoryPaused) || errors.Is(err, ErrWriteCommitted) {
		t.Fatalf("failed log batch outcome: %v", err)
	}
	page, err := m.SelectLogs(ctx, record.ContainerID, now.Add(-time.Minute), now, 10)
	if err != nil || len(page.Logs) != 0 {
		t.Fatalf("failed batch left records: %#v, %v", page, err)
	}
	status, err := m.Status(ctx)
	if err != nil || status.PendingWrite != nil || status.UncertainWrite != nil {
		t.Fatalf("failed batch marker: %#v, %v", status, err)
	}
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER fail_log_cleanup BEFORE DELETE ON pending_write
		BEGIN SELECT RAISE(FAIL, 'injected cleanup failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := m.InsertLogs(ctx, now, []Log{record, record}); !errors.Is(err, ErrWriteCommitted) || !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("committed log cleanup failure: %v", err)
	}
	status, err = m.Status(ctx)
	if err != nil || status.PendingWrite == nil || status.PendingWrite.Kind != "logs" || !status.PendingWrite.Committed {
		t.Fatalf("committed log marker: %#v, %v", status, err)
	}
	page, err = m.SelectLogs(ctx, record.ContainerID, now.Add(-time.Minute), now, 10)
	if err != nil || !reflect.DeepEqual(page.Logs, []Log{record, record}) {
		t.Fatalf("committed batch records: %#v, %v", page, err)
	}
}

func TestInterruptedManagedLogWriteReconcilesWithoutReplay(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	record := Log{ContainerID: strings.Repeat("c", 64), Time: now.Add(-time.Second),
		ReceivedAt: now, Stream: docker.LogTTY, Message: "original\x00\xff"}
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepared", true: "committed"}[committed], func(t *testing.T) {
			dir := t.TempDir()
			m, err := OpenManagedLogs(ctx, dir, 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.store.db.ExecContext(ctx, `INSERT INTO pending_write VALUES (1, 'logs', ?, 0)`, now.UnixNano()); err != nil {
				t.Fatal(err)
			}
			if committed {
				if err := m.store.insertLogs(ctx, now, []Log{record, record}, true); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			m, err = OpenManagedLogs(ctx, dir, 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			if err := m.Maintain(ctx, now); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("maintenance before reconciliation: %v", err)
			}
			if err := m.InsertLogs(ctx, now, []Log{record}); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("write before reconciliation: %v", err)
			}
			pending, err := m.ReconcilePendingWrite(ctx)
			if err != nil || pending == nil || pending.Kind != "logs" || pending.Committed != committed {
				t.Fatalf("reconciled marker: %#v, %v", pending, err)
			}
			if err := m.Maintain(ctx, now); err != nil {
				t.Fatal(err)
			}
			page, err := m.SelectLogs(ctx, record.ContainerID, now.Add(-time.Minute), now, 10)
			want := []Log(nil)
			if committed {
				want = []Log{record, record}
			}
			if err != nil || !reflect.DeepEqual(page.Logs, want) {
				t.Fatalf("reconciliation changed/replayed records: %#v, %v", page, err)
			}
		})
	}
}

func TestManagedLogPressureDoesNotEvictOrPauseMetrics(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Now().UTC()
	id := strings.Repeat("d", 64)
	metrics, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer metrics.Close()
	sample := Sample{ContainerID: id, Observation: docker.Observations{Time: now.Add(-time.Second)}}
	state := State{ContainerID: id, Time: sample.Observation.Time, State: "running", Health: HealthHealthy}
	event := Event{ContainerID: id, Time: sample.Observation.Time, Kind: EventStart}
	if err := metrics.Insert(ctx, now, []Sample{sample}); err != nil {
		t.Fatal(err)
	}
	if err := metrics.InsertStates(ctx, now, []State{state}); err != nil {
		t.Fatal(err)
	}
	if err := metrics.InsertEvents(ctx, now, []Event{event}); err != nil {
		t.Fatal(err)
	}
	logs, err := OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer logs.Close()
	records := make([]Log, 256)
	for i := range records {
		records[i] = Log{ContainerID: id, Time: now.Add(-time.Duration(256-i) * time.Second),
			ReceivedAt: now, Stream: docker.LogStdout, Message: strings.Repeat("x", 4096)}
	}
	for start := 0; start < len(records); start += maxLogBatch {
		if err := logs.store.InsertLogs(ctx, now, records[start:start+maxLogBatch]); err != nil {
			t.Fatal(err)
		}
	}
	if err := logs.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	files, err := logs.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	logs.limit = files.Total + 1
	if err := logs.Maintain(ctx, now); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("pressure must request recovery maintenance: %v", err)
	}
	status, err := logs.Status(ctx)
	if err != nil || status.LogEvictions.Count != evictionBatchSize || status.Evictions.Count != 0 ||
		!status.LogEvictions.Oldest.Equal(records[0].Time) || !status.LogEvictions.Newest.Equal(records[evictionBatchSize-1].Time) {
		t.Fatalf("bounded log eviction status: %#v, %v", status, err)
	}
	if err := logs.Maintain(ctx, now); err != nil {
		t.Fatalf("log pressure recovery: %v", err)
	}
	page, err := logs.SelectLogs(ctx, id, now.Add(-time.Hour), now, 1000)
	if err != nil || len(page.Logs) != len(records)-evictionBatchSize {
		t.Fatalf("remaining log records: %d, %v", len(page.Logs), err)
	}
	samples, err := metrics.Select(ctx, id, now.Add(-time.Hour), now, 10)
	if err != nil || !reflect.DeepEqual(samples, []Sample{sample}) {
		t.Fatalf("log pressure changed metric history: %#v, %v", samples, err)
	}
	states, err := metrics.SelectStates(ctx, id, now.Add(-time.Hour), now, 10)
	if err != nil || !reflect.DeepEqual(states, []State{state}) {
		t.Fatalf("log pressure changed states: %#v, %v", states, err)
	}
	events, err := metrics.SelectEvents(ctx, id, now.Add(-time.Hour), now, 10)
	if err != nil || !reflect.DeepEqual(events, []Event{event}) {
		t.Fatalf("log pressure changed events: %#v, %v", events, err)
	}
	metricStatus, err := metrics.Status(ctx)
	if err != nil || metricStatus.Paused || metricStatus.Evictions.Count != 0 || metricStatus.LogEvictions.Count != 0 {
		t.Fatalf("log pressure affected metrics availability: %#v, %v", metricStatus, err)
	}
}

func TestManagedLogStalledEvictionAndExactAgeBoundary(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	record := Log{ContainerID: strings.Repeat("e", 64), Time: now.Add(-time.Hour),
		ReceivedAt: now, Stream: docker.LogStdout, Message: "boundary"}
	dir := t.TempDir()
	m, err := OpenManagedLogs(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.InsertLogs(ctx, now, []Log{record}); err != nil {
		t.Fatal(err)
	}
	page, err := m.SelectLogs(ctx, record.ContainerID, record.Time, now, 10)
	if err != nil || len(page.Logs) != 1 {
		t.Fatalf("exact one-hour record missing: %#v, %v", page, err)
	}
	if err := m.Maintain(ctx, now.Add(time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	page, err = m.SelectLogs(ctx, record.ContainerID, record.Time, now, 10)
	if err != nil || len(page.Logs) != 0 {
		t.Fatalf("expired record remained: %#v, %v", page, err)
	}
	record.Time = now.Add(-time.Second)
	if err := m.InsertLogs(ctx, now, []Log{record}); err != nil {
		t.Fatal(err)
	}
	files, err := m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	m.limit = files.Total + 1
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("small-record pressure: %v", err)
	}
	before, err := m.Status(ctx)
	if err != nil || before.LogEvictions.Count != 1 {
		t.Fatalf("small-record removal status: %#v, %v", before, err)
	}
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("no-progress retry must remain paused: %v", err)
	}
	after, err := m.Status(ctx)
	if err != nil || after.LogEvictions != before.LogEvictions {
		t.Fatalf("stalled retry removed more history: %#v, %v", after, err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = OpenManagedLogs(ctx, dir, files.Total+1)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("reopened stalled gate: %v", err)
	}
	m.limit = 10 << 20
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatalf("increased budget should recover: %v", err)
	}
}

func TestLogDatabaseRejectsForeignVersionedSchema(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE unrelated (value TEXT); PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if store, err := OpenLogs(ctx, dir); err == nil {
		store.Close()
		t.Fatal("foreign versioned schema accepted as logs.db")
	}
}
