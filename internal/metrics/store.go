// Package metrics provides persistence for collected container observations.
// It is not connected to the application or a background collector yet.
package metrics

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
	_ "modernc.org/sqlite"
)

const schemaVersion = 5

// ErrWriteOutcomeUnknown means a database commit failed and its outcome must
// be reconciled before retrying the same batch.
var ErrWriteOutcomeUnknown = errors.New("metrics write commit outcome is unknown")

type Sample struct {
	ContainerID string
	Observation runtime.Observations
}

type Store struct {
	db   *sql.DB
	path string
}

// Open uses a separate database from application state. It does not start
// collection or enforce a storage budget.
func Open(ctx context.Context, dataDir string) (*Store, error) {
	return open(ctx, dataDir, false)
}

func open(ctx context.Context, dataDir string, exclusive bool) (*Store, error) {
	if dataDir == "" {
		return nil, errors.New("metrics data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create metrics directory: %w", err)
	}
	path := filepath.Join(dataDir, "metrics.db")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("metrics database must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect metrics database: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open metrics database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(err error) (*Store, error) {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close metrics database: %w", closeErr))
		}
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("connect metrics database: %w", err))
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fail(fmt.Errorf("restrict metrics database permissions: %w", err))
	}
	for _, statement := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA synchronous = FULL`,
		`PRAGMA journal_mode = WAL`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fail(fmt.Errorf("configure metrics database: %w", err))
		}
	}
	if exclusive {
		var mode string
		if err := db.QueryRowContext(ctx, `PRAGMA locking_mode = EXCLUSIVE`).Scan(&mode); err != nil {
			return fail(fmt.Errorf("reserve metrics database: %w", err))
		}
		if mode != "exclusive" {
			return fail(fmt.Errorf("metrics database refused exclusive locking: %s", mode))
		}
	}
	if err := initialize(ctx, db); err != nil {
		return fail(err)
	}
	return &Store{db: db, path: path}, nil
}

func initialize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metrics migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read metrics schema version: %w", err)
	}
	if version > schemaVersion || version < 0 {
		return fmt.Errorf("unsupported metrics schema version %d", version)
	}
	if version == 0 {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&count); err != nil {
			return fmt.Errorf("inspect metrics schema: %w", err)
		}
		if count != 0 {
			return errors.New("unversioned metrics database contains tables")
		}
		for _, statement := range []string{
			`CREATE TABLE samples (
				container_id TEXT NOT NULL,
				observed_ns INTEGER NOT NULL,
				cpu_percent REAL,
				memory_usage_bytes INTEGER,
				memory_limit_bytes INTEGER,
				network_rx_bytes_per_sec REAL,
				network_tx_bytes_per_sec REAL,
				block_read_bytes_per_sec REAL,
				block_write_bytes_per_sec REAL,
				PRIMARY KEY (container_id, observed_ns)
			) WITHOUT ROWID`,
			`CREATE INDEX samples_time ON samples(observed_ns)`,
			`PRAGMA user_version = 1`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("create metrics schema: %w", err)
			}
		}
		version = 1
	}
	if version == 1 {
		for _, statement := range []string{
			`CREATE TABLE samples_v2 (
				container_id TEXT NOT NULL,
				observed_ns INTEGER NOT NULL,
				cpu_percent REAL,
				memory_usage_bytes TEXT,
				memory_limit_bytes TEXT,
				network_rx_bytes_per_sec REAL,
				network_tx_bytes_per_sec REAL,
				block_read_bytes_per_sec REAL,
				block_write_bytes_per_sec REAL,
				PRIMARY KEY (container_id, observed_ns)
			) WITHOUT ROWID`,
			`INSERT INTO samples_v2 SELECT container_id, observed_ns, cpu_percent,
				CAST(memory_usage_bytes AS TEXT), CAST(memory_limit_bytes AS TEXT),
				network_rx_bytes_per_sec, network_tx_bytes_per_sec,
				block_read_bytes_per_sec, block_write_bytes_per_sec FROM samples`,
			`DROP TABLE samples`,
			`ALTER TABLE samples_v2 RENAME TO samples`,
			`CREATE INDEX samples_time ON samples(observed_ns)`,
			`PRAGMA user_version = 2`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 2: %w", err)
			}
		}
		version = 2
	}
	if version == 2 {
		for _, statement := range []string{
			`CREATE TABLE container_events (
				sequence INTEGER PRIMARY KEY,
				container_id TEXT NOT NULL,
				observed_ns INTEGER NOT NULL,
				kind TEXT NOT NULL,
				health TEXT NOT NULL
			)`,
			`CREATE INDEX container_events_time ON container_events(observed_ns, sequence)`,
			`CREATE INDEX container_events_container_time ON container_events(container_id, observed_ns, sequence)`,
			`CREATE TABLE container_states (
				container_id TEXT NOT NULL,
				observed_ns INTEGER NOT NULL,
				state TEXT NOT NULL,
				health TEXT NOT NULL,
				PRIMARY KEY (container_id, observed_ns)
			) WITHOUT ROWID`,
			`CREATE INDEX container_states_time ON container_states(observed_ns)`,
			`PRAGMA user_version = 3`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 3: %w", err)
			}
		}
		version = 3
	}
	if version == 3 {
		for _, statement := range []string{
			`CREATE TABLE sample_eviction_history (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				sample_count INTEGER NOT NULL CHECK (typeof(sample_count) = 'integer' AND sample_count > 0),
				oldest_ns INTEGER NOT NULL,
				newest_ns INTEGER NOT NULL CHECK (oldest_ns <= newest_ns)
			)`,
			`PRAGMA user_version = 4`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 4: %w", err)
			}
		}
		version = 4
	}
	if version == 4 {
		for _, statement := range []string{
			`CREATE TABLE pressure_gate (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				active_bytes INTEGER CHECK (active_bytes IS NULL OR active_bytes >= 0)
			)`,
			`INSERT INTO pressure_gate (id, active_bytes) VALUES (1, NULL)`,
			`PRAGMA user_version = 5`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 5: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit metrics migration: %w", err)
	}
	return nil
}

func timestamp(t time.Time) (int64, error) {
	if t.IsZero() {
		return 0, errors.New("observation timestamp is required")
	}
	n := t.UnixNano()
	if !time.Unix(0, n).Equal(t) {
		return 0, errors.New("observation timestamp is outside nanosecond range")
	}
	return n, nil
}

func validateID(id string) error {
	if len(id) != 64 {
		return errors.New("container ID must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return fmt.Errorf("invalid container ID: %w", err)
	}
	return nil
}

func validate(sample Sample) (int64, error) {
	if err := validateID(sample.ContainerID); err != nil {
		return 0, err
	}
	o := sample.Observation
	for _, value := range []*float64{o.CPUPercent, o.NetworkRxBytesPerSec, o.NetworkTxBytesPerSec, o.BlockReadBytesPerSec, o.BlockWriteBytesPerSec} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
			return 0, errors.New("metric must be finite and nonnegative")
		}
	}
	return timestamp(o.Time)
}

func sqliteUint(value *uint64) any {
	if value == nil {
		return nil
	}
	return strconv.FormatUint(*value, 10)
}

func parseUint(value sql.NullString) (*uint64, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := strconv.ParseUint(value.String, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid stored unsigned metric: %w", err)
	}
	return &parsed, nil
}

// Insert atomically writes a batch. Duplicate container/timestamp pairs fail
// rather than silently replacing previously observed values.
func (s *Store) Insert(ctx context.Context, samples []Sample) error {
	if len(samples) == 0 {
		return errors.New("no metric samples provided")
	}
	times := make([]int64, len(samples))
	for i, sample := range samples {
		var err error
		if times[i], err = validate(sample); err != nil {
			return fmt.Errorf("invalid sample %d: %w", i, err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metrics insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO samples VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare metrics insert: %w", err)
	}
	defer stmt.Close()
	for i, sample := range samples {
		o := sample.Observation
		if _, err := stmt.ExecContext(ctx, sample.ContainerID, times[i], o.CPUPercent, sqliteUint(o.MemoryUsageBytes), sqliteUint(o.MemoryLimitBytes),
			o.NetworkRxBytesPerSec, o.NetworkTxBytesPerSec, o.BlockReadBytesPerSec, o.BlockWriteBytesPerSec); err != nil {
			return fmt.Errorf("insert metric sample %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit metric samples: %w", ErrWriteOutcomeUnknown, err)
	}
	return nil
}

// Select reads observations in ascending order in the half-open interval.
// Callers must bound the interval and limit to suit their use.
func (s *Store) Select(ctx context.Context, id string, start, end time.Time, limit int) ([]Sample, error) {
	from, to, err := historyRange(id, start, end, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT observed_ns, cpu_percent, memory_usage_bytes, memory_limit_bytes,
		network_rx_bytes_per_sec, network_tx_bytes_per_sec, block_read_bytes_per_sec, block_write_bytes_per_sec
		FROM samples WHERE container_id = ? AND observed_ns >= ? AND observed_ns < ?
		ORDER BY observed_ns LIMIT ?`, id, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()
	var result []Sample
	for rows.Next() {
		var ns int64
		var o runtime.Observations
		var usage, limit sql.NullString
		if err := rows.Scan(&ns, &o.CPUPercent, &usage, &limit,
			&o.NetworkRxBytesPerSec, &o.NetworkTxBytesPerSec, &o.BlockReadBytesPerSec, &o.BlockWriteBytesPerSec); err != nil {
			return nil, fmt.Errorf("read metric sample: %w", err)
		}
		if o.MemoryUsageBytes, err = parseUint(usage); err != nil {
			return nil, fmt.Errorf("read metric memory usage: %w", err)
		}
		if o.MemoryLimitBytes, err = parseUint(limit); err != nil {
			return nil, fmt.Errorf("read metric memory limit: %w", err)
		}
		o.Time = time.Unix(0, ns).UTC()
		result = append(result, Sample{ContainerID: id, Observation: o})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metric samples: %w", err)
	}
	return result, nil
}

// ExpireBefore removes samples strictly before the cutoff, not the cutoff itself.
// Logical deletion does not guarantee filesystem space is returned.
func (s *Store) ExpireBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	ns, err := timestamp(cutoff)
	if err != nil {
		return 0, err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM samples WHERE observed_ns < ?`, ns)
	if err != nil {
		return 0, fmt.Errorf("expire metrics: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired metrics: %w", err)
	}
	return count, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close metrics database: %w", err)
	}
	return nil
}
