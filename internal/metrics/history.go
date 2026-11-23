package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type EventKind string

const (
	EventStart   EventKind = "start"
	EventStop    EventKind = "stop"
	EventExit    EventKind = "die"
	EventRestart EventKind = "restart"
	EventOOM     EventKind = "oom"
	EventHealth  EventKind = "health_status"
)

type Health string

const (
	HealthUnknown   Health = "unknown"
	HealthNone      Health = "none"
	HealthStarting  Health = "starting"
	HealthHealthy   Health = "healthy"
	HealthUnhealthy Health = "unhealthy"
)

type Event struct {
	ContainerID string
	Time        time.Time
	Kind        EventKind
	// Health is set only for health-status events.
	Health Health
}

type State struct {
	ContainerID string
	Time        time.Time
	State       string
	Health      Health
}

func validHealth(health Health) bool {
	switch health {
	case HealthUnknown, HealthNone, HealthStarting, HealthHealthy, HealthUnhealthy:
		return true
	default:
		return false
	}
}

func validateEvent(event Event) (int64, error) {
	if err := validateID(event.ContainerID); err != nil {
		return 0, err
	}
	switch event.Kind {
	case EventHealth:
		if event.Health != HealthStarting && event.Health != HealthHealthy && event.Health != HealthUnhealthy {
			return 0, errors.New("invalid health event status")
		}
	case EventStart, EventStop, EventExit, EventRestart, EventOOM:
		if event.Health != "" {
			return 0, errors.New("health status is only valid for a health event")
		}
	default:
		return 0, errors.New("unsupported container event")
	}
	return timestamp(event.Time)
}

func validateState(state State) (int64, error) {
	if err := validateID(state.ContainerID); err != nil {
		return 0, err
	}
	switch state.State {
	case "created", "running", "paused", "restarting", "removing", "exited", "dead", "unknown":
	default:
		return 0, errors.New("unsupported container state")
	}
	if !validHealth(state.Health) {
		return 0, errors.New("unsupported health state")
	}
	return timestamp(state.Time)
}

// InsertEvents records only events observed by a caller; it does not fill gaps
// or deduplicate reconnect replays.
func (s *Store) InsertEvents(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return errors.New("no events provided")
	}
	times := make([]int64, len(events))
	for i, event := range events {
		var err error
		if times[i], err = validateEvent(event); err != nil {
			return fmt.Errorf("invalid event %d: %w", i, err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin event insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO container_events(container_id, observed_ns, kind, health) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare event insert: %w", err)
	}
	defer stmt.Close()
	for i, event := range events {
		if _, err := stmt.ExecContext(ctx, event.ContainerID, times[i], event.Kind, event.Health); err != nil {
			return fmt.Errorf("insert event %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit events: %w", ErrWriteOutcomeUnknown, err)
	}
	return nil
}

// InsertStates records discrete snapshots. No state is inferred between them.
func (s *Store) InsertStates(ctx context.Context, states []State) error {
	if len(states) == 0 {
		return errors.New("no states provided")
	}
	times := make([]int64, len(states))
	for i, state := range states {
		var err error
		if times[i], err = validateState(state); err != nil {
			return fmt.Errorf("invalid state %d: %w", i, err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO container_states(container_id, observed_ns, state, health) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare state insert: %w", err)
	}
	defer stmt.Close()
	for i, state := range states {
		if _, err := stmt.ExecContext(ctx, state.ContainerID, times[i], state.State, state.Health); err != nil {
			return fmt.Errorf("insert state %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit states: %w", ErrWriteOutcomeUnknown, err)
	}
	return nil
}

func historyRange(id string, start, end time.Time, limit int) (int64, int64, error) {
	if err := validateID(id); err != nil {
		return 0, 0, err
	}
	from, err := timestamp(start)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start: %w", err)
	}
	to, err := timestamp(end)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end: %w", err)
	}
	if from >= to || limit < 1 {
		return 0, 0, errors.New("invalid history interval or limit")
	}
	return from, to, nil
}

// SelectEvents returns observed events for one container in timestamp order.
func (s *Store) SelectEvents(ctx context.Context, id string, start, end time.Time, limit int) ([]Event, error) {
	from, to, err := historyRange(id, start, end, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT observed_ns, kind, health FROM container_events
		WHERE container_id = ? AND observed_ns >= ? AND observed_ns < ?
		ORDER BY observed_ns, sequence LIMIT ?`, id, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query container events: %w", err)
	}
	defer rows.Close()
	var result []Event
	for rows.Next() {
		var ns int64
		event := Event{ContainerID: id}
		if err := rows.Scan(&ns, &event.Kind, &event.Health); err != nil {
			return nil, fmt.Errorf("read container event: %w", err)
		}
		event.Time = time.Unix(0, ns).UTC()
		if _, err := validateEvent(event); err != nil {
			return nil, fmt.Errorf("invalid stored container event: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read container events: %w", err)
	}
	return result, nil
}

// SelectStates returns recorded snapshots without inferring continuous state.
func (s *Store) SelectStates(ctx context.Context, id string, start, end time.Time, limit int) ([]State, error) {
	from, to, err := historyRange(id, start, end, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT observed_ns, state, health FROM container_states
		WHERE container_id = ? AND observed_ns >= ? AND observed_ns < ?
		ORDER BY observed_ns LIMIT ?`, id, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query container states: %w", err)
	}
	defer rows.Close()
	var result []State
	for rows.Next() {
		var ns int64
		state := State{ContainerID: id}
		if err := rows.Scan(&ns, &state.State, &state.Health); err != nil {
			return nil, fmt.Errorf("read container state: %w", err)
		}
		state.Time = time.Unix(0, ns).UTC()
		if _, err := validateState(state); err != nil {
			return nil, fmt.Errorf("invalid stored container state: %w", err)
		}
		result = append(result, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read container states: %w", err)
	}
	return result, nil
}
