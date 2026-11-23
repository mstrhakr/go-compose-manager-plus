package metrics

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestManagedRetentionWritesAndReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	id := strings.Repeat("a", 64)
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	old := now.Add(-time.Hour - time.Nanosecond)
	cutoff := now.Add(-time.Hour)
	for _, at := range []time.Time{old, cutoff} {
		if err := m.store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: at}}}); err != nil {
			t.Fatal(err)
		}
		if err := m.store.InsertEvents(ctx, []Event{{ContainerID: id, Time: at, Kind: EventStart}}); err != nil {
			t.Fatal(err)
		}
		if err := m.store.InsertStates(ctx, []State{{ContainerID: id, Time: at, State: "running", Health: HealthNone}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	samples, err := m.store.Select(ctx, id, old, now, 10)
	if err != nil || len(samples) != 1 || !samples[0].Observation.Time.Equal(cutoff) {
		t.Fatalf("samples=%#v error=%v", samples, err)
	}
	events, err := m.store.SelectEvents(ctx, id, old, now, 10)
	if err != nil || len(events) != 1 || !events[0].Time.Equal(cutoff) {
		t.Fatalf("events=%#v error=%v", events, err)
	}
	states, err := m.store.SelectStates(ctx, id, old, now, 10)
	if err != nil || len(states) != 1 || !states[0].Time.Equal(cutoff) {
		t.Fatalf("states=%#v error=%v", states, err)
	}
	if err := m.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}); err != nil {
		t.Fatal(err)
	}
	if err := m.InsertEvents(ctx, now, []Event{{ContainerID: id, Time: now, Kind: EventOOM}}); err != nil {
		t.Fatal(err)
	}
	if err := m.InsertStates(ctx, now, []State{{ContainerID: id, Time: now, State: "running", Health: HealthHealthy}}); err != nil {
		t.Fatal(err)
	}
	status, err := m.Status(ctx)
	if err != nil || status.Paused || status.Problem != nil || status.Evictions.Count != 0 {
		t.Fatalf("status=%#v error=%v", status, err)
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
	samples, err = m.store.Select(ctx, id, cutoff, now.Add(time.Second), 10)
	if err != nil || len(samples) != 2 {
		t.Fatalf("reopened samples=%#v error=%v", samples, err)
	}
}

func TestManagedPausesOnFailureAndRetries(t *testing.T) {
	ctx := context.Background()
	m, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER managed_write_failure BEFORE INSERT ON samples BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	sample := []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}
	if err := m.Insert(ctx, now, sample); !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("insert failure=%v", err)
	}
	status, err := m.Status(ctx)
	if err != nil || !status.Paused || status.Problem == nil {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if _, err := m.store.db.ExecContext(ctx, `DROP TRIGGER managed_write_failure`); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := m.Insert(ctx, now, sample); err != nil {
		t.Fatalf("retry=%v", err)
	}
	status, err = m.Status(ctx)
	if err != nil || status.Paused {
		t.Fatalf("recovered status=%#v error=%v", status, err)
	}
}

func TestManagedExclusiveDatabase(t *testing.T) {
	ctx := context.Background()
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
	other, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.ExecContext(ctx, `PRAGMA busy_timeout = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.ExecContext(ctx, `INSERT INTO samples(container_id, observed_ns) VALUES (?, ?)`, strings.Repeat("b", 64), time.Now().UnixNano()); err == nil {
		t.Fatal("external writer bypassed managed store")
	}
}

func TestManagedConcurrentWritesAndClose(t *testing.T) {
	ctx := context.Background()
	m, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			at := now.Add(time.Duration(i) * time.Nanosecond)
			if err := m.Insert(ctx, now, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: at}}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	samples, err := m.store.Select(ctx, id, now, now.Add(time.Second), 20)
	if err != nil || len(samples) != 12 {
		t.Fatalf("samples=%d error=%v", len(samples), err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(ctx, now); err == nil {
		t.Fatal("maintenance succeeded after close")
	}
}
