package metrics

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestEvictionFailureRollsBack(t *testing.T) {
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
	samples := []Sample{
		{ContainerID: id, Observation: runtime.Observations{Time: base}},
		{ContainerID: id, Observation: runtime.Observations{Time: base.Add(time.Second)}},
	}
	if err := store.Insert(ctx, samples); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER eviction_failure BEFORE DELETE ON samples
		WHEN OLD.observed_ns = `+strconv.FormatInt(base.Add(time.Second).UnixNano(), 10)+
		` BEGIN SELECT RAISE(ABORT, 'injected eviction failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EvictOldestSamples(ctx, 2); err == nil {
		t.Fatal("expected eviction failure")
	}
	got, err := store.Select(ctx, id, base, base.Add(2*time.Second), 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("failed eviction changed samples: %#v, error %v", got, err)
	}
}
