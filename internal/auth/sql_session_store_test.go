package auth

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	_ "modernc.org/sqlite"
)

func TestSQLSessionStoreModerncRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE sessions (
		token TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		expiry_unix INTEGER NOT NULL,
		deadline_unix INTEGER NOT NULL,
		revoked INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}

	store := NewSQLSessionStore(db)
	expiry := time.Now().Add(time.Hour)
	first, err := (scs.GobCodec{}).Encode(expiry, map[string]interface{}{"value": "first"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit("session", first, expiry); err != nil {
		t.Fatal(err)
	}
	data, found, err := store.Find("session")
	if err != nil || !found || !bytes.Equal(data, first) {
		t.Fatalf("Find() = (%q, %v, %v)", data, found, err)
	}

	updated, err := (scs.GobCodec{}).Encode(expiry, map[string]interface{}{"value": "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit("session", updated, expiry); err != nil {
		t.Fatal(err)
	}
	data, found, err = store.Find("session")
	if err != nil || !found || !bytes.Equal(data, updated) {
		t.Fatalf("Find() after update = (%q, %v, %v)", data, found, err)
	}

	if _, err := db.Exec(`INSERT INTO sessions VALUES ('expired', ?, ?, ?, 0)`, first, time.Now().Add(-time.Second).Unix(), expiry.Unix()); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Find("expired"); err != nil || found {
		t.Fatalf("expired session found=%v err=%v", found, err)
	}

	if err := store.Delete("session"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Find("session"); err != nil || found {
		t.Fatalf("deleted session found=%v err=%v", found, err)
	}
	if err := store.Commit("session", updated, expiry); err == nil {
		t.Fatal("stale commit restored revoked session")
	}
	if err := store.Commit("new", first, expiry); err != nil {
		t.Fatal(err)
	}
	var revoked int
	var dataLength int
	if err := db.QueryRow(`SELECT revoked, length(data) FROM sessions WHERE token = 'expired'`).Scan(&revoked, &dataLength); err != nil {
		t.Fatal(err)
	}
	if revoked != 1 || dataLength != 0 {
		t.Fatalf("expired session retained data: revoked=%d length=%d", revoked, dataLength)
	}
	if _, err := db.Exec(`UPDATE sessions SET deadline_unix = ? WHERE token = 'expired'`, time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit("new", first, expiry); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = 'expired'`).Scan(&revoked); err != nil || revoked != 0 {
		t.Fatalf("expired tombstone count=%d err=%v", revoked, err)
	}
}

func TestLogoutCannotBeUndoneByStaleSessionCommit(t *testing.T) {
	database, err := OpenDatabase(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	sessions := scs.New()
	sessions.Store = database.Sessions
	sessions.IdleTimeout = time.Minute
	sessions.Lifetime = time.Hour
	sessions.HashTokenInStore = true
	ctx, err := sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	sessions.Put(ctx, "user_id", "admin")
	token, _, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := sessions.Load(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	logout, err := sessions.Load(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Destroy(logout); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sessions.Commit(stale); err == nil {
		t.Fatal("stale request recommitted a logged-out session")
	}
	reloaded, err := sessions.Load(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if sessions.GetString(reloaded, "user_id") != "" {
		t.Fatal("logged-out session is still authenticated")
	}
}
