package metrics

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestCheckpointReportsReaderAndRecovers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := store.db.ExecContext(ctx, `PRAGMA busy_timeout = 50`); err != nil {
		t.Fatal(err)
	}
	reader, err := sql.Open("sqlite", filepath.Join(dir, "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: time.Now()}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Checkpoint(ctx); !errors.Is(err, ErrCheckpointBusy) {
		t.Fatalf("checkpoint error = %v, want blocked", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := store.Checkpoint(ctx); err != nil {
		t.Fatalf("checkpoint after reader release: %v", err)
	}
	usage, err := store.FileUsage()
	if err != nil || usage.WAL != 0 {
		t.Fatalf("checkpoint usage = %#v, error %v", usage, err)
	}
}
