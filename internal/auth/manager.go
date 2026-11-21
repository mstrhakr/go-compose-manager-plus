package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
	"github.com/alexedwards/scs/v2"
)

const (
	sessionUserIDKey      = "user_id"
	sessionAuthVersionKey = "auth_version"
	sessionMustChangeKey  = "must_change_password"
	maxLoginKeys          = 1024
	maxLoginKeysPerClient = 8
	maxLoginAttempts      = 5
	loginWindow           = time.Minute
	maxPasswordBytes      = 1024
	minPasswordCodepoints = 15
)

var (
	ErrBootstrapRequired  = errors.New("initial administrator credentials are required")
	ErrRateLimited        = errors.New("too many login attempts")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidPassword    = errors.New("password does not meet the minimum length")
	ErrPasswordUnchanged  = errors.New("new password must differ from the current password")
	ErrRestrictedSession  = errors.New("password change required")
)

type attempt struct {
	started time.Time
	count   int
}

type Manager struct {
	database       *Database
	sessions       *scs.SessionManager
	hashSlots      chan struct{}
	dummyHash      string
	trustedProxies []netip.Prefix
	limiterMu      sync.Mutex
	loginAttempts  map[string]attempt
}

func NewManager(ctx context.Context, database *Database, secureCookies bool, trustedProxies []netip.Prefix) (*Manager, error) {
	if database == nil || database.SQL == nil {
		return nil, errors.New("authentication database is required")
	}
	params := &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	manager := &Manager{
		database:       database,
		hashSlots:      make(chan struct{}, 1),
		trustedProxies: append([]netip.Prefix(nil), trustedProxies...),
		loginAttempts:  make(map[string]attempt),
	}
	if err := manager.acquireHashSlot(ctx); err != nil {
		return nil, err
	}
	dummyHash, err := argon2id.CreateHash("unavailable-account-dummy-password", params)
	manager.releaseHashSlot()
	if err != nil {
		return nil, fmt.Errorf("initialize password verifier: %w", err)
	}
	manager.dummyHash = dummyHash

	manager.sessions = scs.New()
	manager.sessions.Store = database.Sessions
	manager.sessions.IdleTimeout = 30 * time.Minute
	manager.sessions.Lifetime = 12 * time.Hour
	manager.sessions.Cookie.Name = "compose_manager_session"
	manager.sessions.Cookie.Domain = ""
	manager.sessions.Cookie.Path = "/"
	manager.sessions.Cookie.HttpOnly = true
	manager.sessions.Cookie.Secure = secureCookies
	manager.sessions.Cookie.SameSite = http.SameSiteLaxMode
	manager.sessions.Cookie.Persist = false
	manager.sessions.HashTokenInStore = true
	return manager, nil
}

func (manager *Manager) SessionManager() *scs.SessionManager {
	return manager.sessions
}

func (manager *Manager) Bootstrap(ctx context.Context, username, password string) error {
	needsBootstrap, err := manager.database.NeedsBootstrap(ctx)
	if err != nil {
		return err
	}
	if !needsBootstrap {
		return nil
	}
	if err := ValidateUsername(username); err != nil {
		return ErrBootstrapRequired
	}
	if err := ValidatePassword(password); err != nil {
		return ErrBootstrapRequired
	}
	hash, err := manager.HashPassword(ctx, password)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	_, err = manager.database.ProvisionInitialAdmin(ctx, username, hash)
	return err
}

func (manager *Manager) Authenticate(ctx context.Context, username, password, clientKey string) (User, error) {
	if ValidateUsername(username) != nil || len(password) > maxPasswordBytes {
		return User{}, ErrInvalidCredentials
	}
	attemptKey := clientKey + "\x00" + strings.ToLower(username)
	if !manager.allowAttempt(clientKey, attemptKey) {
		return User{}, ErrRateLimited
	}
	user, err := manager.database.UserByUsername(ctx, username)
	if err != nil && !errors.Is(err, ErrNoUser) {
		return User{}, err
	}
	hash := manager.dummyHash
	if err == nil {
		hash = user.PasswordHash
	}
	if err := manager.acquireHashSlot(ctx); err != nil {
		return User{}, err
	}
	matched, _, verifyErr := argon2id.CheckHash(password, hash)
	manager.releaseHashSlot()
	if verifyErr != nil {
		if err == nil {
			return User{}, fmt.Errorf("verify stored password hash: %w", verifyErr)
		}
		return User{}, ErrInvalidCredentials
	}
	if err != nil || !matched {
		return User{}, ErrInvalidCredentials
	}
	manager.clearAttempts(attemptKey)
	return user, nil
}

func (manager *Manager) HashPassword(ctx context.Context, password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if err := manager.acquireHashSlot(ctx); err != nil {
		return "", err
	}
	defer manager.releaseHashSlot()
	params := &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	hash, err := argon2id.CreateHash(password, params)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

func (manager *Manager) ChangePassword(ctx context.Context, user User, password string) (User, error) {
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	matched, err := manager.passwordMatches(ctx, password, user.PasswordHash)
	if err != nil {
		return User{}, err
	}
	if matched {
		return User{}, ErrPasswordUnchanged
	}
	hash, err := manager.HashPassword(ctx, password)
	if err != nil {
		return User{}, err
	}
	version, err := manager.database.ChangePassword(ctx, user.ID, user.AuthVersion, hash)
	if err != nil {
		return User{}, err
	}
	user.PasswordHash = hash
	user.MustChange = false
	user.AuthVersion = version
	return user, nil
}

func (manager *Manager) StartSession(ctx context.Context, user User) error {
	if err := manager.sessions.RenewToken(ctx); err != nil {
		return err
	}
	manager.sessions.Put(ctx, sessionUserIDKey, user.ID)
	manager.sessions.Put(ctx, sessionAuthVersionKey, user.AuthVersion)
	manager.sessions.Put(ctx, sessionMustChangeKey, user.MustChange)
	return nil
}

func (manager *Manager) CurrentUser(ctx context.Context) (User, bool, error) {
	userID := manager.sessions.GetString(ctx, sessionUserIDKey)
	if userID == "" {
		return User{}, false, nil
	}
	user, err := manager.database.UserByID(ctx, userID)
	if errors.Is(err, ErrNoUser) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	if manager.sessions.GetInt64(ctx, sessionAuthVersionKey) != user.AuthVersion {
		return User{}, false, nil
	}
	return user, true, nil
}

func ValidateUsername(username string) error {
	if username == "" || len(username) > 128 || strings.TrimSpace(username) != username || !utf8.ValidString(username) {
		return errors.New("invalid username")
	}
	for _, char := range username {
		if unicode.IsControl(char) {
			return errors.New("invalid username")
		}
	}
	return nil
}

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || len(password) > maxPasswordBytes || utf8.RuneCountInString(password) < minPasswordCodepoints {
		return ErrInvalidPassword
	}
	return nil
}

func ClientKey(remoteAddr, forwardedFor string, trustedProxies []netip.Prefix) string {
	peer, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "unknown"
	}
	peerAddress, err := netip.ParseAddr(peer)
	if err != nil {
		return "unknown"
	}
	if !isTrusted(peerAddress, trustedProxies) || forwardedFor == "" {
		return peerAddress.Unmap().String()
	}
	addresses := strings.Split(forwardedFor, ",")
	client := peerAddress
	for index := len(addresses) - 1; index >= 0; index-- {
		address, err := netip.ParseAddr(strings.TrimSpace(addresses[index]))
		if err != nil {
			break
		}
		client = address
		if !isTrusted(address, trustedProxies) {
			break
		}
	}
	return client.Unmap().String()
}

func (manager *Manager) passwordMatches(ctx context.Context, password, hash string) (bool, error) {
	if err := manager.acquireHashSlot(ctx); err != nil {
		return false, err
	}
	matched, _, err := argon2id.CheckHash(password, hash)
	manager.releaseHashSlot()
	if err != nil {
		return false, fmt.Errorf("verify password: %w", err)
	}
	return matched, nil
}

func (manager *Manager) acquireHashSlot(ctx context.Context) error {
	select {
	case manager.hashSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrRateLimited
	}
}

func (manager *Manager) releaseHashSlot() {
	<-manager.hashSlots
}

func (manager *Manager) allowAttempt(clientKey, key string) bool {
	now := time.Now()
	manager.limiterMu.Lock()
	defer manager.limiterMu.Unlock()
	clientKeys := 0
	for candidate, value := range manager.loginAttempts {
		if now.Sub(value.started) >= loginWindow {
			delete(manager.loginAttempts, candidate)
		} else if strings.HasPrefix(candidate, clientKey+"\x00") {
			clientKeys++
		}
	}
	value, exists := manager.loginAttempts[key]
	if !exists {
		if len(manager.loginAttempts) >= maxLoginKeys || clientKeys >= maxLoginKeysPerClient {
			return false
		}
		manager.loginAttempts[key] = attempt{started: now, count: 1}
		return true
	}
	if now.Sub(value.started) >= loginWindow {
		manager.loginAttempts[key] = attempt{started: now, count: 1}
		return true
	}
	if value.count >= maxLoginAttempts {
		return false
	}
	value.count++
	manager.loginAttempts[key] = value
	return true
}

func (manager *Manager) clearAttempts(key string) {
	manager.limiterMu.Lock()
	delete(manager.loginAttempts, key)
	manager.limiterMu.Unlock()
}

func isTrusted(address netip.Addr, ranges []netip.Prefix) bool {
	address = address.Unmap()
	for _, prefix := range ranges {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
