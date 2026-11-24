package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type StateGap struct {
	ContainerID string
	Start       time.Time
	End         time.Time
	Shared      bool
}

type StateHistory struct {
	Baseline  *State
	States    []State
	Gaps      []StateGap
	Truncated bool
}

func (s *Store) SelectStateHistory(ctx context.Context, id string, start, end time.Time, limit int) (StateHistory, error) {
	from, to, err := historyRange(id, start, end, limit)
	if err != nil {
		return StateHistory{}, err
	}
	if end.Sub(start) > time.Hour {
		return StateHistory{}, errors.New("state history range exceeds one hour")
	}
	if limit == int(^uint(0)>>1) {
		return StateHistory{}, errors.New("state history limit is too large")
	}
	history := StateHistory{}
	var baseline State
	var ns int64
	err = s.db.QueryRowContext(ctx, `SELECT observed_ns, state, health, reconnect_observed FROM (
		SELECT observed_ns, state, health, reconnect_observed FROM container_states
			WHERE container_id = ? AND observed_ns < ?
		UNION ALL
		SELECT observed_ns, state, health, reconnect_observed FROM container_state_baselines
			WHERE container_id = ? AND observed_ns < ?
	) ORDER BY observed_ns DESC LIMIT 1`, id, from, id, from).
		Scan(&ns, &baseline.State, &baseline.Health, &baseline.ReconnectObserved)
	if err == nil {
		baseline.ContainerID = id
		baseline.Time = time.Unix(0, ns).UTC()
		if _, err := validateState(baseline); err != nil {
			return StateHistory{}, fmt.Errorf("invalid stored state baseline: %w", err)
		}
		history.Baseline = &baseline
	} else if !errors.Is(err, sql.ErrNoRows) {
		return StateHistory{}, fmt.Errorf("query state baseline: %w", err)
	}
	history.States, history.Truncated, err = s.selectStatesNewest(ctx, id, from, to, limit)
	if err != nil {
		return StateHistory{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT started_ns, ended_ns, shared FROM (
		SELECT started_ns, ended_ns, 0 AS shared FROM container_state_gaps
			WHERE container_id = ? AND started_ns < ? AND (ended_ns IS NULL OR ended_ns > ?)
		UNION ALL
		SELECT started_ns, ended_ns, 1 AS shared FROM state_source_gaps
			WHERE started_ns < ? AND (ended_ns IS NULL OR ended_ns > ?)
				AND EXISTS (
					SELECT 1 FROM container_states AS observed
					WHERE observed.container_id = ? AND observed.observed_ns <= state_source_gaps.started_ns
					UNION ALL
					SELECT 1 FROM container_state_baselines AS baseline
					WHERE baseline.container_id = ? AND baseline.observed_ns <= state_source_gaps.started_ns
				)
	) ORDER BY started_ns DESC LIMIT ?`, id, to, from, to, from, id, id, limit+1)
	if err != nil {
		return StateHistory{}, fmt.Errorf("query state observation gaps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var started int64
		var ended sql.NullInt64
		var shared bool
		if err := rows.Scan(&started, &ended, &shared); err != nil {
			return StateHistory{}, fmt.Errorf("read state observation gap: %w", err)
		}
		if len(history.Gaps) == limit {
			history.Truncated = true
			break
		}
		gap := StateGap{ContainerID: id, Start: time.Unix(0, started).UTC(), Shared: shared}
		if ended.Valid {
			gap.End = time.Unix(0, ended.Int64).UTC()
		}
		history.Gaps = append(history.Gaps, gap)
	}
	if err := rows.Err(); err != nil {
		return StateHistory{}, fmt.Errorf("read state observation gaps: %w", err)
	}
	for left, right := 0, len(history.Gaps)-1; left < right; left, right = left+1, right-1 {
		history.Gaps[left], history.Gaps[right] = history.Gaps[right], history.Gaps[left]
	}
	return history, nil
}

func (s *Store) StartStateGaps(ctx context.Context, ids []string, at time.Time) error {
	return s.startStateGaps(ctx, ids, at, false)
}

func (s *Store) startStateGaps(ctx context.Context, ids []string, at time.Time, pending bool) error {
	if s.kind != metricsDatabase {
		return errors.New("state observation gaps require metrics.db")
	}
	ns, err := timestamp(at)
	if err != nil {
		return fmt.Errorf("invalid state gap start: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state gap insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO container_state_gaps(container_id, started_ns)
		SELECT ?, ? WHERE NOT EXISTS (
			SELECT 1 FROM container_state_gaps WHERE container_id = ? AND ended_ns IS NULL
		)`)
	if err != nil {
		return fmt.Errorf("prepare state gap insert: %w", err)
	}
	defer stmt.Close()
	for _, id := range ids {
		if err := validateID(id); err != nil {
			return fmt.Errorf("invalid state gap identity: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, id, ns, id); err != nil {
			return fmt.Errorf("insert state gap for %s: %w", id, err)
		}
	}
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit state gaps: %w", err)
	}
	return nil
}

func (s *Store) EndStateGap(ctx context.Context, id string, at time.Time) error {
	return s.endStateGap(ctx, id, at, false)
}

func (s *Store) endStateGap(ctx context.Context, id string, at time.Time, pending bool) error {
	if s.kind != metricsDatabase {
		return errors.New("state observation gaps require metrics.db")
	}
	if err := validateID(id); err != nil {
		return err
	}
	ns, err := timestamp(at)
	if err != nil {
		return fmt.Errorf("invalid state gap end: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state gap close: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE container_state_gaps SET ended_ns = ?
		WHERE container_id = ? AND ended_ns IS NULL AND started_ns < ?`, ns, id, ns); err != nil {
		return fmt.Errorf("end state observation gap: %w", err)
	}
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit state gap close: %w", err)
	}
	return nil
}

func (s *Store) StartGlobalStateGap(ctx context.Context, at time.Time) error {
	return s.startGlobalStateGap(ctx, at, false)
}

func (s *Store) startGlobalStateGap(ctx context.Context, at time.Time, pending bool) error {
	if s.kind != metricsDatabase {
		return errors.New("shared state-source gaps require metrics.db")
	}
	ns, err := timestamp(at)
	if err != nil {
		return fmt.Errorf("invalid shared state gap start: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin shared state gap insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO state_source_gaps(started_ns)
		SELECT ? WHERE NOT EXISTS (SELECT 1 FROM state_source_gaps WHERE ended_ns IS NULL)`, ns); err != nil {
		return fmt.Errorf("insert shared state gap: %w", err)
	}
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit shared state gap: %w", err)
	}
	return nil
}

func (s *Store) EndGlobalStateGap(ctx context.Context, at time.Time) error {
	return s.endGlobalStateGap(ctx, at, false)
}

func (s *Store) endGlobalStateGap(ctx context.Context, at time.Time, pending bool) error {
	if s.kind != metricsDatabase {
		return errors.New("shared state-source gaps require metrics.db")
	}
	ns, err := timestamp(at)
	if err != nil {
		return fmt.Errorf("invalid shared state gap end: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin shared state gap close: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE state_source_gaps SET ended_ns = ?
		WHERE ended_ns IS NULL AND started_ns < ?`, ns, ns); err != nil {
		return fmt.Errorf("end shared state gap: %w", err)
	}
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit shared state gap close: %w", err)
	}
	return nil
}

func (s *Store) HasOpenGlobalStateGap(ctx context.Context) (bool, error) {
	if s.kind != metricsDatabase {
		return false, errors.New("shared state-source gaps require metrics.db")
	}
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM state_source_gaps WHERE ended_ns IS NULL
	)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check shared state-source gap: %w", err)
	}
	return exists != 0, nil
}

func (s *Store) HasOpenStateGap(ctx context.Context, id string) (bool, error) {
	if s.kind != metricsDatabase {
		return false, errors.New("state observation gaps require metrics.db")
	}
	if err := validateID(id); err != nil {
		return false, err
	}
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM container_state_gaps WHERE container_id = ? AND ended_ns IS NULL
	)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check open state observation gap: %w", err)
	}
	return exists != 0, nil
}

func (s *Store) selectStatesNewest(ctx context.Context, id string, from, to int64, limit int) ([]State, bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT observed_ns, state, health, reconnect_observed FROM container_states
		WHERE container_id = ? AND observed_ns >= ? AND observed_ns < ?
		ORDER BY observed_ns DESC LIMIT ?`, id, from, to, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("query recent container states: %w", err)
	}
	defer rows.Close()
	var result []State
	for rows.Next() {
		var ns int64
		state := State{ContainerID: id}
		if err := rows.Scan(&ns, &state.State, &state.Health, &state.ReconnectObserved); err != nil {
			return nil, false, fmt.Errorf("read recent container state: %w", err)
		}
		state.Time = time.Unix(0, ns).UTC()
		if _, err := validateState(state); err != nil {
			return nil, false, fmt.Errorf("invalid stored container state: %w", err)
		}
		result = append(result, state)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("read recent container states: %w", err)
	}
	truncated := len(result) > limit
	if truncated {
		result = result[:limit]
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result, truncated, nil
}
