package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrMetricsPaused  = errors.New("metrics writes paused")
	ErrWriteCommitted = errors.New("metrics write committed but subsequent maintenance failed")
	ErrReconcileWrite = errors.New("uncertain metrics write requires reconciliation")
)

const evictionBatchSize = 128

// ManagedStore owns its SQLite connection and serializes maintenance and writes.
// Callers must schedule Maintain even when there are no observations.
type ManagedStore struct {
	lock      chan struct{}
	store     *Store
	limit     int64
	paused    error
	uncertain error
	closed    bool
}

type storageSnapshot struct {
	files FileUsage
	pages PageUsage
}

type ManagedStatus struct {
	Paused         bool
	Problem        error
	UncertainWrite error
	Files          FileUsage
	Pages          PageUsage
	Evictions      EvictedSamples
	PendingWrite   *PendingWrite
}

// PendingWrite identifies an interrupted managed write, not its complete payload.
type PendingWrite struct {
	Kind      string
	CreatedAt time.Time
	Committed bool
}

// OpenManaged reserves the database using SQLite exclusive locking. The caller
// must close it and must not open another metrics writer for the same database.
// limit is a logical-file-length target, not a physical disk quota.
func OpenManaged(ctx context.Context, dataDir string, limit int64) (*ManagedStore, error) {
	if limit <= 0 {
		return nil, errors.New("metrics storage limit must be positive")
	}
	store, err := open(ctx, dataDir, true)
	if err != nil {
		return nil, err
	}
	m := &ManagedStore{store: store, limit: limit, paused: errors.New("metrics maintenance has not run"), lock: make(chan struct{}, 1)}
	pending, err := m.pendingWrite(ctx)
	if err != nil {
		return nil, errors.Join(err, store.Close())
	}
	if pending != nil {
		m.uncertain = fmt.Errorf("interrupted %s write started at %s (committed: %t)", pending.Kind, pending.CreatedAt.Format(time.RFC3339Nano), pending.Committed)
		m.paused = fmt.Errorf("%w: %w", ErrReconcileWrite, m.uncertain)
	}
	m.lock <- struct{}{}
	return m, nil
}

func (m *ManagedStore) pendingWrite(ctx context.Context) (*PendingWrite, error) {
	var pending PendingWrite
	var created int64
	err := m.store.db.QueryRowContext(ctx, `SELECT kind, created_ns, committed FROM pending_write WHERE id = 1`).Scan(&pending.Kind, &created, &pending.Committed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pending metrics write: %w", err)
	}
	pending.CreatedAt = time.Unix(0, created).UTC()
	return &pending, nil
}

func markPendingCommitted(ctx context.Context, tx *sql.Tx) error {
	result, err := tx.ExecContext(ctx, `UPDATE pending_write SET committed = 1 WHERE id = 1 AND committed = 0`)
	if err != nil {
		return fmt.Errorf("mark metrics write committed: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check pending metrics write: %w", err)
	}
	if count != 1 {
		return errors.New("pending metrics write is missing or already committed")
	}
	return nil
}

func (m *ManagedStore) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.lock:
		if err := ctx.Err(); err != nil {
			m.release()
			return err
		}
		return nil
	}
}

func (m *ManagedStore) release() {
	m.lock <- struct{}{}
}

func (m *ManagedStore) snapshot(ctx context.Context) (storageSnapshot, error) {
	files, err := m.store.FileUsage()
	if err != nil {
		return storageSnapshot{}, err
	}
	pages, err := m.store.PageUsage(ctx)
	if err != nil {
		return storageSnapshot{}, err
	}
	if pages.Pages*pages.PageSize != files.Database {
		return storageSnapshot{}, errors.New("metrics page count differs from checkpointed database length")
	}
	return storageSnapshot{files: files, pages: pages}, nil
}

func (m *ManagedStore) pause(err error) error {
	m.paused = err
	return fmt.Errorf("%w: %w", ErrMetricsPaused, err)
}

// Maintain expires history strictly older than one hour and attempts at most
// one bounded sample eviction when logical usage exceeds the target. Repeated
// attempts stop if a batch produced no reusable pages.
func (m *ManagedStore) Maintain(ctx context.Context, now time.Time) error {
	if err := m.acquire(ctx); err != nil {
		return err
	}
	defer m.release()
	return m.maintain(ctx, now)
}

func (m *ManagedStore) maintain(ctx context.Context, now time.Time) error {
	if m.closed {
		return errors.New("metrics store is closed")
	}
	if _, err := timestamp(now); err != nil {
		return fmt.Errorf("invalid maintenance time: %w", err)
	}
	if m.uncertain != nil {
		return m.pause(fmt.Errorf("%w: %w", ErrReconcileWrite, m.uncertain))
	}
	if err := m.store.Checkpoint(ctx); err != nil {
		return m.pause(err)
	}
	if _, err := m.store.ExpireHistoryBefore(ctx, now.Add(-time.Hour)); err != nil {
		return m.pause(err)
	}
	if err := m.store.Checkpoint(ctx); err != nil {
		return m.pause(err)
	}
	before, err := m.snapshot(ctx)
	if err != nil {
		return m.pause(err)
	}
	if before.files.Total >= m.limit {
		return m.pause(errors.New("metrics files leave no space under the configured limit; SQLite deletion does not shrink the database"))
	}
	target := m.limit - (m.limit / 10) - boolInt(m.limit%10 != 0)
	active := before.files.Total - before.pages.ReusablePages*before.pages.PageSize
	var gate sql.NullInt64
	if err := m.store.db.QueryRowContext(ctx, `SELECT active_bytes FROM pressure_gate WHERE id = 1`).Scan(&gate); err != nil {
		return m.pause(fmt.Errorf("read metrics pressure gate: %w", err))
	}
	if gate.Valid {
		if active >= target && active >= gate.Int64 {
			return m.pause(errors.New("sample eviction did not reduce active storage; waiting for storage conditions to change"))
		}
		if _, err := m.store.db.ExecContext(ctx, `UPDATE pressure_gate SET active_bytes = NULL WHERE id = 1`); err != nil {
			return m.pause(fmt.Errorf("clear metrics pressure gate: %w", err))
		}
		if err := m.store.Checkpoint(ctx); err != nil {
			return m.pause(err)
		}
		before, err = m.snapshot(ctx)
		if err != nil {
			return m.pause(err)
		}
		if before.files.Total >= m.limit {
			return m.pause(errors.New("metrics files leave no space under the configured limit; SQLite deletion does not shrink the database"))
		}
		active = before.files.Total - before.pages.ReusablePages*before.pages.PageSize
	}
	if active < target {
		m.paused = nil
		return nil
	}
	evicted, err := m.store.evictOldestSamples(ctx, evictionBatchSize, &active)
	if err != nil {
		return m.pause(err)
	}
	if evicted.Count == 0 {
		return m.pause(errors.New("storage target exceeded and no samples remain to evict"))
	}
	if err := m.store.Checkpoint(ctx); err != nil {
		return m.pause(err)
	}
	after, err := m.snapshot(ctx)
	if err != nil {
		return m.pause(err)
	}
	afterActive := after.files.Total - after.pages.ReusablePages*after.pages.PageSize
	if afterActive >= active {
		return m.pause(errors.New("sample eviction did not reduce active storage"))
	}
	return m.pause(errors.New("bounded sample eviction completed; retry maintenance to verify recovery"))
}

func boolInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// Status reports the last maintenance outcome and the durable eviction
// summary; it does not initiate a recovery attempt.
func (m *ManagedStore) Status(ctx context.Context) (ManagedStatus, error) {
	if err := m.acquire(ctx); err != nil {
		return ManagedStatus{}, err
	}
	defer m.release()
	if m.closed {
		return ManagedStatus{}, errors.New("metrics store is closed")
	}
	files, err := m.store.FileUsage()
	if err != nil {
		return ManagedStatus{}, fmt.Errorf("read metrics status: %w", err)
	}
	pages, err := m.store.PageUsage(ctx)
	if err != nil {
		return ManagedStatus{}, fmt.Errorf("read metrics status: %w", err)
	}
	evictions, err := m.store.SampleEvictionHistory(ctx)
	if err != nil {
		return ManagedStatus{}, fmt.Errorf("read metrics status: %w", err)
	}
	pending, err := m.pendingWrite(ctx)
	if err != nil {
		return ManagedStatus{}, err
	}
	return ManagedStatus{Paused: m.paused != nil, Problem: m.paused, UncertainWrite: m.uncertain, Files: files, Pages: pages, Evictions: evictions, PendingWrite: pending}, nil
}

// AcknowledgeReconciledWrite clears the durable and in-process guards only
// after the caller has reconciled the interrupted write. It does not retry it.
func (m *ManagedStore) AcknowledgeReconciledWrite(ctx context.Context) error {
	if err := m.acquire(ctx); err != nil {
		return err
	}
	defer m.release()
	if m.closed {
		return errors.New("metrics store is closed")
	}
	if m.uncertain == nil {
		return errors.New("no uncertain metrics write to acknowledge")
	}
	if _, err := m.store.db.ExecContext(ctx, `DELETE FROM pending_write WHERE id = 1`); err != nil {
		return fmt.Errorf("acknowledge pending metrics write: %w", err)
	}
	m.uncertain = nil
	return nil
}

// ReconcilePendingWrite inspects the transactional marker and removes it
// without replaying the interrupted batch. The committed bit changed in the
// same transaction as its records, so either outcome is known after reopen.
func (m *ManagedStore) ReconcilePendingWrite(ctx context.Context) (*PendingWrite, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	pending, err := m.pendingWrite(ctx)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		if m.uncertain != nil {
			return nil, fmt.Errorf("cannot reconcile metrics write without a durable marker: %w", m.uncertain)
		}
		return nil, nil
	}
	result, err := m.store.db.ExecContext(ctx, `DELETE FROM pending_write WHERE id = 1`)
	if err != nil {
		return nil, fmt.Errorf("clear reconciled metrics write: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check reconciled metrics write: %w", err)
	}
	if count != 1 {
		return nil, errors.New("pending metrics write changed during reconciliation")
	}
	m.uncertain = nil
	m.paused = nil
	return pending, nil
}

func (m *ManagedStore) write(ctx context.Context, now time.Time, kind string, insert func() error) error {
	if err := m.acquire(ctx); err != nil {
		return err
	}
	defer m.release()
	if err := m.maintain(ctx, now); err != nil {
		return err
	}
	if _, err := m.store.db.ExecContext(ctx, `INSERT INTO pending_write(id, kind, created_ns, committed) VALUES (1, ?, ?, 0)`, kind, now.UnixNano()); err != nil {
		m.uncertain = fmt.Errorf("prepare metrics write: %w", err)
		return m.pause(m.uncertain)
	}
	if err := insert(); err != nil {
		if errors.Is(err, ErrWriteOutcomeUnknown) {
			m.uncertain = err
		} else if _, cleanupErr := m.store.db.ExecContext(ctx, `DELETE FROM pending_write WHERE id = 1`); cleanupErr != nil {
			m.uncertain = errors.Join(err, fmt.Errorf("clear failed metrics write: %w", cleanupErr))
			err = m.uncertain
		}
		return m.pause(err)
	}
	result, err := m.store.db.ExecContext(ctx, `DELETE FROM pending_write WHERE id = 1 AND committed = 1`)
	if err == nil {
		var count int64
		count, err = result.RowsAffected()
		if err == nil && count != 1 {
			err = errors.New("committed metrics write marker is missing")
		}
	}
	if err != nil {
		m.uncertain = fmt.Errorf("clear committed metrics write: %w", err)
		return fmt.Errorf("%w: %w", ErrWriteCommitted, m.pause(m.uncertain))
	}
	if err := m.maintain(ctx, now); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteCommitted, err)
	}
	return nil
}

func (m *ManagedStore) Insert(ctx context.Context, now time.Time, samples []Sample) error {
	if len(samples) == 0 {
		return errors.New("no metric samples provided")
	}
	for i, sample := range samples {
		if _, err := validate(sample); err != nil {
			return fmt.Errorf("invalid sample %d: %w", i, err)
		}
		if err := withinHistory(now, sample.Observation.Time); err != nil {
			return fmt.Errorf("invalid sample %d: %w", i, err)
		}
	}
	return m.write(ctx, now, "samples", func() error { return m.store.insert(ctx, samples, true) })
}

func (m *ManagedStore) InsertEvents(ctx context.Context, now time.Time, events []Event) error {
	if len(events) == 0 {
		return errors.New("no events provided")
	}
	for i, event := range events {
		if _, err := validateEvent(event); err != nil {
			return fmt.Errorf("invalid event %d: %w", i, err)
		}
		if err := withinHistory(now, event.Time); err != nil {
			return fmt.Errorf("invalid event %d: %w", i, err)
		}
	}
	return m.write(ctx, now, "events", func() error { return m.store.insertEvents(ctx, events, true) })
}

func (m *ManagedStore) InsertStates(ctx context.Context, now time.Time, states []State) error {
	if len(states) == 0 {
		return errors.New("no states provided")
	}
	for i, state := range states {
		if _, err := validateState(state); err != nil {
			return fmt.Errorf("invalid state %d: %w", i, err)
		}
		if err := withinHistory(now, state.Time); err != nil {
			return fmt.Errorf("invalid state %d: %w", i, err)
		}
	}
	return m.write(ctx, now, "states", func() error { return m.store.insertStates(ctx, states, true) })
}

func withinHistory(now, at time.Time) error {
	if _, err := timestamp(now); err != nil {
		return fmt.Errorf("invalid write time: %w", err)
	}
	if at.Before(now.Add(-time.Hour)) {
		return errors.New("observation is older than the one-hour history window")
	}
	return nil
}

func (m *ManagedStore) Select(ctx context.Context, id string, start, end time.Time, limit int) ([]Sample, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.Select(ctx, id, start, end, limit)
}

func (m *ManagedStore) SelectChartSeries(ctx context.Context, start, end time.Time, bucket time.Duration) ([]ContainerChartSeries, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.SelectChartSeries(ctx, start, end, bucket)
}

func (m *ManagedStore) SelectEvents(ctx context.Context, id string, start, end time.Time, limit int) ([]Event, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.SelectEvents(ctx, id, start, end, limit)
}

func (m *ManagedStore) SelectTimelineEvents(ctx context.Context, id string, start, end time.Time, limit int) ([]Event, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.SelectTimelineEvents(ctx, id, start, end, limit)
}

func (m *ManagedStore) SelectObservedContainers(ctx context.Context, start, end time.Time) ([]string, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.SelectObservedContainers(ctx, start, end)
}

func (m *ManagedStore) SelectStates(ctx context.Context, id string, start, end time.Time, limit int) ([]State, error) {
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	if m.closed {
		return nil, errors.New("metrics store is closed")
	}
	return m.store.SelectStates(ctx, id, start, end, limit)
}

func (m *ManagedStore) Close() error {
	<-m.lock
	defer m.release()
	if m.closed {
		return errors.New("metrics store is closed")
	}
	m.closed = true
	return m.store.Close()
}
