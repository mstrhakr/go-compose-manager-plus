package metrics

import (
	"context"
	"fmt"
	"time"
)

type ExpiredHistory struct {
	Samples int64
	Events  int64
	States  int64
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
