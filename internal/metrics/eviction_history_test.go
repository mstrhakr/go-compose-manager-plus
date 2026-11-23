package metrics

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestConcurrentEvictionsRecordAllDeletions(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	const count = 20
	samples := make([]Sample, count)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}
	}
	if err := store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := store.EvictOldestSamples(ctx, 1)
			if err != nil {
				errs <- err
			} else if result.Count != 1 {
				t.Errorf("evicted %d samples, want 1", result.Count)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	history, err := store.SampleEvictionHistory(ctx)
	if err != nil || history.Count != count || !history.Oldest.Equal(base) || !history.Newest.Equal(base.Add((count-1)*time.Second)) {
		t.Fatalf("concurrent eviction history = %#v, error %v", history, err)
	}
	remaining, err := store.Select(ctx, id, base, base.Add(count*time.Second), count)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("remaining samples = %#v, error %v", remaining, err)
	}
}

func TestEvictionHistoryOverflowRollsBackDeletion(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	if err := store.Insert(ctx, []Sample{{ContainerID: id, Observation: runtime.Observations{Time: base}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO sample_eviction_history VALUES (1, ?, ?, ?)`,
		int64(math.MaxInt64), base.UnixNano(), base.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EvictOldestSamples(ctx, 1); err == nil {
		t.Fatal("expected history count overflow to fail")
	}
	got, err := store.Select(ctx, id, base, base.Add(time.Second), 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("overflow changed samples: %#v, error %v", got, err)
	}
	history, err := store.SampleEvictionHistory(ctx)
	if err != nil || history.Count != math.MaxInt64 {
		t.Fatalf("overflow changed history: %#v, error %v", history, err)
	}
}
