package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ExpiredHistory struct {
	Samples int64
	Events  int64
	States  int64
}

type EvictedSamples struct {
	Count  int64
	Oldest time.Time
	Newest time.Time
}

// EvictOldestSamples removes up to limit samples, ordered by observation time
// and container ID. It does not infer that deleting rows returns disk space.
// Callers must report early expiry and decide whether writes can resume.
func (s *Store) EvictOldestSamples(ctx context.Context, limit int) (EvictedSamples, error) {
	if limit < 1 {
		return EvictedSamples{}, errors.New("eviction limit must be positive")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EvictedSamples{}, fmt.Errorf("begin sample eviction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var count, oldest, newest int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MIN(observed_ns), 0), COALESCE(MAX(observed_ns), 0)
		FROM (SELECT observed_ns FROM samples ORDER BY observed_ns, container_id LIMIT ?)`, limit).
		Scan(&count, &oldest, &newest); err != nil {
		return EvictedSamples{}, fmt.Errorf("select oldest samples: %w", err)
	}
	if count == 0 {
		if err := tx.Commit(); err != nil {
			return EvictedSamples{}, fmt.Errorf("commit sample eviction: %w", err)
		}
		return EvictedSamples{}, nil
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM samples WHERE (container_id, observed_ns) IN
		(SELECT container_id, observed_ns FROM samples ORDER BY observed_ns, container_id LIMIT ?)`, limit)
	if err != nil {
		return EvictedSamples{}, fmt.Errorf("delete oldest samples: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return EvictedSamples{}, fmt.Errorf("count evicted samples: %w", err)
	}
	if deleted != count {
		return EvictedSamples{}, errors.New("evicted sample count changed during transaction")
	}
	if err := tx.Commit(); err != nil {
		return EvictedSamples{}, fmt.Errorf("commit sample eviction: %w", err)
	}
	return EvictedSamples{Count: count, Oldest: time.Unix(0, oldest).UTC(), Newest: time.Unix(0, newest).UTC()}, nil
}

// ExpireHistoryBefore atomically removes observations older than cutoff.
// The caller chooses and schedules the cutoff; deleted pages may remain
// allocated in SQLite and WAL files.
func (s *Store) ExpireHistoryBefore(ctx context.Context, cutoff time.Time) (ExpiredHistory, error) {
	ns, err := timestamp(cutoff)
	if err != nil {
		return ExpiredHistory{}, fmt.Errorf("invalid history cutoff: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ExpiredHistory{}, fmt.Errorf("begin history expiry: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var counts ExpiredHistory
	for _, step := range []struct {
		query string
		count *int64
	}{
		{`DELETE FROM samples WHERE observed_ns < ?`, &counts.Samples},
		{`DELETE FROM container_events WHERE observed_ns < ?`, &counts.Events},
		{`DELETE FROM container_states WHERE observed_ns < ?`, &counts.States},
	} {
		result, err := tx.ExecContext(ctx, step.query, ns)
		if err != nil {
			return ExpiredHistory{}, fmt.Errorf("expire history: %w", err)
		}
		if *step.count, err = result.RowsAffected(); err != nil {
			return ExpiredHistory{}, fmt.Errorf("count expired history: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ExpiredHistory{}, fmt.Errorf("commit history expiry: %w", err)
	}
	return counts, nil
}
