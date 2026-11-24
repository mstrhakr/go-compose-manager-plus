package metrics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagedCanceledWaitAndClose(t *testing.T) {
	m, err := OpenManaged(context.Background(), t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Maintain(ctx, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- m.Close() }()
	m.release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := m.Maintain(context.Background(), time.Now()); err == nil {
		t.Fatal("maintenance succeeded after close")
	}
}

func TestVersionFourMigrationCreatesPressureGate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE pressure_gate`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE pending_write`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE event_dedup`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE container_logs`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA user_version = 4`); err != nil {
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
	if err := m.Maintain(ctx, time.Now()); err != nil {
		t.Fatalf("maintenance after version-four migration: %v", err)
	}
	var version int
	if err := m.store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("version=%d error=%v", version, err)
	}
}
