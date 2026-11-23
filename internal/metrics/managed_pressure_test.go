package metrics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestManagedPressureEvictsOldestAndRecordsWarning(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	id := strings.Repeat("a", 64)
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	samples := make([]Sample, 2000)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: now.Add(time.Duration(i-2000) * time.Second)}}
	}
	if err := m.store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if err := m.store.InsertEvents(ctx, []Event{{ContainerID: id, Time: now, Kind: EventOOM}}); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	files, err := m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	m.limit = files.Total + 1
	for i := 0; i < 20; i++ {
		err = m.Maintain(ctx, now)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrMetricsPaused) {
			t.Fatal(err)
		}
	}
	if err != nil {
		t.Fatalf("pressure did not recover: %v", err)
	}
	status, err := m.Status(ctx)
	if err != nil || status.Paused || status.Evictions.Count == 0 || !status.Evictions.Oldest.Equal(samples[0].Observation.Time) {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	remaining, err := m.Select(ctx, id, samples[0].Observation.Time, now, len(samples))
	if err != nil || len(remaining) != len(samples)-int(status.Evictions.Count) {
		t.Fatalf("remaining=%d status=%#v error=%v", len(remaining), status, err)
	}
	events, err := m.SelectEvents(ctx, id, now, now.Add(time.Second), 1)
	if err != nil || len(events) != 1 {
		t.Fatalf("event history=%#v error=%v", events, err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	reopened, err := m.Status(ctx)
	if err != nil || reopened.Evictions != status.Evictions {
		t.Fatalf("reopened status=%#v error=%v", reopened, err)
	}
}

func TestManagedStalledEvictionDoesNotRepeat(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	dir := t.TempDir()
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := m.store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	files, err := m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	m.limit = files.Total + 1
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("expected stalled pressure: %v", err)
	}
	first, err := m.Status(ctx)
	if err != nil || !first.Paused || first.Evictions.Count != 1 {
		t.Fatalf("status=%#v error=%v", first, err)
	}
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("expected paused retry: %v", err)
	}
	second, err := m.Status(ctx)
	if err != nil || second.Evictions != first.Evictions {
		t.Fatalf("retry status=%#v error=%v", second, err)
	}
	limit := m.limit
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = OpenManaged(ctx, dir, limit)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	reopenedFiles, err := m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	m.limit = reopenedFiles.Total + 1
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrMetricsPaused) || !strings.Contains(err.Error(), "did not reduce active storage") {
		t.Fatalf("expected stalled gate after reopen: %v", err)
	}
	reopened, err := m.Status(ctx)
	if err != nil || reopened.Evictions != first.Evictions {
		t.Fatalf("reopened status=%#v error=%v", reopened, err)
	}
	m.limit = 10 << 20
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatalf("recovery after raising limit: %v", err)
	}
}

func TestManagedFailedEvictionCanRetryAfterReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	files, err := m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	limit := files.Total + 1
	m.limit = limit
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER reject_pressure_delete BEFORE DELETE ON samples BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("expected failure: %v", err)
	}
	if _, err := m.store.db.ExecContext(ctx, `DROP TRIGGER reject_pressure_delete`); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m, err = OpenManaged(ctx, dir, limit)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := m.store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	files, err = m.store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	m.limit = files.Total + 1
	if err := m.Maintain(ctx, now); !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("expected paused after retry: %v", err)
	}
	got, err := m.Select(ctx, id, now, now.Add(time.Second), 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("retry did not evict sample: %#v error=%v", got, err)
	}
	status, err := m.Status(ctx)
	if err != nil || status.Evictions.Count != 1 {
		t.Fatalf("retry eviction history=%#v error=%v", status, err)
	}
}

func TestManagedCommittedWriteReportsMaintenanceFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	m, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TRIGGER inject_expired AFTER INSERT ON samples WHEN NEW.observed_ns = %d
			BEGIN INSERT INTO samples(container_id, observed_ns) VALUES (NEW.container_id, %d); END`,
			now.UnixNano(), now.Add(-2*time.Hour).UnixNano()),
		`CREATE TRIGGER fail_expiry BEFORE DELETE ON samples BEGIN SELECT RAISE(ABORT, 'expiry failure'); END`,
	} {
		if _, err := m.store.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	err = m.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}})
	if !errors.Is(err, ErrWriteCommitted) || !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("expected committed write and paused maintenance: %v", err)
	}
	got, err := m.Select(ctx, id, now, now.Add(time.Second), 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("committed sample=%#v error=%v", got, err)
	}
}

func TestManagedRejectsOldObservation(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	m, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	old := now.Add(-time.Hour - time.Nanosecond)
	if err := m.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: old}}}); err == nil {
		t.Error("accepted expired sample")
	}
	if err := m.InsertEvents(ctx, now, []Event{{ContainerID: id, Time: old, Kind: EventOOM}}); err == nil {
		t.Error("accepted expired event")
	}
	if err := m.InsertStates(ctx, now, []State{{ContainerID: id, Time: old, State: "running", Health: HealthNone}}); err == nil {
		t.Error("accepted expired state")
	}
}
