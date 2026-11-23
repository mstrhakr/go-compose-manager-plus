package metrics

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestExpireHistoryBeforeIsAtomicAndPreservesCutoff(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	now := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	cutoff := now.Add(-time.Hour)
	times := []time.Time{cutoff.Add(-time.Nanosecond), cutoff, now}
	for _, at := range times {
		if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: at}}}); err != nil {
			t.Fatal(err)
		}
		if err := store.InsertEvents(ctx, []Event{{ContainerID: id, Time: at, Kind: EventStart}}); err != nil {
			t.Fatal(err)
		}
		if err := store.InsertStates(ctx, []State{{ContainerID: id, Time: at, State: "running", Health: HealthNone}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER expiry_failure BEFORE DELETE ON container_states BEGIN SELECT RAISE(ABORT, 'injected expiry failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExpireHistoryBefore(ctx, cutoff); err == nil {
		t.Fatal("expected expiry failure")
	}
	samples, err := store.Select(ctx, id, times[0], now.Add(time.Second), 10)
	if err != nil || len(samples) != 3 {
		t.Fatalf("failed transaction changed samples: %d, %v", len(samples), err)
	}
	events, err := store.SelectEvents(ctx, id, times[0], now.Add(time.Second), 10)
	if err != nil || len(events) != 3 {
		t.Fatalf("failed transaction changed events: %d, %v", len(events), err)
	}
	var identities int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_dedup`).Scan(&identities); err != nil || identities != 3 {
		t.Fatalf("failed transaction changed replay identities: %d, %v", identities, err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER expiry_failure`); err != nil {
		t.Fatal(err)
	}
	counts, err := store.ExpireHistoryBefore(ctx, cutoff)
	if err != nil || counts != (ExpiredHistory{Samples: 1, Events: 1, States: 1}) {
		t.Fatalf("expiry counts = %#v, error %v", counts, err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_dedup`).Scan(&identities); err != nil || identities != 2 {
		t.Fatalf("expiry retained replay identities beyond cutoff: %d, %v", identities, err)
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
	samples, err = store.Select(ctx, id, times[0], now.Add(time.Second), 10)
	if err != nil || len(samples) != 2 || !samples[0].Observation.Time.Equal(cutoff) || !samples[1].Observation.Time.Equal(now) {
		t.Fatalf("remaining samples = %#v, error %v", samples, err)
	}
	events, err = store.SelectEvents(ctx, id, times[0], now.Add(time.Second), 10)
	if err != nil || len(events) != 2 || !events[0].Time.Equal(cutoff) || !events[1].Time.Equal(now) {
		t.Fatalf("remaining events = %#v, error %v", events, err)
	}
	states, err := store.SelectStates(ctx, id, times[0], now.Add(time.Second), 10)
	if err != nil || len(states) != 2 || !states[0].Time.Equal(cutoff) || !states[1].Time.Equal(now) {
		t.Fatalf("remaining states = %#v, error %v", states, err)
	}
	counts, err = store.ExpireHistoryBefore(ctx, cutoff)
	if err != nil || !reflect.DeepEqual(counts, ExpiredHistory{}) {
		t.Fatalf("repeated expiry = %#v, error %v", counts, err)
	}
	if _, err := store.ExpireHistoryBefore(ctx, time.Time{}); err == nil {
		t.Fatal("accepted zero cutoff")
	}
}
