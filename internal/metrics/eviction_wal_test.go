package metrics

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestBoundedEvictionCheckpointProbe(t *testing.T) {
	if os.Getenv("CMP_METRICS_SQLITE_EVICTION_PROBE") != "1" {
		t.Skip("set CMP_METRICS_SQLITE_EVICTION_PROBE=1 to run the synthetic eviction probe")
	}
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
	samples := make([]Sample, 5000)
	for i := range samples {
		samples[i] = Sample{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Duration(i) * time.Second)}}
	}
	if err := store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if err := store.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	var maxWAL int64
	for range 40 {
		evicted, err := store.EvictOldestSamples(ctx, 100)
		if err != nil || evicted.Count != 100 {
			t.Fatalf("eviction=%#v error=%v", evicted, err)
		}
		files, err := store.FileUsage()
		if err != nil {
			t.Fatal(err)
		}
		if files.WAL > maxWAL {
			maxWAL = files.WAL
		}
		if err := store.Checkpoint(ctx); err != nil {
			t.Fatal(err)
		}
	}
	after, err := store.FileUsage()
	if err != nil {
		t.Fatal(err)
	}
	if maxWAL == 0 || after.Database != before.Database || after.WAL != 0 {
		t.Fatalf("before=%#v after=%#v peak observed WAL=%d", before, after, maxWAL)
	}
	history, err := store.SampleEvictionHistory(ctx)
	if err != nil || history.Count != 4000 || !history.Newest.Equal(base.Add(3999*time.Second)) {
		t.Fatalf("history=%#v error=%v", history, err)
	}
	survivors, err := store.Select(ctx, id, base, base.Add(5000*time.Second), 5000)
	if err != nil || len(survivors) != 1000 || !survivors[0].Observation.Time.Equal(base.Add(4000*time.Second)) {
		t.Fatalf("surviving samples=%d error=%v", len(survivors), err)
	}
	t.Logf("40 batches of 100: database bytes=%d; largest observed WAL before checkpoint=%d", after.Database, maxWAL)
}
