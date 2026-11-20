package auth

import (
	"bytes"
	"database/sql"
	"testing"
	"time"

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
		expiry_unix INTEGER NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}

	store := NewSQLSessionStore(db)
	expiry := time.Now().Add(time.Hour)
	if err := store.Commit("session", []byte("first"), expiry); err != nil {
		t.Fatal(err)
	}
	data, found, err := store.Find("session")
	if err != nil || !found || !bytes.Equal(data, []byte("first")) {
		t.Fatalf("Find() = (%q, %v, %v)", data, found, err)
	}

	if err := store.Commit("session", []byte("updated"), expiry); err != nil {
		t.Fatal(err)
	}
	data, found, err = store.Find("session")
	if err != nil || !found || !bytes.Equal(data, []byte("updated")) {
		t.Fatalf("Find() after update = (%q, %v, %v)", data, found, err)
	}

	if err := store.Commit("expired", []byte("old"), time.Now().Add(-time.Second)); err != nil {
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
}
