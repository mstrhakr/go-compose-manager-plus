// Package metrics provides persistence and opt-in collection for container observations.
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
	"sort"
	"strconv"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
	_ "modernc.org/sqlite"
)

const schemaVersion = 10

// ErrWriteOutcomeUnknown means a database commit failed and its outcome must
// be reconciled before retrying the same batch.
var ErrWriteOutcomeUnknown = errors.New("history write commit outcome is unknown")

type Sample struct {
	ContainerID string
	Observation runtime.Observations
}

type ChartPoint struct {
	Time                 time.Time
	CPUPercent           *float64
	MemoryUsageBytes     *float64
	NetworkRxBytesPerSec *float64
	NetworkTxBytesPerSec *float64
}

type ContainerChartSeries struct {
	ContainerID string
	Points      []ChartPoint
}

type Store struct {
	db   *sql.DB
	path string
	kind historyDatabase
}

type historyDatabase uint8

const (
	metricsDatabase historyDatabase = iota
	logsDatabase
)

// Open uses a separate database from application state without starting collection.
func Open(ctx context.Context, dataDir string) (*Store, error) {
	return open(ctx, dataDir, false)
}

func open(ctx context.Context, dataDir string, exclusive bool) (*Store, error) {
	return openHistory(ctx, dataDir, metricsDatabase, exclusive)
}

func openHistory(ctx context.Context, dataDir string, kind historyDatabase, exclusive bool) (*Store, error) {
	var name string
	var migrate func(context.Context, *sql.DB) error
	switch kind {
	case metricsDatabase:
		name, migrate = "metrics", initialize
	case logsDatabase:
		name, migrate = "logs", initializeLogs
	default:
		return nil, errors.New("invalid history database")
	}
	if dataDir == "" {
		return nil, fmt.Errorf("%s data directory is required", name)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s directory: %w", name, err)
	}
	path := filepath.Join(dataDir, name+".db")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s database must be a regular file", name)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect %s database: %w", name, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", name, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(err error) (*Store, error) {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s database: %w", name, closeErr))
		}
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("connect %s database: %w", name, err))
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fail(fmt.Errorf("restrict %s database permissions: %w", name, err))
	}
	for _, statement := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA synchronous = FULL`,
		`PRAGMA journal_mode = WAL`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fail(fmt.Errorf("configure %s database: %w", name, err))
		}
	}
	if exclusive {
		var mode string
		if err := db.QueryRowContext(ctx, `PRAGMA locking_mode = EXCLUSIVE`).Scan(&mode); err != nil {
			return fail(fmt.Errorf("reserve %s database: %w", name, err))
		}
		if mode != "exclusive" {
			return fail(fmt.Errorf("%s database refused exclusive locking: %s", name, mode))
		}
	}
	if err := migrate(ctx, db); err != nil {
		return fail(err)
	}
	return &Store{db: db, path: path, kind: kind}, nil
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
		version = 5
	}
	if version == 5 {
		for _, statement := range []string{
			`CREATE TABLE pending_write (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				kind TEXT NOT NULL CHECK (kind IN ('samples', 'events', 'states')),
				created_ns INTEGER NOT NULL,
				committed INTEGER NOT NULL CHECK (committed IN (0, 1))
			)`,
			`PRAGMA user_version = 6`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 6: %w", err)
			}
		}
		version = 6
	}
	if version == 6 {
		for _, statement := range []string{
			`CREATE TABLE event_dedup (
				container_id TEXT NOT NULL,
				observed_ns INTEGER NOT NULL,
				kind TEXT NOT NULL,
				health TEXT NOT NULL,
				PRIMARY KEY (container_id, observed_ns, kind, health)
			) WITHOUT ROWID`,
			`INSERT OR IGNORE INTO event_dedup SELECT container_id, observed_ns, kind, health FROM container_events`,
			`PRAGMA user_version = 7`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 7: %w", err)
			}
		}
		version = 7
	}
	if version == 7 {
		for _, statement := range []string{
			createLogsTable,
			createLogsTimeIndex,
			createLogsContainerIndex,
			`PRAGMA user_version = 8`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 8: %w", err)
			}
		}
		version = 8
	}
	if version == 8 {
		for _, statement := range []string{
			`ALTER TABLE container_states ADD COLUMN reconnect_observed INTEGER NOT NULL DEFAULT 0 CHECK (reconnect_observed IN (0, 1))`,
			`CREATE TABLE container_state_baselines (
				container_id TEXT PRIMARY KEY,
				observed_ns INTEGER NOT NULL,
				state TEXT NOT NULL,
				health TEXT NOT NULL,
				reconnect_observed INTEGER NOT NULL CHECK (reconnect_observed IN (0, 1))
			) WITHOUT ROWID`,
			`CREATE TABLE container_state_gaps (
				sequence INTEGER PRIMARY KEY,
				container_id TEXT NOT NULL,
				started_ns INTEGER NOT NULL,
				ended_ns INTEGER,
				CHECK (ended_ns IS NULL OR ended_ns > started_ns)
			)`,
			`CREATE INDEX container_state_gaps_range ON container_state_gaps(container_id, started_ns, ended_ns)`,
			`CREATE UNIQUE INDEX container_state_gaps_open ON container_state_gaps(container_id) WHERE ended_ns IS NULL`,
			`PRAGMA user_version = 9`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 9: %w", err)
			}
		}
		version = 9
	}
	if version == 9 {
		for _, statement := range []string{
			`CREATE TABLE state_source_gaps (
				sequence INTEGER PRIMARY KEY,
				started_ns INTEGER NOT NULL,
				ended_ns INTEGER,
				CHECK (ended_ns IS NULL OR ended_ns > started_ns)
			)`,
			`CREATE INDEX state_source_gaps_range ON state_source_gaps(started_ns, ended_ns)`,
			`CREATE UNIQUE INDEX state_source_gaps_open ON state_source_gaps((1)) WHERE ended_ns IS NULL`,
			`PRAGMA user_version = 10`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate metrics schema to version 10: %w", err)
			}
		}
		version = 10
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
	return s.insert(ctx, samples, false)
}

func (s *Store) insert(ctx context.Context, samples []Sample, pending bool) error {
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
	if pending {
		if err := markPendingCommitted(ctx, tx); err != nil {
			return err
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

// SelectChartSeries returns at most one recorded observation per container and
// display bucket. It does not combine values across containers; missing buckets
// remain nil and display selection does not alter retained samples.
// An empty id selects all containers; a nonempty id scopes rows before ranking.
func (s *Store) SelectChartSeries(ctx context.Context, id string, start, end time.Time, bucket time.Duration) ([]ContainerChartSeries, error) {
	if id != "" {
		if err := validateID(id); err != nil {
			return nil, err
		}
	}
	from, err := timestamp(start)
	if err != nil {
		return nil, fmt.Errorf("invalid chart start: %w", err)
	}
	to, err := timestamp(end)
	if err != nil {
		return nil, fmt.Errorf("invalid chart end: %w", err)
	}
	rangeDuration := end.Sub(start)
	if from >= to || rangeDuration <= 0 || rangeDuration > time.Hour || bucket <= 0 {
		return nil, errors.New("invalid chart range or bucket")
	}
	bucketNS := int64(bucket)
	rangeNS := int64(rangeDuration)
	if bucketNS <= 0 || (rangeNS-1)/bucketNS+1 > 120 {
		return nil, errors.New("chart range exceeds 120 buckets")
	}
	bucketCount := int((rangeNS-1)/bucketNS + 1)
	filter := ""
	args := []any{from, bucketNS, from, bucketNS, from, to}
	if id != "" {
		filter = " AND container_id = ?"
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `WITH ranked AS (
		SELECT container_id, observed_ns,
			((observed_ns - ?) / ?) AS bucket_index,
			cpu_percent, CAST(memory_usage_bytes AS REAL) AS memory_usage_bytes,
			network_rx_bytes_per_sec, network_tx_bytes_per_sec,
			ROW_NUMBER() OVER (
				PARTITION BY container_id, ((observed_ns - ?) / ?)
				ORDER BY observed_ns DESC
			) AS sample_rank
		FROM samples
		WHERE observed_ns >= ? AND observed_ns < ?`+filter+`
	)
	SELECT container_id, bucket_index, observed_ns, cpu_percent, memory_usage_bytes,
		network_rx_bytes_per_sec, network_tx_bytes_per_sec
	FROM ranked WHERE sample_rank = 1
	ORDER BY container_id, bucket_index`, args...)
	if err != nil {
		return nil, fmt.Errorf("query chart metrics: %w", err)
	}
	defer rows.Close()
	type containerBuckets struct {
		id     string
		points []ChartPoint
	}
	seriesByID := make(map[string]*containerBuckets)
	for rows.Next() {
		var id string
		var index int
		var ns int64
		var point ChartPoint
		var cpu, memory, rx, tx sql.NullFloat64
		if err := rows.Scan(&id, &index, &ns, &cpu, &memory, &rx, &tx); err != nil {
			return nil, fmt.Errorf("read chart metrics: %w", err)
		}
		series := seriesByID[id]
		if series == nil {
			points := make([]ChartPoint, bucketCount)
			for i := range points {
				points[i].Time = start.Add(time.Duration(i) * bucket)
			}
			series = &containerBuckets{id: id, points: points}
			seriesByID[id] = series
		}
		point.Time = time.Unix(0, ns).UTC()
		point.CPUPercent = nullableFloat(cpu)
		point.MemoryUsageBytes = nullableFloat(memory)
		point.NetworkRxBytesPerSec = nullableFloat(rx)
		point.NetworkTxBytesPerSec = nullableFloat(tx)
		if index < 0 || index >= len(series.points) {
			return nil, errors.New("chart query returned an invalid bucket")
		}
		series.points[index] = point
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read chart metrics: %w", err)
	}
	result := make([]ContainerChartSeries, 0, len(seriesByID))
	for _, series := range seriesByID {
		result = append(result, ContainerChartSeries{ContainerID: series.id, Points: series.points})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ContainerID < result[j].ContainerID })
	return result, nil
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
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
