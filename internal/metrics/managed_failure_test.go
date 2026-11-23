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

func TestManagedGateFailureRollsBackEviction(t *testing.T) {
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
	if err := m.store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: now}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.db.ExecContext(ctx, `CREATE TRIGGER reject_gate BEFORE UPDATE ON pressure_gate BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
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
		t.Fatalf("expected failed pressure gate: %v", err)
	}
	got, err := m.Select(ctx, id, now, now.Add(time.Second), 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("gate failure changed samples=%#v error=%v", got, err)
	}
	status, err := m.Status(ctx)
	if err != nil || status.Evictions.Count != 0 {
		t.Fatalf("gate failure changed history=%#v error=%v", status, err)
	}
}

func TestManagedSQLiteFullPausesAndRetries(t *testing.T) {
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
	if err := m.Maintain(ctx, now); err != nil {
		t.Fatal(err)
	}
	pages, err := m.store.PageUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var maximum int64
	if err := m.store.db.QueryRowContext(ctx, fmt.Sprintf("PRAGMA max_page_count = %d", pages.Pages)).Scan(&maximum); err != nil || maximum != pages.Pages {
		t.Fatalf("set page limit=%d error=%v", maximum, err)
	}
	samples := make([]Sample, 1000)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: now.Add(time.Duration(i) * time.Nanosecond)}}
	}
	if err := m.Insert(ctx, now, samples); !errors.Is(err, ErrMetricsPaused) || errors.Is(err, ErrWriteCommitted) {
		t.Fatalf("expected failed uncommitted write: %v", err)
	}
	got, err := m.Select(ctx, id, now, now.Add(time.Second), 1000)
	if err != nil || len(got) != 0 {
		t.Fatalf("failed transaction samples=%d error=%v", len(got), err)
	}
	if err := m.store.db.QueryRowContext(ctx, `PRAGMA max_page_count = 10000`).Scan(&maximum); err != nil || maximum != 10000 {
		t.Fatalf("increase page limit=%d error=%v", maximum, err)
	}
	if err := m.Insert(ctx, now, samples); err != nil {
		t.Fatalf("retry after page limit increase: %v", err)
	}
	got, err = m.Select(ctx, id, now, now.Add(time.Second), 1000)
	if err != nil || len(got) != len(samples) {
		t.Fatalf("retry samples=%d error=%v", len(got), err)
	}
}
