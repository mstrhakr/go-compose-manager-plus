package auth

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func TestManagerBootstrapLoginAndPasswordChange(t *testing.T) {
	ctx := context.Background()
	database, err := OpenDatabase(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = database.SQL.Close() })
	manager, err := NewManager(ctx, database, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Bootstrap(ctx, "operator", "bootstrap-password-123"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Bootstrap(ctx, "ignored", "ignored-bootstrap-password"); err != nil {
		t.Fatalf("restart bootstrap should ignore changed env: %v", err)
	}

	if _, err := manager.Authenticate(ctx, "operator", "wrong-password-123", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("bad login error = %v", err)
	}
	user, err := manager.Authenticate(ctx, "operator", "bootstrap-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChange || user.AuthVersion != 1 {
		t.Fatalf("bootstrap user state = %+v", user)
	}

	sessionContext, err := manager.sessions.Load(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.StartSession(sessionContext, user); err != nil {
		t.Fatal(err)
	}
	if current, authenticated, err := manager.CurrentUser(sessionContext); err != nil || !authenticated || current.ID != user.ID {
		t.Fatalf("CurrentUser() before password change = (%+v, %v, %v)", current, authenticated, err)
	}
	updated, err := manager.ChangePassword(ctx, user, "new-local-password-456")
	if err != nil {
		t.Fatal(err)
	}
	if updated.AuthVersion != 2 || updated.MustChange {
		t.Fatalf("updated user = %+v", updated)
	}
	if _, authenticated, err := manager.CurrentUser(sessionContext); err != nil || authenticated {
		t.Fatalf("old session remained authenticated: authenticated=%v err=%v", authenticated, err)
	}
	updated, err = database.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MustChange || updated.AuthVersion != 2 {
		t.Fatalf("updated account state = %+v", updated)
	}
	if _, err := manager.Authenticate(ctx, "operator", "bootstrap-password-123", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login error = %v", err)
	}
	if _, err := manager.Authenticate(ctx, "operator", "new-local-password-456", "127.0.0.1"); err != nil {
		t.Fatalf("new password login error = %v", err)
	}
}

func TestLoginLimiterBoundsEachClientsKeys(t *testing.T) {
	manager := &Manager{loginAttempts: make(map[string]attempt)}
	for i := 0; i < maxLoginKeysPerClient; i++ {
		key := fmt.Sprintf("client-a\x00user-%d", i)
		if !manager.allowAttempt("client-a", key) {
			t.Fatalf("client A key %d was refused prematurely", i)
		}
	}
	if manager.allowAttempt("client-a", "client-a\x00overflow") {
		t.Fatal("client A exceeded its key capacity")
	}
	if !manager.allowAttempt("client-b", "client-b\x00admin") {
		t.Fatal("client A exhausted capacity for another client")
	}
}

func TestInvalidLoginDoesNotAllocateLimiterEntry(t *testing.T) {
	manager := &Manager{loginAttempts: make(map[string]attempt)}
	for i := 0; i < maxLoginKeys+1; i++ {
		_, err := manager.Authenticate(context.Background(), fmt.Sprintf(" invalid-%d", i), "password", "client-a")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("invalid username: %v", err)
		}
	}
	if len(manager.loginAttempts) != 0 {
		t.Fatalf("invalid usernames allocated %d limiter entries", len(manager.loginAttempts))
	}
}
