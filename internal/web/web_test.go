package web

import (
	"context"
	"html"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
)

const testOrigin = "http://127.0.0.1:8080"

func TestAuthenticationSetupAndDashboardFlow(t *testing.T) {
	handler := testHandler(t, nil)
	cookies := make(map[string]*http.Cookie)
	response := perform(handler, http.MethodGet, "/", nil, cookies, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous dashboard = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/healthz", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("health endpoint = %d %q", response.Code, response.Body.String())
	}
	response = perform(handler, http.MethodGet, "/login", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Welcome back") {
		t.Fatalf("login page = %d %q", response.Code, response.Body.String())
	}
	csrfToken := tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"bootstrap-password-123"},
	}, cookies, "bad-token")
	if response.Code != http.StatusForbidden {
		t.Fatalf("login without CSRF = %d", response.Code)
	}
	response = perform(handler, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"bootstrap-password-123"},
	}, cookies, csrfToken)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
		t.Fatalf("bootstrap login = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	collectCookies(cookies, response)
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
		t.Fatalf("forced-change dashboard gate = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Secure your account") {
		t.Fatalf("password setup = %d %q", response.Code, response.Body.String())
	}
	csrfToken = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/setup/password", url.Values{
		"password": {"new-local-password-456"}, "confirm_password": {"new-local-password-456"},
	}, cookies, csrfToken)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("password change = %d, location %q, body %q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	collectCookies(cookies, response)
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Compose projects") || !strings.Contains(response.Body.String(), "Docker integration pending") {
		t.Fatalf("dashboard = %d %q", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "Hello,") {
		t.Fatal("template greeting is still exposed as the application home")
	}
	csrfToken = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/logout", url.Values{}, cookies, csrfToken)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("logout = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	collectCookies(cookies, response)
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("dashboard after logout = %d, location %q", response.Code, response.Header().Get("Location"))
	}
}

func TestOversizedAuthenticationFormsAreRejected(t *testing.T) {
	handler := testHandler(t, nil)
	cookies := make(map[string]*http.Cookie)
	response := perform(handler, http.MethodGet, "/login", nil, cookies, "")
	token := tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"bootstrap-password-123"}, "padding": {strings.Repeat("x", maxFormBytes)},
	}, cookies, token)
	if response.Code != http.StatusForbidden && response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized login = %d", response.Code)
	}
	response = perform(handler, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"bootstrap-password-123"},
	}, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
		t.Fatalf("valid login = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/setup/password", url.Values{
		"password": {"new-local-password-456"}, "confirm_password": {"new-local-password-456"}, "padding": {strings.Repeat("x", maxFormBytes)},
	}, cookies, token)
	if response.Code != http.StatusForbidden && response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized password change = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	if response.Code != http.StatusOK {
		t.Fatalf("password setup after oversized request = %d", response.Code)
	}
}

func TestScriptContentTypeIgnoresHostRegistry(t *testing.T) {
	original := mime.TypeByExtension(".js")
	t.Cleanup(func() { _ = mime.AddExtensionType(".js", original) })
	if err := mime.AddExtensionType(".js", "application/javascript"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler := testHandler(t, nil)
	handler.ServeHTTP(response, httptest.NewRequest("GET", testOrigin+"/static/app.js", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("host MIME leaked into response: %d %v", response.Code, response.Header())
	}
}

func TestCanonicalHostRequiresTrustedProxy(t *testing.T) {
	handler := testHandler(t, nil)
	request := httptest.NewRequest(http.MethodGet, "http://internal:8080/login", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("X-Forwarded-Host", "manager.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest {
		t.Fatalf("untrusted forwarded host = %d", response.Code)
	}
}

func TestLoopbackOriginLoginFromContainerBridge(t *testing.T) {
	handler := testHandler(t, nil)
	page := httptest.NewRequest(http.MethodGet, "/login", nil)
	page.Host = "127.0.0.1:8080"
	page.RemoteAddr = "172.17.0.1:12345"
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	token := tokenFromBody(t, pageResponse.Body.String())
	values := url.Values{"username": {"operator"}, "password": {"bootstrap-password-123"}, "csrf_token": {token}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(values.Encode()))
	request.Host = "127.0.0.1:8080"
	request.RemoteAddr = "172.17.0.1:12345"
	request.Header.Set("Origin", testOrigin)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range pageResponse.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
		t.Fatalf("bridge login = %d, location %q", response.Code, response.Header().Get("Location"))
	}
}

func TestTrustedProxyLoginRequiresHTTPSOrigin(t *testing.T) {
	ctx := context.Background()
	database, err := auth.OpenDatabase(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	proxies := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	manager, err := auth.NewManager(ctx, database, true, proxies)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Bootstrap(ctx, "operator", "bootstrap-password-123"); err != nil {
		t.Fatal(err)
	}
	publicURL, err := url.Parse("https://manager.example.test")
	if err != nil {
		t.Fatal(err)
	}
	handler := Handler(manager, publicURL, proxies)
	page := httptest.NewRequest(http.MethodGet, "/login", nil)
	page.Host = "backend:8080"
	page.RemoteAddr = "192.0.2.10:12345"
	page.Header.Set("X-Forwarded-Host", publicURL.Host)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("login page through trusted proxy = %d", pageResponse.Code)
	}
	token := tokenFromBody(t, pageResponse.Body.String())
	for _, test := range []struct {
		origin string
		want   int
	}{
		{"http://manager.example.test", http.StatusForbidden},
		{"https://attacker.example.test", http.StatusForbidden},
		{"https://manager.example.test", http.StatusSeeOther},
	} {
		t.Run(test.origin, func(t *testing.T) {
			values := url.Values{"username": {"operator"}, "password": {"bootstrap-password-123"}, "csrf_token": {token}}
			request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(values.Encode()))
			request.Host = "backend:8080"
			request.RemoteAddr = "192.0.2.10:12345"
			request.Header.Set("X-Forwarded-Host", publicURL.Host)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for _, cookie := range pageResponse.Result().Cookies() {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("origin %q: got %d, want %d", test.origin, response.Code, test.want)
			}
		})
	}
}

func testHandler(t *testing.T, trustedProxies []netip.Prefix) http.Handler {
	t.Helper()
	ctx := context.Background()
	database, err := auth.OpenDatabase(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.SQL.Close() })
	manager, err := auth.NewManager(ctx, database, false, trustedProxies)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Bootstrap(ctx, "operator", "bootstrap-password-123"); err != nil {
		t.Fatal(err)
	}
	publicURL, err := url.Parse(testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	return Handler(manager, publicURL, trustedProxies)
}

func perform(handler http.Handler, method, path string, values url.Values, cookies map[string]*http.Cookie, csrfToken string) *httptest.ResponseRecorder {
	body := strings.NewReader("")
	if values != nil {
		values.Set("csrf_token", csrfToken)
		body = strings.NewReader(values.Encode())
	}
	request := httptest.NewRequest(method, testOrigin+path, body)
	request.RemoteAddr = "127.0.0.1:23456"
	if values != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", testOrigin)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	collectCookies(cookies, response)
	return response
}

func collectCookies(cookies map[string]*http.Cookie, response *httptest.ResponseRecorder) {
	for _, cookie := range response.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
}

func tokenFromBody(t *testing.T, body string) string {
	t.Helper()
	pattern := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	match := pattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("CSRF token field missing from response: %q", body)
	}
	return html.UnescapeString(match[1])
}
