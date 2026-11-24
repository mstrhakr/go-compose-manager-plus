package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStateHistoryRetainsSinglePreWindowBaseline(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := strings.Repeat("a", 64)
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	states := []State{
		{ContainerID: id, Time: start.Add(-2 * time.Hour), State: "running", Health: HealthHealthy},
		{ContainerID: id, Time: start.Add(-time.Hour - time.Minute), State: "running", Health: HealthUnhealthy},
		{ContainerID: id, Time: start.Add(time.Minute), State: "running", Health: HealthHealthy, ReconnectObserved: true},
	}
	if err := store.InsertStates(ctx, states); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExpireHistoryBefore(ctx, start); err != nil {
		t.Fatal(err)
	}
	history, err := store.SelectStateHistory(ctx, id, start, start.Add(time.Hour), 10)
	if err != nil || history.Baseline == nil ||
		!history.Baseline.Time.Equal(states[1].Time) ||
		history.Baseline.Health != HealthUnhealthy ||
		len(history.States) != 1 || !history.States[0].ReconnectObserved {
		t.Fatalf("state history baseline and observations = %#v, error %v", history, err)
	}
	var baselineCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM container_state_baselines`).Scan(&baselineCount); err != nil || baselineCount != 1 {
		t.Fatalf("retained state baseline count = %d, error %v", baselineCount, err)
	}
}

func TestStateHistoryRecordsExplicitUnavailableSpans(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := strings.Repeat("b", 64)
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	if err := store.StartStateGaps(ctx, []string{id}, start); err != nil {
		t.Fatal(err)
	}
	if err := store.StartStateGaps(ctx, []string{id}, start.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertStates(ctx, []State{{
		ContainerID: id, Time: start.Add(5 * time.Second), State: "running",
		Health: HealthHealthy, ReconnectObserved: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.EndStateGap(ctx, id, start.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	history, err := store.SelectStateHistory(ctx, id, start.Add(time.Second), start.Add(10*time.Second), 10)
	if err != nil || len(history.Gaps) != 1 || !history.Gaps[0].Start.Equal(start) ||
		!history.Gaps[0].End.Equal(start.Add(5*time.Second)) || len(history.States) != 1 ||
		!history.States[0].ReconnectObserved {
		t.Fatalf("state history gap and reconnect snapshot = %#v, error %v", history, err)
	}
}

func TestStateHistoryDetectsTruncationAndKeepsNewestStates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := strings.Repeat("c", 64)
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	states := []State{
		{ContainerID: id, Time: start.Add(time.Minute), State: "running", Health: HealthUnhealthy},
		{ContainerID: id, Time: start.Add(2 * time.Minute), State: "running", Health: HealthHealthy},
		{ContainerID: id, Time: start.Add(3 * time.Minute), State: "running", Health: HealthUnhealthy},
	}
	if err := store.InsertStates(ctx, states); err != nil {
		t.Fatal(err)
	}
	history, err := store.SelectStateHistory(ctx, id, start, start.Add(time.Hour), 2)
	if err != nil || !history.Truncated || len(history.States) != 2 ||
		!history.States[0].Time.Equal(states[1].Time) || !history.States[1].Time.Equal(states[2].Time) {
		t.Fatalf("truncated state history = %#v, error %v", history, err)
	}
}

func TestSharedStateGapIsStoredOnceAndProjectedToObservedContainers(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, second := strings.Repeat("d", 64), strings.Repeat("e", 64)
	start := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	if err := store.StartGlobalStateGap(ctx, start); err != nil {
		t.Fatal(err)
	}
	if err := store.StartGlobalStateGap(ctx, start.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	states := []State{
		{ContainerID: first, Time: start.Add(-time.Second), State: "running", Health: HealthHealthy},
		{ContainerID: second, Time: start.Add(-time.Second), State: "paused", Health: HealthNone},
		{ContainerID: first, Time: start.Add(5 * time.Second), State: "running", Health: HealthHealthy},
		{ContainerID: second, Time: start.Add(5 * time.Second), State: "paused", Health: HealthNone},
	}
	if err := store.InsertStates(ctx, states); err != nil {
		t.Fatal(err)
	}
	if err := store.EndGlobalStateGap(ctx, start.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first, second} {
		history, err := store.SelectStateHistory(ctx, id, start, start.Add(time.Minute), 10)
		if err != nil || len(history.Gaps) != 1 || !history.Gaps[0].Shared ||
			!history.Gaps[0].Start.Equal(start) || !history.Gaps[0].End.Equal(start.Add(5*time.Second)) {
			t.Fatalf("projected shared gap for %s = %#v, error %v", id, history, err)
		}
	}
	var shared, perContainer int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM state_source_gaps`).Scan(&shared); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM container_state_gaps`).Scan(&perContainer); err != nil {
		t.Fatal(err)
	}
	if shared != 1 || perContainer != 0 {
		t.Fatalf("shared outage storage rows = shared %d, per-container %d", shared, perContainer)
	}
}

func TestManagedStateGapsRespectStoragePressure(t *testing.T) {
	ctx := context.Background()
	store, err := OpenManaged(ctx, t.TempDir(), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("f", 64)
	start := time.Now().UTC().Add(-time.Second)
	if err := store.StartStateGaps(ctx, []string{id}, start); err != nil {
		t.Fatal(err)
	}
	if err := store.StartGlobalStateGap(ctx, start); err != nil {
		t.Fatal(err)
	}
	store.limit = 1
	if err := store.Maintain(ctx, time.Now().UTC()); err == nil {
		t.Fatal("tiny storage target did not pause")
	}
	if err := store.EndStateGap(ctx, id, start.Add(time.Second)); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("state gap close bypassed pressure pause: %v", err)
	}
	if err := store.EndGlobalStateGap(ctx, start.Add(time.Second)); !errors.Is(err, ErrHistoryPaused) {
		t.Fatalf("shared state gap close bypassed pressure pause: %v", err)
	}
	localOpen, err := store.HasOpenStateGap(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	globalOpen, err := store.HasOpenGlobalStateGap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !localOpen || !globalOpen {
		t.Fatalf("paused close changed durable gaps: local=%t global=%t", localOpen, globalOpen)
	}
}
