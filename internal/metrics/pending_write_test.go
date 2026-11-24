package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInterruptedEventWriteSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepared", true: "committed"}[committed], func(t *testing.T) {
			dir := t.TempDir()
			m, err := OpenManaged(ctx, dir, 10<<20)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.store.db.ExecContext(ctx, `INSERT INTO pending_write VALUES (1, 'events', ?, 0)`, now.UnixNano()); err != nil {
				t.Fatal(err)
			}
			if committed {
				if err := m.store.insertEvents(ctx, []Event{{ContainerID: id, Time: now, Kind: EventOOM}}, true); err != nil {
					t.Fatal(err)
				}
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
			status, err := m.Status(ctx)
			if err != nil || !status.Paused || !errors.Is(status.Problem, ErrReconcileWrite) || status.UncertainWrite == nil || status.PendingWrite == nil ||
				status.PendingWrite.Kind != "events" || status.PendingWrite.Committed != committed || !status.PendingWrite.CreatedAt.Equal(now) {
				t.Fatalf("reopened status = %#v, %v", status, err)
			}
			if err := m.Maintain(ctx, now); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("maintenance without reconciliation: %v", err)
			}
			event := Event{ContainerID: id, Time: now, Kind: EventOOM}
			if err := m.InsertEvents(ctx, now, []Event{event}); !errors.Is(err, ErrReconcileWrite) {
				t.Fatalf("write without reconciliation: %v", err)
			}
			events, err := m.SelectEvents(ctx, id, now.Add(-time.Second), now.Add(time.Second), 10)
			if err != nil || len(events) != map[bool]int{false: 0, true: 1}[committed] {
				t.Fatalf("events after interruption = %#v, %v", events, err)
			}
			pending, err := m.ReconcilePendingWrite(ctx)
			if err != nil || pending == nil || pending.Committed != committed {
				t.Fatalf("reconcile marker = %#v, %v", pending, err)
			}
			if err := m.Maintain(ctx, now); err != nil {
				t.Fatalf("maintenance after acknowledgement: %v", err)
			}
			status, err = m.Status(ctx)
			if err != nil || status.PendingWrite != nil || status.UncertainWrite != nil {
				t.Fatalf("acknowledged status = %#v, %v", status, err)
			}
		})
	}
}

func TestCommittedWriteCleanupFailureSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	dir := t.TempDir()
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER fail_pending_cleanup BEFORE DELETE ON pending_write BEGIN SELECT RAISE(FAIL, 'cleanup failed'); END`); err != nil {
		t.Fatal(err)
	}
	err = m.InsertEvents(ctx, now, []Event{{ContainerID: id, Time: now, Kind: EventOOM}})
	if !errors.Is(err, ErrWriteCommitted) || !errors.Is(err, ErrMetricsPaused) {
		t.Fatalf("committed write with failed cleanup = %v", err)
	}
	if _, err := m.store.db.ExecContext(ctx, `DROP TRIGGER fail_pending_cleanup`); err != nil {
		t.Fatal(err)
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
	status, err := m.Status(ctx)
	if err != nil || status.PendingWrite == nil || !status.PendingWrite.Committed {
		t.Fatalf("reopened status = %#v, %v", status, err)
	}
	events, err := m.SelectEvents(ctx, id, now.Add(-time.Second), now.Add(time.Second), 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("committed events = %#v, %v", events, err)
	}
	pending, err := m.ReconcilePendingWrite(ctx)
	if err != nil || pending == nil || !pending.Committed {
		t.Fatalf("reconcile committed marker = %#v, %v", pending, err)
	}
}

func TestVersionFiveMigrationCreatesPendingWrite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	removeCurrentStateSchema(t, store.db)
	if _, err := store.db.ExecContext(ctx, `DROP TABLE pending_write`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE event_dedup`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE container_logs`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := OpenManaged(ctx, dir, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	status, err := m.Status(ctx)
	if err != nil || status.PendingWrite != nil {
		t.Fatalf("migrated status = %#v, %v", status, err)
	}
}

func TestEventReplayDeduplicatesAndMigrationPreservesHistory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	removeCurrentStateSchema(t, store.db)
	id := strings.Repeat("b", 64)
	at := time.Now().UTC().Truncate(time.Nanosecond)
	for range 2 {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO container_events(container_id, observed_ns, kind, health) VALUES (?, ?, 'oom', '')`, id, at.UnixNano()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE event_dedup`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE container_logs`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	event := Event{ContainerID: id, Time: at, Kind: EventOOM}
	if err := store.InsertEvents(ctx, []Event{event}); err != nil {
		t.Fatal(err)
	}
	events, err := store.SelectEvents(ctx, id, at.Add(-time.Second), at.Add(time.Second), 10)
	if err != nil || len(events) != 2 {
		t.Fatalf("migration discarded history or replay was inserted: %#v, %v", events, err)
	}
	freshID := strings.Repeat("f", 64)
	fresh := Event{ContainerID: freshID, Time: at.Add(time.Second), Kind: EventOOM}
	if err := store.InsertEvents(ctx, []Event{fresh, fresh}); err != nil {
		t.Fatal(err)
	}
	events, err = store.SelectEvents(ctx, freshID, at, at.Add(2*time.Second), 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("new replay was not deduplicated: %#v, %v", events, err)
	}
}
