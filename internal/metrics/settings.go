package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

type Profile string
type CollectionScope string

const (
	ProfileLow    Profile = "low"
	ProfileMedium Profile = "medium"
	ProfileHigh   Profile = "high"
	ProfileCustom Profile = "custom"

	ScopeActivated CollectionScope = "activated"
	ScopeAll       CollectionScope = "all"
)

var ErrStaleSettings = errors.New("metrics settings changed; reload before saving")

// Settings are unconfigured and disabled when Revision is zero. StorageLimit
// is explicit; there are no measured preset size defaults.
type Settings struct {
	Enabled      bool
	Profile      Profile
	Scope        CollectionScope
	Interval     time.Duration
	StorageLimit int64
	Revision     int64
}

func (s Settings) validate() error {
	if s.StorageLimit <= 0 || s.Interval <= 0 {
		return errors.New("metrics interval and storage limit must be positive")
	}
	if s.Scope != ScopeActivated && s.Scope != ScopeAll {
		return errors.New("invalid metrics collection scope")
	}
	switch s.Profile {
	case ProfileLow:
		if s.Scope != ScopeActivated || s.Interval != 15*time.Second {
			return errors.New("low profile requires activated scope and 15-second interval")
		}
	case ProfileMedium:
		if s.Scope != ScopeActivated || s.Interval != 5*time.Second {
			return errors.New("medium profile requires activated scope and 5-second interval")
		}
	case ProfileHigh:
		if s.Scope != ScopeAll || s.Interval != time.Second {
			return errors.New("high profile requires all-container scope and 1-second interval")
		}
	case ProfileCustom:
	default:
		return errors.New("invalid metrics profile")
	}
	return nil
}

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) Load(ctx context.Context) (Settings, error) {
	var result Settings
	var interval int64
	err := s.db.QueryRowContext(ctx, `SELECT enabled, profile, scope, interval_ns, storage_limit_bytes, revision
		FROM metrics_settings WHERE id = 1`).Scan(&result.Enabled, &result.Profile, &result.Scope, &interval, &result.StorageLimit, &result.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read metrics settings: %w", err)
	}
	result.Interval = time.Duration(interval)
	if err := result.validate(); err != nil {
		return Settings{}, fmt.Errorf("invalid stored metrics settings: %w", err)
	}
	if result.Revision < 1 {
		return Settings{}, errors.New("invalid stored metrics settings revision")
	}
	return result, nil
}

// Save uses optimistic concurrency; revision zero is valid only for the first
// save. It does not start collection or alter existing metrics history.
func (s *SettingsStore) Save(ctx context.Context, settings Settings) (Settings, error) {
	if err := settings.validate(); err != nil {
		return Settings{}, err
	}
	if settings.Revision < 0 || settings.Revision == math.MaxInt64 {
		return Settings{}, errors.New("invalid metrics settings revision")
	}
	var result sql.Result
	var err error
	if settings.Revision == 0 {
		result, err = s.db.ExecContext(ctx, `INSERT INTO metrics_settings
			(id, enabled, profile, scope, interval_ns, storage_limit_bytes, revision)
			VALUES (1, ?, ?, ?, ?, ?, 1) ON CONFLICT(id) DO NOTHING`,
			settings.Enabled, settings.Profile, settings.Scope, int64(settings.Interval), settings.StorageLimit)
	} else {
		result, err = s.db.ExecContext(ctx, `UPDATE metrics_settings SET
			enabled = ?, profile = ?, scope = ?, interval_ns = ?, storage_limit_bytes = ?, revision = revision + 1
			WHERE id = 1 AND revision = ?`,
			settings.Enabled, settings.Profile, settings.Scope, int64(settings.Interval), settings.StorageLimit, settings.Revision)
	}
	if err != nil {
		return Settings{}, fmt.Errorf("save metrics settings: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Settings{}, fmt.Errorf("check metrics settings update: %w", err)
	}
	if count != 1 {
		return Settings{}, ErrStaleSettings
	}
	settings.Revision++
	return settings, nil
}
