package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ExpiredHistory struct {
	Samples int64
	Events  int64
	States  int64
	Logs    int64
}

type EvictedHistory struct {
	Count  int64
	Oldest time.Time
	Newest time.Time
}

type EvictedSamples = EvictedHistory
type EvictedLogs = EvictedHistory

// EvictOldestSamples removes up to limit samples, ordered by observation time
// and container ID, and atomically records the cumulative eviction history.
// It does not infer that deleting rows returns disk space.
func (s *Store) EvictOldestSamples(ctx context.Context, limit int) (EvictedSamples, error) {
	return s.evictOldestSamples(ctx, limit, nil)
}

func (s *Store) evictOldestSamples(ctx context.Context, limit int, activeBytes *int64) (EvictedSamples, error) {
	if s.kind != metricsDatabase {
		return EvictedSamples{}, errors.New("sample eviction requires metrics.db")
	}
	return s.evictOldest(ctx, limit, activeBytes)
}

// EvictOldestLogs removes a bounded oldest-first batch from logs.db only.
// Cumulative counts and timestamp bounds do not describe a continuous gap.
func (s *Store) EvictOldestLogs(ctx context.Context, limit int) (EvictedLogs, error) {
	if s.kind != logsDatabase {
		return EvictedLogs{}, errors.New("log eviction requires logs.db")
	}
	return s.evictOldest(ctx, limit, nil)
}

func (s *Store) evictOldest(ctx context.Context, limit int, activeBytes *int64) (EvictedHistory, error) {
	if limit < 1 {
		return EvictedHistory{}, errors.New("eviction limit must be positive")
	}
	selectSQL := `SELECT COUNT(*), COALESCE(MIN(observed_ns), 0), COALESCE(MAX(observed_ns), 0)
		FROM (SELECT observed_ns FROM samples ORDER BY observed_ns, container_id LIMIT ?)`
	deleteSQL := `DELETE FROM samples WHERE (container_id, observed_ns) IN
		(SELECT container_id, observed_ns FROM samples ORDER BY observed_ns, container_id LIMIT ?)`
	recordSQL := `INSERT INTO sample_eviction_history (id, sample_count, oldest_ns, newest_ns)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			sample_count = sample_count + excluded.sample_count,
			oldest_ns = min(oldest_ns, excluded.oldest_ns),
			newest_ns = max(newest_ns, excluded.newest_ns)`
	if s.kind == logsDatabase {
		selectSQL = `SELECT COUNT(*), COALESCE(MIN(observed_ns), 0), COALESCE(MAX(observed_ns), 0)
			FROM (SELECT observed_ns FROM container_logs ORDER BY observed_ns, sequence LIMIT ?)`
		deleteSQL = `DELETE FROM container_logs WHERE sequence IN
			(SELECT sequence FROM container_logs ORDER BY observed_ns, sequence LIMIT ?)`
		recordSQL = `INSERT INTO log_eviction_history (id, log_count, oldest_ns, newest_ns)
			VALUES (1, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				log_count = log_count + excluded.log_count,
				oldest_ns = min(oldest_ns, excluded.oldest_ns),
				newest_ns = max(newest_ns, excluded.newest_ns)`
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EvictedHistory{}, fmt.Errorf("begin history eviction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var count, oldest, newest int64
	if err := tx.QueryRowContext(ctx, selectSQL, limit).
		Scan(&count, &oldest, &newest); err != nil {
		return EvictedHistory{}, fmt.Errorf("select oldest history: %w", err)
	}
	if count == 0 {
		if err := tx.Commit(); err != nil {
			return EvictedHistory{}, fmt.Errorf("commit history eviction: %w", err)
		}
		return EvictedHistory{}, nil
	}
	result, err := tx.ExecContext(ctx, deleteSQL, limit)
	if err != nil {
		return EvictedHistory{}, fmt.Errorf("delete oldest history: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return EvictedHistory{}, fmt.Errorf("count evicted history: %w", err)
	}
	if deleted != count {
		return EvictedHistory{}, errors.New("evicted history count changed during transaction")
	}
	if _, err := tx.ExecContext(ctx, recordSQL, count, oldest, newest); err != nil {
		return EvictedHistory{}, fmt.Errorf("record history eviction: %w", err)
	}
	if activeBytes != nil {
		result, err := tx.ExecContext(ctx, `UPDATE pressure_gate SET active_bytes = ? WHERE id = 1`, *activeBytes)
		if err != nil {
			return EvictedHistory{}, fmt.Errorf("record history pressure gate: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return EvictedHistory{}, fmt.Errorf("count history pressure gate updates: %w", err)
		}
		if updated != 1 {
			return EvictedHistory{}, errors.New("history pressure gate is missing")
		}
	}
	if err := tx.Commit(); err != nil {
		return EvictedHistory{}, fmt.Errorf("commit history eviction: %w", err)
	}
	return EvictedHistory{Count: count, Oldest: time.Unix(0, oldest).UTC(), Newest: time.Unix(0, newest).UTC()}, nil
}

// SampleEvictionHistory reports cumulative sample eviction since schema
// version 4. The span bounds evicted timestamps, not a continuous gap.
// Older unrecorded evictions cannot be reconstructed.
func (s *Store) SampleEvictionHistory(ctx context.Context) (EvictedSamples, error) {
	if s.kind != metricsDatabase {
		return EvictedSamples{}, errors.New("sample eviction history requires metrics.db")
	}
	return s.evictionHistory(ctx)
}

func (s *Store) LogEvictionHistory(ctx context.Context) (EvictedLogs, error) {
	if s.kind != logsDatabase {
		return EvictedLogs{}, errors.New("log eviction history requires logs.db")
	}
	return s.evictionHistory(ctx)
}

func (s *Store) evictionHistory(ctx context.Context) (EvictedHistory, error) {
	var count, oldest, newest int64
	query := `SELECT sample_count, oldest_ns, newest_ns FROM sample_eviction_history WHERE id = 1`
	if s.kind == logsDatabase {
		query = `SELECT log_count, oldest_ns, newest_ns FROM log_eviction_history WHERE id = 1`
	}
	err := s.db.QueryRowContext(ctx, query).Scan(&count, &oldest, &newest)
	if errors.Is(err, sql.ErrNoRows) {
		return EvictedHistory{}, nil
	}
	if err != nil {
		return EvictedHistory{}, fmt.Errorf("read history eviction history: %w", err)
	}
	return EvictedHistory{Count: count, Oldest: time.Unix(0, oldest).UTC(), Newest: time.Unix(0, newest).UTC()}, nil
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
	steps := []struct {
		query string
		count *int64
	}{
		{`DELETE FROM samples WHERE observed_ns < ?`, &counts.Samples},
		{`DELETE FROM container_events WHERE observed_ns < ?`, &counts.Events},
		{`DELETE FROM container_states WHERE observed_ns < ?`, &counts.States},
		{`DELETE FROM container_logs WHERE observed_ns < ?`, &counts.Logs},
	}
	if s.kind == logsDatabase {
		steps = steps[3:]
	}
	for _, step := range steps {
		result, err := tx.ExecContext(ctx, step.query, ns)
		if err != nil {
			return ExpiredHistory{}, fmt.Errorf("expire history: %w", err)
		}
		if *step.count, err = result.RowsAffected(); err != nil {
			return ExpiredHistory{}, fmt.Errorf("count expired history: %w", err)
		}
	}
	if s.kind != logsDatabase {
		if _, err := tx.ExecContext(ctx, `DELETE FROM event_dedup WHERE observed_ns < ?`, ns); err != nil {
			return ExpiredHistory{}, fmt.Errorf("expire event replay identities: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ExpiredHistory{}, fmt.Errorf("commit history expiry: %w", err)
	}
	return counts, nil
}
