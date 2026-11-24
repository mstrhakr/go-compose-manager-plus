package metrics

import (
	"context"
	"errors"
	"fmt"
)

var ErrCheckpointBusy = errors.New("history WAL checkpoint blocked by another connection")

// Checkpoint consolidates and truncates the WAL when no readers prevent it.
// It does not compact the database or establish a hard disk-usage limit.
func (s *Store) Checkpoint(ctx context.Context) error {
	var busy, logFrames, checkpointed int
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return fmt.Errorf("checkpoint history WAL: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("checkpoint history WAL (%d of %d frames): %w", checkpointed, logFrames, ErrCheckpointBusy)
	}
	return nil
}
