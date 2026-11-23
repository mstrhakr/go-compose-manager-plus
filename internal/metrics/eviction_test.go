package metrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestEvictOldestSamplesAcrossContainers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	for _, id := range ids {
		for i := 0; i < 3; i++ {
			if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := store.InsertEvents(ctx, []Event{{ContainerID: ids[0], Time: base, Kind: EventStart}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EvictOldestSamples(ctx, 0); err == nil {
		t.Fatal("accepted zero eviction limit")
	}
	evicted, err := store.EvictOldestSamples(ctx, 3)
	if err != nil || evicted.Count != 3 || !evicted.Oldest.Equal(base) || !evicted.Newest.Equal(base.Add(time.Second)) {
		t.Fatalf("eviction = %#v, error %v", evicted, err)
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
	a, err := store.Select(ctx, ids[0], base, base.Add(3*time.Second), 10)
	if err != nil || len(a) != 1 || !a[0].Observation.Time.Equal(base.Add(2*time.Second)) {
		t.Fatalf("container a = %#v, error %v", a, err)
	}
	b, err := store.Select(ctx, ids[1], base, base.Add(3*time.Second), 10)
	if err != nil || len(b) != 2 || !b[0].Observation.Time.Equal(base.Add(time.Second)) {
		t.Fatalf("container b = %#v, error %v", b, err)
	}
	events, err := store.SelectEvents(ctx, ids[0], base, base.Add(time.Second), 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("event history affected: %#v, error %v", events, err)
	}
	evicted, err = store.EvictOldestSamples(ctx, 10)
	if err != nil || evicted.Count != 3 || !evicted.Oldest.Equal(base.Add(time.Second)) || !evicted.Newest.Equal(base.Add(2*time.Second)) {
		t.Fatalf("remaining eviction = %#v, error %v", evicted, err)
	}
	evicted, err = store.EvictOldestSamples(ctx, 10)
	if err != nil || evicted != (EvictedSamples{}) {
		t.Fatalf("empty eviction = %#v, error %v", evicted, err)
	}
}
