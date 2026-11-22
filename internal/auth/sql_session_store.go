package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/scs/v2"
)

var _ scs.Store = (*SQLSessionStore)(nil)

type SQLSessionStore struct {
	db *sql.DB
}

func NewSQLSessionStore(db *sql.DB) *SQLSessionStore {
	return &SQLSessionStore{db: db}
}

func (store *SQLSessionStore) Commit(token string, data []byte, expiry time.Time) error {
	deadline, _, err := (scs.GobCodec{}).Decode(data)
	if err != nil {
		return fmt.Errorf("decode session deadline: %w", err)
	}
	now := time.Now().Unix()
	if expiry.Unix() <= now || deadline.Unix() <= now {
		return errors.New("cannot commit expired session")
	}
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM sessions WHERE deadline_unix <= ?`, now); err != nil {
		return fmt.Errorf("remove expired sessions: %w", err)
	}
	if _, err := tx.Exec(`UPDATE sessions SET revoked = 1, data = X'' WHERE revoked = 0 AND expiry_unix <= ?`, now); err != nil {
		return fmt.Errorf("expire idle sessions: %w", err)
	}
	result, err := tx.Exec(`
		INSERT INTO sessions (token, data, expiry_unix, deadline_unix, revoked)
		VALUES (?, ?, ?, ?, 0)
		ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry_unix = excluded.expiry_unix,
			deadline_unix = excluded.deadline_unix
		WHERE sessions.revoked = 0 AND sessions.expiry_unix > ? AND sessions.deadline_unix > ?
	`, token, data, expiry.Unix(), deadline.Unix(), now, now)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return errors.New("session has been revoked or expired")
	}
	return tx.Commit()
}

func (store *SQLSessionStore) Find(token string) ([]byte, bool, error) {
	var data []byte
	var expiry int64
	err := store.db.QueryRow(`
		SELECT data, expiry_unix FROM sessions WHERE token = ? AND revoked = 0
	`, token).Scan(&data, &expiry)
	if err == sql.ErrNoRows || (err == nil && time.Now().Unix() >= expiry) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (store *SQLSessionStore) Delete(token string) error {
	_, err := store.db.Exec(`UPDATE sessions SET revoked = 1, data = X'' WHERE token = ?`, token)
	return err
}
