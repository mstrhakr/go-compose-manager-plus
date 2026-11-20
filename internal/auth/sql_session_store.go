package auth

import (
	"database/sql"
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
	_, err := store.db.Exec(`
		INSERT INTO sessions (token, data, expiry_unix)
		VALUES (?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry_unix = excluded.expiry_unix
	`, token, data, expiry.Unix())
	return err
}

func (store *SQLSessionStore) Find(token string) ([]byte, bool, error) {
	var data []byte
	var expiry int64
	err := store.db.QueryRow(`
		SELECT data, expiry_unix FROM sessions WHERE token = ?
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
	_, err := store.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}
