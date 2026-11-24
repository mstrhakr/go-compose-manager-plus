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

// InsertEvents records only observed events. A container/timestamp/kind/health
// key makes identical reconnect replays idempotent; it does not fill gaps.
func (s *Store) InsertEvents(ctx context.Context, events []Event) error {
	return s.insertEvents(ctx, events, false)
}

func (s *Store) insertEvents(ctx context.Context, events []Event, pending bool) error {
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
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO event_dedup(container_id, observed_ns, kind, health) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare event deduplication: %w", err)
	}
	defer stmt.Close()
	eventStmt, err := tx.PrepareContext(ctx, `INSERT INTO container_events(container_id, observed_ns, kind, health) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare event insert: %w", err)
	}
	defer eventStmt.Close()
	for i, event := range events {
		result, err := stmt.ExecContext(ctx, event.ContainerID, times[i], event.Kind, event.Health)
		if err != nil {
			return fmt.Errorf("deduplicate event %d: %w", i, err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check event deduplication %d: %w", i, err)
		}
		if inserted == 0 {
			continue
		}
		if _, err := eventStmt.ExecContext(ctx, event.ContainerID, times[i], event.Kind, event.Health); err != nil {
			return fmt.Errorf("insert event %d: %w", i, err)
		}
	}
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit events: %w", ErrWriteOutcomeUnknown, err)
	}
	return nil
}

// InsertStates records discrete snapshots. No state is inferred between them.
func (s *Store) InsertStates(ctx context.Context, states []State) error {
	return s.insertStates(ctx, states, false)
}

func (s *Store) insertStates(ctx context.Context, states []State, pending bool) error {
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
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
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
	return historyTimeRange(start, end, limit)
}

func historyTimeRange(start, end time.Time, limit int) (int64, int64, error) {
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

// SelectTimelineEvents returns a container's newest events in chronological order.
func (s *Store) SelectTimelineEvents(ctx context.Context, id string, start, end time.Time, limit int) ([]Event, error) {
	from, to, err := historyRange(id, start, end, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT container_id, observed_ns, kind, health FROM container_events
		WHERE container_id = ? AND observed_ns >= ? AND observed_ns < ?
		ORDER BY observed_ns DESC, sequence DESC LIMIT ?`, id, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query timeline events: %w", err)
	}
	defer rows.Close()
	var result []Event
	for rows.Next() {
		var ns int64
		event := Event{}
		if err := rows.Scan(&event.ContainerID, &ns, &event.Kind, &event.Health); err != nil {
			return nil, fmt.Errorf("read timeline event: %w", err)
		}
		event.Time = time.Unix(0, ns).UTC()
		if _, err := validateEvent(event); err != nil {
			return nil, fmt.Errorf("invalid stored timeline event: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read timeline events: %w", err)
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result, nil
}

// SelectObservedContainers includes history-only identities without a live snapshot.
func (s *Store) SelectObservedContainers(ctx context.Context, start, end time.Time) ([]string, error) {
	from, to, err := historyTimeRange(start, end, 1)
	if err != nil {
		return nil, err
	}
	if end.Sub(start) > time.Hour {
		return nil, errors.New("observed container range exceeds one hour")
	}
	query := `SELECT container_id FROM samples WHERE observed_ns >= ? AND observed_ns < ?
		UNION SELECT container_id FROM container_events WHERE observed_ns >= ? AND observed_ns < ?
		UNION SELECT container_id FROM container_states WHERE observed_ns >= ? AND observed_ns < ?
		UNION SELECT container_id FROM container_logs WHERE observed_ns >= ? AND observed_ns < ?
		ORDER BY container_id`
	args := []any{from, to, from, to, from, to, from, to}
	if s.kind == logsDatabase {
		query = `SELECT DISTINCT container_id FROM container_logs
			WHERE observed_ns >= ? AND observed_ns < ? ORDER BY container_id`
		args = []any{from, to}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query observed containers: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read observed container: %w", err)
		}
		if err := validateID(id); err != nil {
			return nil, fmt.Errorf("invalid stored container identity: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read observed containers: %w", err)
	}
	return ids, nil
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
