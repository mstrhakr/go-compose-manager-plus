package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type fakeSamplerSource struct {
	inspections []runtime.ContainerInspection
	stats       runtime.Stats
	err         error
}

func (f *fakeSamplerSource) InspectContainer(context.Context, string) (runtime.ContainerInspection, error) {
	if len(f.inspections) == 0 {
		return runtime.ContainerInspection{}, errors.New("no inspection")
	}
	next := f.inspections[0]
	f.inspections = f.inspections[1:]
	return next, nil
}

func (f *fakeSamplerSource) ContainerStats(context.Context, string) (runtime.Stats, error) {
	return f.stats, f.err
}

func TestSamplerRatesRequireContinuityAndInstance(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("a", 64)
	base := time.Date(2026, 11, 22, 21, 0, 0, 0, time.UTC)
	source := &fakeSamplerSource{}
	sampler := newSampler(source)
	started := base.Add(-time.Hour)
	run := func(at time.Time, start time.Time, counter uint64, continuous bool) *Sample {
		t.Helper()
		inspect := runtime.ContainerInspection{ID: id, State: "running", Health: "healthy", StartedAt: start}
		source.inspections = []runtime.ContainerInspection{inspect, inspect}
		source.stats = runtime.Stats{Read: at}
		source.stats.CPU.Usage.Total = &counter
		system, cpus := counter*10, uint64(1)
		source.stats.CPU.System = &system
		source.stats.CPU.Online = &cpus
		state, sample, err := sampler.Observe(ctx, id, at, continuous)
		if err != nil || state.Health != HealthHealthy || sample == nil {
			t.Fatalf("state=%#v sample=%#v error=%v", state, sample, err)
		}
		return sample
	}
	if sample := run(base, started, 10, true); sample.Observation.CPUPercent != nil {
		t.Fatal("first sample has rate")
	}
	if sample := run(base.Add(time.Second), started, 20, true); sample.Observation.CPUPercent == nil || *sample.Observation.CPUPercent != 10 {
		t.Fatalf("continuous CPU=%v", sample.Observation.CPUPercent)
	}
	if sample := run(base.Add(2*time.Second), started, 30, false); sample.Observation.CPUPercent != nil {
		t.Fatal("gap produced a rate")
	}
	if sample := run(base.Add(3*time.Second), base, 40, true); sample.Observation.CPUPercent != nil {
		t.Fatal("restart produced a rate")
	}
	sampler.Reset()
	if sample := run(base.Add(4*time.Second), base, 50, true); sample.Observation.CPUPercent != nil {
		t.Fatal("reset produced a rate")
	}
}

func TestSamplerStoppedAndInterruptedObservation(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("a", 64)
	now := time.Now().UTC()
	source := &fakeSamplerSource{}
	sampler := newSampler(source)
	source.inspections = []runtime.ContainerInspection{{ID: id, State: "exited", Health: "none"}}
	state, sample, err := sampler.Observe(ctx, id, now, false)
	if err != nil || state.State != "exited" || state.Health != HealthNone || sample != nil {
		t.Fatalf("stopped state=%#v sample=%#v error=%v", state, sample, err)
	}
	before := runtime.ContainerInspection{ID: id, State: "running", Health: "unknown", StartedAt: now.Add(-time.Minute)}
	after := before
	after.StartedAt = now
	source.inspections = []runtime.ContainerInspection{before, after}
	source.stats = runtime.Stats{Read: now}
	if _, sample, err := sampler.Observe(ctx, id, now, true); err == nil || sample != nil {
		t.Fatalf("interrupted sample=%#v error=%v", sample, err)
	}
	source.inspections = []runtime.ContainerInspection{before}
	source.err = errors.New("disconnected")
	if _, sample, err := sampler.Observe(ctx, id, now, true); err == nil || sample != nil {
		t.Fatalf("failed stats sample=%#v error=%v", sample, err)
	}
}
