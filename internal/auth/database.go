package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 3

var (
	ErrBootstrapComplete = errors.New("initial administrator is already provisioned")
	ErrNoUser            = errors.New("administrator account is missing")
)

type Database struct {
	SQL      *sql.DB
	Sessions *SQLSessionStore
}

type User struct {
	ID           string
	Username     string
	PasswordHash string
	MustChange   bool
	AuthVersion  int64
	CreatedAt    time.Time
}

func OpenDatabase(ctx context.Context, dataDir string) (*Database, error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	databasePath := filepath.Join(dataDir, "app.db")
	if info, err := os.Lstat(databasePath); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("application database must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect application database: %w", err)
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open application database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	closeOnError := func(err error) (*Database, error) {
		_ = db.Close()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(fmt.Errorf("connect application database: %w", err))
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		return closeOnError(fmt.Errorf("restrict application database permissions: %w", err))
	}
	if err := configureSQLite(ctx, db); err != nil {
		return closeOnError(err)
	}
	if err := migrate(ctx, db); err != nil {
		return closeOnError(err)
	}
	if err := validateState(ctx, db); err != nil {
		return closeOnError(err)
	}
	return &Database{SQL: db, Sessions: NewSQLSessionStore(db)}, nil
}

func configureSQLite(ctx context.Context, db *sql.DB) error {
	for _, statement := range []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA journal_mode = WAL`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure application database: %w", err)
		}
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin application database migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// This first write serializes concurrent first-start migration attempts.
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY NOT NULL
	)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	var currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if currentVersion > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", currentVersion, schemaVersion)
	}
	if currentVersion == 0 {
		var applicationTables int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'
		`).Scan(&applicationTables); err != nil {
			return fmt.Errorf("inspect unversioned application database: %w", err)
		}
		if applicationTables != 0 {
			return errors.New("application database has tables but no migration version")
		}
		if err := migrateV1(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (1)`); err != nil {
			return fmt.Errorf("record database schema version: %w", err)
		}
		currentVersion = 1
	}
	if currentVersion == 1 {
		if err := migrateV2(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (2)`); err != nil {
			return fmt.Errorf("record database schema version: %w", err)
		}
		currentVersion = 2
	}
	if currentVersion == 2 {
		if err := migrateV3(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (3)`); err != nil {
			return fmt.Errorf("record database schema version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit application database migration: %w", err)
	}
	return nil
}

func migrateV3(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`CREATE TABLE discovery_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			root TEXT NOT NULL,
			revision INTEGER NOT NULL CHECK (revision > 0)
		)`,
		`CREATE TABLE discovered_projects (
			path TEXT PRIMARY KEY NOT NULL,
			root TEXT NOT NULL,
			active INTEGER NOT NULL CHECK (active IN (0, 1)),
			available INTEGER NOT NULL CHECK (available IN (0, 1)),
			resolution_error TEXT NOT NULL
		)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database migration v3: %w", err)
		}
	}
	return nil
}

func migrateV2(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE sessions ADD COLUMN deadline_unix INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN revoked INTEGER NOT NULL DEFAULT 0`,
		`UPDATE sessions SET deadline_unix = expiry_unix + 43200`,
		`CREATE INDEX sessions_deadline_idx ON sessions(deadline_unix)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database migration v2: %w", err)
		}
	}
	return nil
}

func migrateV1(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE app_state (
			key TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL
		)`,
		`INSERT INTO app_state(key, value) VALUES ('bootstrap_complete', '0')`,
		`INSERT INTO app_state(key, value) VALUES ('csrf_secret', '')`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			must_change_password INTEGER NOT NULL CHECK (must_change_password IN (0, 1)),
			auth_version INTEGER NOT NULL CHECK (auth_version > 0),
			created_at_unix INTEGER NOT NULL
		)`,
		`CREATE TABLE sessions (
			token TEXT PRIMARY KEY NOT NULL,
			data BLOB NOT NULL,
			expiry_unix INTEGER NOT NULL
		)`,
		`CREATE INDEX sessions_expiry_idx ON sessions(expiry_unix)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database migration v1: %w", err)
		}
	}
	return nil
}

func validateState(ctx context.Context, db *sql.DB) error {
	var bootstrapComplete string
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = 'bootstrap_complete'`).Scan(&bootstrapComplete); err != nil {
		return fmt.Errorf("read authentication bootstrap state: %w", err)
	}
	var users int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return fmt.Errorf("read administrator state: %w", err)
	}
	if bootstrapComplete == "1" && users != 1 {
		return errors.New("application authentication state is inconsistent")
	}
	if bootstrapComplete == "0" && users != 0 {
		return errors.New("application bootstrap state is inconsistent")
	}
	if bootstrapComplete != "0" && bootstrapComplete != "1" {
		return errors.New("application bootstrap state is invalid")
	}
	return nil
}

func (database *Database) NeedsBootstrap(ctx context.Context) (bool, error) {
	var value string
	err := database.SQL.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = 'bootstrap_complete'`).Scan(&value)
	if err != nil {
		return false, err
	}
	return value == "0", nil
}

func (database *Database) ProvisionInitialAdmin(ctx context.Context, username, passwordHash string) (User, error) {
	var user User
	if username == "" || passwordHash == "" {
		return user, errors.New("bootstrap username and password hash are required")
	}
	id, err := newUserID()
	if err != nil {
		return user, fmt.Errorf("generate administrator identity: %w", err)
	}
	now := time.Now().UTC()
	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		return user, fmt.Errorf("begin administrator provisioning: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `UPDATE app_state SET value = '1' WHERE key = 'bootstrap_complete' AND value = '0'`)
	if err != nil {
		return user, fmt.Errorf("reserve initial administrator provisioning: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return user, fmt.Errorf("verify initial administrator provisioning: %w", err)
	}
	if updated != 1 {
		return user, ErrBootstrapComplete
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, must_change_password, auth_version, created_at_unix)
		VALUES (?, ?, ?, 1, 1, ?)
	`, id, username, passwordHash, now.Unix()); err != nil {
		return user, fmt.Errorf("save initial administrator: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return user, fmt.Errorf("commit initial administrator: %w", err)
	}
	return User{ID: id, Username: username, PasswordHash: passwordHash, MustChange: true, AuthVersion: 1, CreatedAt: now}, nil
}

func (database *Database) UserByUsername(ctx context.Context, username string) (User, error) {
	var user User
	var mustChange int
	var createdAt int64
	err := database.SQL.QueryRowContext(ctx, `
		SELECT id, username, password_hash, must_change_password, auth_version, created_at_unix
		FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &mustChange, &user.AuthVersion, &createdAt)
	if err == sql.ErrNoRows {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, fmt.Errorf("read administrator account: %w", err)
	}
	user.MustChange = mustChange == 1
	user.CreatedAt = time.Unix(createdAt, 0).UTC()
	return user, nil
}

func (database *Database) UserByID(ctx context.Context, id string) (User, error) {
	var user User
	var mustChange int
	var createdAt int64
	err := database.SQL.QueryRowContext(ctx, `
		SELECT id, username, password_hash, must_change_password, auth_version, created_at_unix
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &mustChange, &user.AuthVersion, &createdAt)
	if err == sql.ErrNoRows {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, fmt.Errorf("read account identity: %w", err)
	}
	user.MustChange = mustChange == 1
	user.CreatedAt = time.Unix(createdAt, 0).UTC()
	return user, nil
}

func (database *Database) ChangePassword(ctx context.Context, id string, currentVersion int64, passwordHash string) (int64, error) {
	result, err := database.SQL.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, must_change_password = 0, auth_version = auth_version + 1
		WHERE id = ? AND auth_version = ?
	`, passwordHash, id, currentVersion)
	if err != nil {
		return 0, fmt.Errorf("save password change: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("verify password change: %w", err)
	}
	if updated != 1 {
		return 0, errors.New("account changed; restart password setup")
	}
	return currentVersion + 1, nil
}

func newUserID() (string, error) {
	return randomHex(16)
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
