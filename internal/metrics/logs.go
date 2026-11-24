package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

const maxLogBatch = 128
const maxLogQuery = 1000
const maxLogQueryBytes = 1 << 20

type Log struct {
	ContainerID string
	Time        time.Time
	ReceivedAt  time.Time
	Stream      runtime.LogStream
	Message     string
	Truncated   bool
}

type LogPage struct {
	Logs []Log
	More bool
}

func validateLog(log Log) (int64, int64, error) {
	if err := validateID(log.ContainerID); err != nil {
		return 0, 0, err
	}
	observed, err := timestamp(log.Time)
	if err != nil || observed <= 0 {
		return 0, 0, errors.New("invalid Docker log observation timestamp")
	}
	received, err := timestamp(log.ReceivedAt)
	if err != nil || received <= 0 {
		return 0, 0, errors.New("invalid Docker log receipt timestamp")
	}
	switch log.Stream {
	case runtime.LogStdout, runtime.LogStderr, runtime.LogTTY:
	default:
		return 0, 0, errors.New("invalid Docker log stream")
	}
	if len(log.Message) > runtime.MaxLogMessageBytes {
		return 0, 0, errors.New("Docker log message exceeds size limit")
	}
	return observed, received, nil
}

// InsertLogs preserves duplicate output and original message bytes. It does
// not start collection or choose replay, pressure eviction or scheduling policy.
// Shared maintenance owns expiry; production writes need managed-store wiring.
func (s *Store) InsertLogs(ctx context.Context, now time.Time, logs []Log) error {
	if len(logs) == 0 || len(logs) > maxLogBatch {
		return errors.New("log batch must contain between 1 and 128 records")
	}
	if _, err := timestamp(now); err != nil {
		return fmt.Errorf("invalid log write time: %w", err)
	}
	for i, log := range logs {
		if _, _, err := validateLog(log); err != nil {
			return fmt.Errorf("invalid log %d: %w", i, err)
		}
		if err := withinHistory(now, log.Time); err != nil {
			return fmt.Errorf("invalid log %d: %w", i, err)
		}
		if log.Time.After(now) || log.ReceivedAt.After(now) {
			return fmt.Errorf("invalid log %d: timestamp is after write time", i)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin log insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO container_logs
		(container_id, observed_ns, received_ns, stream, message, truncated) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare log insert: %w", err)
	}
	defer stmt.Close()
	for i, log := range logs {
		if _, err := stmt.ExecContext(ctx, log.ContainerID, log.Time.UnixNano(), log.ReceivedAt.UnixNano(),
			log.Stream, log.Message, log.Truncated); err != nil {
			return fmt.Errorf("insert log %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit logs: %w", ErrWriteOutcomeUnknown, err)
	}
	return nil
}

// SelectLogs returns newest-first records and explicitly signals omitted rows.
// An empty ID is the aggregate query; selected-container callers must supply ID.
func (s *Store) SelectLogs(ctx context.Context, id string, start, end time.Time, limit int) (LogPage, error) {
	if id != "" {
		if err := validateID(id); err != nil {
			return LogPage{}, err
		}
	}
	from, to, err := historyTimeRange(start, end, limit)
	if err != nil {
		return LogPage{}, err
	}
	if end.Sub(start) > time.Hour || limit > maxLogQuery {
		return LogPage{}, errors.New("log query exceeds one-hour or 1000-record limit")
	}
	query := `SELECT container_id, observed_ns, received_ns, stream, message, truncated
		FROM container_logs WHERE observed_ns >= ? AND observed_ns < ?`
	args := []any{from, to}
	if id != "" {
		query += ` AND container_id = ?`
		args = append(args, id)
	}
	query += ` ORDER BY observed_ns DESC, sequence DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return LogPage{}, fmt.Errorf("query container logs: %w", err)
	}
	defer rows.Close()
	var page LogPage
	messageBytes := 0
	for rows.Next() {
		var log Log
		var observed, received int64
		if err := rows.Scan(&log.ContainerID, &observed, &received, &log.Stream, &log.Message, &log.Truncated); err != nil {
			return LogPage{}, fmt.Errorf("read container log: %w", err)
		}
		log.Time, log.ReceivedAt = time.Unix(0, observed).UTC(), time.Unix(0, received).UTC()
		if _, _, err := validateLog(log); err != nil {
			return LogPage{}, fmt.Errorf("invalid stored log: %w", err)
		}
		if len(page.Logs) == limit || len(log.Message) > maxLogQueryBytes-messageBytes {
			page.More = true
			break
		}
		page.Logs = append(page.Logs, log)
		messageBytes += len(log.Message)
	}
	if err := rows.Err(); err != nil {
		return LogPage{}, fmt.Errorf("read container logs: %w", err)
	}
	return page, nil
}
