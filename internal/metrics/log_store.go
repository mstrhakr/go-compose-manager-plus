package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const logSchemaVersion = 1

const createLogsTable = `CREATE TABLE container_logs (
	sequence INTEGER PRIMARY KEY,
	container_id TEXT NOT NULL,
	observed_ns INTEGER NOT NULL CHECK (observed_ns > 0),
	received_ns INTEGER NOT NULL CHECK (received_ns > 0),
	stream TEXT NOT NULL CHECK (stream IN ('stdout', 'stderr', 'tty')),
	message TEXT NOT NULL CHECK (length(CAST(message AS BLOB)) <= 65536),
	truncated INTEGER NOT NULL CHECK (truncated IN (0, 1))
)`

const createLogsTimeIndex = `CREATE INDEX container_logs_time ON container_logs(observed_ns, sequence)`
const createLogsContainerIndex = `CREATE INDEX container_logs_container_time ON container_logs(container_id, observed_ns, sequence)`

// OpenLogs opens only logs.db. It neither migrates legacy metric-database logs
// nor starts collection; callers use InsertLogs and SelectLogs for its records.
func OpenLogs(ctx context.Context, dataDir string) (*Store, error) {
	return openHistory(ctx, dataDir, logsDatabase, false)
}

// OpenManagedLogs gives logs.db the same exclusive ownership and maintenance
// safeguards as metrics.db, with its own budget, pending writes and evictions.
func OpenManagedLogs(ctx context.Context, dataDir string, limit int64) (*ManagedStore, error) {
	return openManagedHistory(ctx, dataDir, limit, logsDatabase)
}

func initializeLogs(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin log migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read log schema version: %w", err)
	}
	if version < 0 || version > logSchemaVersion {
		return fmt.Errorf("unsupported log schema version %d", version)
	}
	if version == 0 {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&count); err != nil {
			return fmt.Errorf("inspect log schema: %w", err)
		}
		if count != 0 {
			return errors.New("unversioned log database contains tables")
		}
		for _, statement := range []string{
			createLogsTable, createLogsTimeIndex, createLogsContainerIndex,
			`CREATE TABLE pressure_gate (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				active_bytes INTEGER
			)`,
			`INSERT INTO pressure_gate (id, active_bytes) VALUES (1, NULL)`,
			`CREATE TABLE pending_write (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				kind TEXT NOT NULL CHECK (kind = 'logs'),
				created_ns INTEGER NOT NULL,
				committed INTEGER NOT NULL CHECK (committed IN (0, 1))
			)`,
			`CREATE TABLE log_eviction_history (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				log_count INTEGER NOT NULL CHECK (log_count > 0),
				oldest_ns INTEGER NOT NULL,
				newest_ns INTEGER NOT NULL
			)`,
			`PRAGMA user_version = 1`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("create log schema: %w", err)
			}
		}
	}
	for _, statement := range []string{
		`SELECT sequence, container_id, observed_ns, received_ns, stream, message, truncated FROM container_logs LIMIT 0`,
		`SELECT id, active_bytes FROM pressure_gate LIMIT 0`,
		`SELECT id, kind, created_ns, committed FROM pending_write LIMIT 0`,
		`SELECT id, log_count, oldest_ns, newest_ns FROM log_eviction_history LIMIT 0`,
	} {
		rows, err := tx.QueryContext(ctx, statement)
		if err != nil {
			return fmt.Errorf("validate log schema: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close log schema validation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit log migration: %w", err)
	}
	return nil
}
