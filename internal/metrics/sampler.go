package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type sampleSource interface {
	InspectContainer(context.Context, string) (runtime.ContainerInspection, error)
	ContainerStats(context.Context, string) (runtime.Stats, error)
}

type priorStats struct {
	started time.Time
	stats   runtime.Stats
}

// Sampler takes explicitly requested read-only observations. It does not own
// scope, scheduling, persistence or Docker event-stream continuity.
type Sampler struct {
	source   sampleSource
	previous map[string]priorStats
}

func NewSampler(source *runtime.Client) (*Sampler, error) {
	if source == nil {
		return nil, errors.New("Docker client is required")
	}
	return newSampler(source), nil
}

func newSampler(source sampleSource) *Sampler {
	return &Sampler{source: source, previous: make(map[string]priorStats)}
}

// Reset discards baselines after a disconnect, scope change or uncertain
// lifecycle event. Sampler is single-owner and is not safe for concurrent use.
func (s *Sampler) Reset() {
	clear(s.previous)
}

// Observe returns a discrete state and, only for a stably running container,
// a stats sample. continuous must be true only when the caller can establish
// uninterrupted observation since the previous sample, including event-stream
// continuity. A matching start time alone cannot establish that guarantee.
// The caller chooses whether and when to persist the returned observations.
func (s *Sampler) Observe(ctx context.Context, id string, at time.Time, continuous bool) (State, *Sample, error) {
	if err := validateID(id); err != nil {
		return State{}, nil, fmt.Errorf("invalid container ID: %w", err)
	}
	if _, err := timestamp(at); err != nil {
		delete(s.previous, id)
		return State{}, nil, fmt.Errorf("invalid observation time: %w", err)
	}
	before, err := s.source.InspectContainer(ctx, id)
	if err != nil {
		delete(s.previous, id)
		return State{}, nil, fmt.Errorf("inspect before stats: %w", err)
	}
	if before.ID != id {
		delete(s.previous, id)
		return State{}, nil, errors.New("inspected container ID does not match request")
	}
	state := State{ContainerID: id, Time: at, State: before.State, Health: Health(before.Health)}
	if _, err := validateState(state); err != nil {
		delete(s.previous, id)
		return State{}, nil, fmt.Errorf("invalid inspected state: %w", err)
	}
	if before.State != "running" {
		delete(s.previous, id)
		return state, nil, nil
	}
	stats, err := s.source.ContainerStats(ctx, id)
	if err != nil {
		delete(s.previous, id)
		return state, nil, fmt.Errorf("read container stats: %w", err)
	}
	after, err := s.source.InspectContainer(ctx, id)
	if err != nil {
		delete(s.previous, id)
		return state, nil, fmt.Errorf("inspect after stats: %w", err)
	}
	if after.ID != before.ID || after.State != "running" || !after.StartedAt.Equal(before.StartedAt) {
		delete(s.previous, id)
		return state, nil, errors.New("container state or instance changed during stats observation")
	}
	var previous *runtime.Stats
	if old, ok := s.previous[id]; ok && continuous && !before.StartedAt.IsZero() && before.StartedAt.Equal(old.started) {
		previous = &old.stats
	}
	observed, err := runtime.ObserveStats(stats, previous)
	if err != nil {
		delete(s.previous, id)
		return state, nil, fmt.Errorf("convert container stats: %w", err)
	}
	sample := &Sample{ContainerID: id, Observation: observed}
	if _, err := validate(*sample); err != nil {
		delete(s.previous, id)
		return state, nil, fmt.Errorf("invalid stats observation: %w", err)
	}
	s.previous[id] = priorStats{started: before.StartedAt, stats: stats}
	return state, sample, nil
}

// Forget removes rate baselines for containers no longer eligible for scope.
func (s *Sampler) Forget(id string) {
	delete(s.previous, id)
}
