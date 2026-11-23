package metrics

import (
	"context"
	"errors"
	"time"
)

// RunMaintenance performs an immediate maintenance attempt, then retries at
// interval until cancellation. The callback receives every completed attempt,
// including failures and subsequent recoveries. It must return promptly.
// The caller owns the store and must wait for this function before closing it.
func RunMaintenance(ctx context.Context, store *ManagedStore, interval time.Duration, report func(error)) error {
	if store == nil || interval <= 0 || report == nil {
		return errors.New("metrics maintenance requires a store, positive interval and report callback")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	return runMaintenance(ctx, store.Maintain, time.Now, ticker.C, report)
}

func runMaintenance(ctx context.Context, maintain func(context.Context, time.Time) error, now func() time.Time, ticks <-chan time.Time, report func(error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := maintain(ctx, now())
		if ctx.Err() != nil {
			return ctx.Err()
		}
		report(err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ticks:
			if !ok {
				return errors.New("metrics maintenance schedule stopped")
			}
		}
	}
}
