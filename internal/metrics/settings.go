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
	ProfileOff    Profile = "off"
	ProfileLow    Profile = "low"
	ProfileMedium Profile = "medium"
	ProfileHigh   Profile = "high"
	ProfileCustom Profile = "custom"

	ScopeActivated CollectionScope = "activated"
	ScopeAll       CollectionScope = "all"
)

const (
	DefaultMetricsStorageLimit int64 = 128_000_000
	DefaultLogsStorageLimit    int64 = 256_000_000
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

type LogSettings struct {
	Configured     bool
	Enabled        bool
	Profile        Profile
	Scope          CollectionScope
	DeliveryPreset Profile
	StorageLimit   int64
	Revision       int64
}

type SourceSettings struct {
	OverallProfile     Profile
	CollectionRevision int64
	RestoreStats       bool
	RestoreState       bool
	RestoreLogs        bool
	StateEnabled       bool
	StateScope         CollectionScope
	StateRevision      int64
	Logs               LogSettings
}

func defaultSourceSettings() SourceSettings {
	return SourceSettings{
		StateScope: ScopeActivated,
		Logs: LogSettings{
			Scope: ScopeActivated, StorageLimit: DefaultLogsStorageLimit,
		},
	}
}

func (s LogSettings) validate() error {
	if s.Scope != ScopeActivated && s.Scope != ScopeAll {
		return errors.New("invalid log collection scope")
	}
	if s.StorageLimit <= 0 || s.Revision < 0 {
		return errors.New("invalid log storage limit or revision")
	}
	if !s.Configured {
		if s.Enabled || s.Profile != "" || s.DeliveryPreset != "" {
			return errors.New("unconfigured logs cannot be enabled or have saved preferences")
		}
		return nil
	}
	switch s.DeliveryPreset {
	case ProfileLow, ProfileMedium, ProfileHigh:
	default:
		return errors.New("invalid log delivery preset")
	}
	switch s.Profile {
	case ProfileLow:
		if s.Scope != ScopeActivated || s.DeliveryPreset != ProfileLow {
			return errors.New("low logs require activated scope and low delivery")
		}
	case ProfileMedium:
		if s.Scope != ScopeActivated || s.DeliveryPreset != ProfileMedium {
			return errors.New("medium logs require activated scope and medium delivery")
		}
	case ProfileHigh:
		if s.Scope != ScopeAll || s.DeliveryPreset != ProfileHigh {
			return errors.New("high logs require all-container scope and live delivery")
		}
	case ProfileCustom:
	default:
		return errors.New("invalid log profile")
	}
	return nil
}

func (s SourceSettings) validate() error {
	switch s.OverallProfile {
	case "", ProfileOff, ProfileLow, ProfileMedium, ProfileHigh, ProfileCustom:
	default:
		return errors.New("invalid overall collection profile")
	}
	if s.CollectionRevision < 0 || s.CollectionRevision == math.MaxInt64 {
		return errors.New("invalid collection settings revision")
	}
	if s.StateScope != ScopeActivated && s.StateScope != ScopeAll {
		return errors.New("invalid state collection scope")
	}
	if s.StateRevision < 0 {
		return errors.New("invalid state settings revision")
	}
	if err := s.Logs.validate(); err != nil {
		return err
	}
	return nil
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

type settingsReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) Load(ctx context.Context) (Settings, error) {
	return loadMetricsSettings(ctx, s.db)
}

func loadMetricsSettings(ctx context.Context, reader settingsReader) (Settings, error) {
	var result Settings
	var interval int64
	err := reader.QueryRowContext(ctx, `SELECT enabled, profile, scope, interval_ns, storage_limit_bytes, revision
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
	sources, err := s.LoadSources(ctx)
	if err != nil {
		return Settings{}, err
	}
	sources.OverallProfile = ""
	saved, _, err := s.SaveConfiguration(ctx, settings, sources)
	return saved, err
}

func (s *SettingsStore) LoadSources(ctx context.Context) (SourceSettings, error) {
	return loadSourceSettings(ctx, s.db)
}

func (s *SettingsStore) LoadConfiguration(ctx context.Context) (Settings, SourceSettings, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("begin configuration read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := loadMetricsSettings(ctx, tx)
	if err != nil {
		return Settings{}, SourceSettings{}, err
	}
	sources, err := loadSourceSettings(ctx, tx)
	if err != nil {
		return Settings{}, SourceSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("complete configuration read: %w", err)
	}
	return settings, sources, nil
}

func loadSourceSettings(ctx context.Context, reader settingsReader) (SourceSettings, error) {
	result := defaultSourceSettings()
	var stateRevision int64
	err := reader.QueryRowContext(ctx, `SELECT enabled, scope, revision FROM state_settings WHERE id = 1`).
		Scan(&result.StateEnabled, &result.StateScope, &stateRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SourceSettings{}, fmt.Errorf("read state settings: %w", err)
	}
	result.StateRevision = stateRevision
	var logs LogSettings
	var profile, deliveryPreset sql.NullString
	err = reader.QueryRowContext(ctx, `SELECT configured, enabled, profile, scope, delivery_preset, storage_limit_bytes, revision
		FROM log_settings WHERE id = 1`).
		Scan(&logs.Configured, &logs.Enabled, &profile, &logs.Scope, &deliveryPreset, &logs.StorageLimit, &logs.Revision)
	if err == nil {
		logs.Profile = Profile(profile.String)
		logs.DeliveryPreset = Profile(deliveryPreset.String)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SourceSettings{}, fmt.Errorf("read log settings: %w", err)
	} else {
		logs = result.Logs
	}
	result.Logs = logs
	err = reader.QueryRowContext(ctx, `SELECT profile, restore_stats, restore_state, restore_logs, revision
		FROM collection_settings WHERE id = 1`).Scan(&result.OverallProfile, &result.RestoreStats,
		&result.RestoreState, &result.RestoreLogs, &result.CollectionRevision)
	if err != nil {
		return SourceSettings{}, fmt.Errorf("read overall collection settings: %w", err)
	}
	if err := result.validate(); err != nil {
		return SourceSettings{}, fmt.Errorf("invalid stored source settings: %w", err)
	}
	return result, nil
}

// SaveConfiguration persists metrics and independent history-source settings
// in one transaction. Revision zero is valid only for a source's first save.
func (s *SettingsStore) SaveConfiguration(ctx context.Context, settings Settings, sources SourceSettings) (Settings, SourceSettings, error) {
	if err := settings.validate(); err != nil {
		return Settings{}, SourceSettings{}, err
	}
	if settings.Revision < 0 || settings.Revision == math.MaxInt64 {
		return Settings{}, SourceSettings{}, errors.New("invalid metrics settings revision")
	}
	if err := sources.validate(); err != nil {
		return Settings{}, SourceSettings{}, err
	}
	if sources.StateRevision == math.MaxInt64 || sources.Logs.Revision == math.MaxInt64 {
		return Settings{}, SourceSettings{}, errors.New("invalid source settings revision")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("begin observability settings save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if sources.OverallProfile == "" {
		current, err := loadSourceSettings(ctx, tx)
		if err != nil {
			return Settings{}, SourceSettings{}, err
		}
		sources.CollectionRevision = current.CollectionRevision
		sources.OverallProfile = InferCollectionProfile(settings, sources)
	}
	if sources.OverallProfile != ProfileCustom && sources.OverallProfile != InferCollectionProfile(settings, sources) {
		sources.OverallProfile = InferCollectionProfile(settings, sources)
	}
	if err := saveMetricsSettingsTx(ctx, tx, settings); err != nil {
		return Settings{}, SourceSettings{}, err
	}
	if err := saveStateSettingsTx(ctx, tx, sources); err != nil {
		return Settings{}, SourceSettings{}, err
	}
	if err := saveLogSettingsTx(ctx, tx, sources.Logs); err != nil {
		return Settings{}, SourceSettings{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE collection_settings SET profile = ?, restore_stats = ?,
		restore_state = ?, restore_logs = ?, revision = revision + 1 WHERE id = 1 AND revision = ?`,
		sources.OverallProfile, sources.RestoreStats, sources.RestoreState, sources.RestoreLogs, sources.CollectionRevision)
	if err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("save overall collection settings: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("check collection settings update: %w", err)
	}
	if count != 1 {
		return Settings{}, SourceSettings{}, ErrStaleSettings
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, SourceSettings{}, fmt.Errorf("commit observability settings: %w", err)
	}
	settings.Revision++
	sources.StateRevision++
	sources.Logs.Revision++
	sources.CollectionRevision++
	return settings, sources, nil
}

func InferCollectionProfile(stats Settings, sources SourceSettings) Profile {
	if !stats.Enabled && !sources.StateEnabled && !sources.Logs.Enabled {
		return ProfileOff
	}
	if stats.Enabled && sources.StateEnabled && sources.Logs.Enabled && stats.Profile == sources.Logs.Profile {
		switch stats.Profile {
		case ProfileLow, ProfileMedium:
			if sources.StateScope == ScopeActivated && sources.Logs.Scope == ScopeActivated {
				return stats.Profile
			}
		case ProfileHigh:
			if sources.StateScope == ScopeAll && sources.Logs.Scope == ScopeAll {
				return ProfileHigh
			}
		}
	}
	return ProfileCustom
}

func saveMetricsSettingsTx(ctx context.Context, tx *sql.Tx, settings Settings) error {
	var result sql.Result
	var err error
	if settings.Revision == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO metrics_settings
			(id, enabled, profile, scope, interval_ns, storage_limit_bytes, revision)
			VALUES (1, ?, ?, ?, ?, ?, 1) ON CONFLICT(id) DO NOTHING`,
			settings.Enabled, settings.Profile, settings.Scope, int64(settings.Interval), settings.StorageLimit)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE metrics_settings SET
			enabled = ?, profile = ?, scope = ?, interval_ns = ?, storage_limit_bytes = ?, revision = revision + 1
			WHERE id = 1 AND revision = ?`,
			settings.Enabled, settings.Profile, settings.Scope, int64(settings.Interval), settings.StorageLimit, settings.Revision)
	}
	if err != nil {
		return fmt.Errorf("save metrics settings: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check metrics settings update: %w", err)
	}
	if count != 1 {
		return ErrStaleSettings
	}
	return nil
}

func saveStateSettingsTx(ctx context.Context, tx *sql.Tx, sources SourceSettings) error {
	var result sql.Result
	var err error
	if sources.StateRevision == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO state_settings (id, enabled, scope, revision)
			VALUES (1, ?, ?, 1) ON CONFLICT(id) DO NOTHING`, sources.StateEnabled, sources.StateScope)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE state_settings SET enabled = ?, scope = ?, revision = revision + 1
			WHERE id = 1 AND revision = ?`, sources.StateEnabled, sources.StateScope, sources.StateRevision)
	}
	if err != nil {
		return fmt.Errorf("save state settings: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check state settings update: %w", err)
	}
	if count != 1 {
		return ErrStaleSettings
	}
	return nil
}

func saveLogSettingsTx(ctx context.Context, tx *sql.Tx, settings LogSettings) error {
	var profile, deliveryPreset any
	if settings.Configured {
		profile, deliveryPreset = settings.Profile, settings.DeliveryPreset
	}
	var result sql.Result
	var err error
	if settings.Revision == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO log_settings
			(id, configured, enabled, profile, scope, delivery_preset, storage_limit_bytes, revision)
			VALUES (1, ?, ?, ?, ?, ?, ?, 1) ON CONFLICT(id) DO NOTHING`,
			settings.Configured, settings.Enabled, profile, settings.Scope, deliveryPreset, settings.StorageLimit)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE log_settings SET
			configured = ?, enabled = ?, profile = ?, scope = ?, delivery_preset = ?, storage_limit_bytes = ?, revision = revision + 1
			WHERE id = 1 AND revision = ?`,
			settings.Configured, settings.Enabled, profile, settings.Scope, deliveryPreset, settings.StorageLimit, settings.Revision)
	}
	if err != nil {
		return fmt.Errorf("save log settings: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check log settings update: %w", err)
	}
	if count != 1 {
		return ErrStaleSettings
	}
	return nil
}
