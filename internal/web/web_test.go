package web

import (
	"bytes"
	"context"
	"html"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

const testOrigin = "http://127.0.0.1:8080"

func TestAuthenticationSetupAndDashboardFlow(t *testing.T) {
	handler := testHandler(t, nil)
	cookies := make(map[string]*http.Cookie)
	var response *httptest.ResponseRecorder
	for _, path := range []string{"/", "/projects", "/containers"} {
		response = perform(handler, http.MethodGet, path, nil, cookies, "")
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
			t.Fatalf("anonymous %s = %d, location %q", path, response.Code, response.Header().Get("Location"))
		}
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
	for _, path := range []string{"/", "/projects", "/containers"} {
		response = perform(handler, http.MethodGet, path, nil, cookies, "")
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup/password" {
			t.Fatalf("forced-change gate %s = %d, location %q", path, response.Code, response.Header().Get("Location"))
		}
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
	for _, path := range []string{"/projects", "/containers"} {
		response = perform(handler, http.MethodGet, path, nil, cookies, "")
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Docker status unavailable: no socket configured.") {
			t.Fatalf("unconfigured %s = %d %q", path, response.Code, response.Body.String())
		}
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

func TestDiscoveryAuthenticatedFlow(t *testing.T) {
	var logs bytes.Buffer
	handler := testHandlerWithLogger(t, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
	root := t.TempDir()
	project := filepath.Join(root, "example")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	compose := filepath.Join(project, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cookies := make(map[string]*http.Cookie)
	response := perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {root}}, cookies, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("anonymous root POST without token = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/login", nil, cookies, "")
	token := tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/login", url.Values{"username": {"operator"}, "password": {"bootstrap-password-123"}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/setup/password", url.Values{"password": {"new-local-password-456"}, "confirm_password": {"new-local-password-456"}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("setup = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {"relative"}, "return_to": {"/projects"}}, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/projects" {
		t.Fatalf("inventory form return = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Enter an absolute") {
		t.Fatalf("inventory form error missing: %s", response.Body.String())
	}
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {"relative"}, "return_to": {"/containers"}}, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/containers" {
		t.Fatalf("containers form return = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/containers", nil, cookies, "")
	if !strings.Contains(response.Body.String(), `role="alert">Enter an absolute`) {
		t.Fatalf("containers form error missing: %s", response.Body.String())
	}
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {root}, "return_to": {"/containers"}}, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/containers" {
		t.Fatalf("containers notice return = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/containers", nil, cookies, "")
	if !strings.Contains(response.Body.String(), `role="status">Discovery root saved.`) {
		t.Fatalf("containers notice missing: %s", response.Body.String())
	}
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {"relative"}, "return_to": {"https://example.com"}}, cookies, token)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("untrusted form return = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {root}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save root = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("scan = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Inactive") || !strings.Contains(response.Body.String(), html.EscapeString(project)) {
		t.Fatalf("discovered project missing: %s", response.Body.String())
	}
	later := filepath.Join(root, "later")
	if err := os.Mkdir(later, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(later, "compose.yaml"), []byte("services:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token = tokenFromBody(t, response.Body.String())
	response = perform(handler, http.MethodPost, "/discovery/activate", url.Values{"path": {project}}, cookies, token)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("activation = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Active") || !strings.Contains(response.Body.String(), "No Compose operation was performed") {
		t.Fatalf("activation not reflected: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), html.EscapeString(later)) {
		t.Fatal("activation unexpectedly discovered another project")
	}
	if err := os.Rename(root, root+"-temporarily-unavailable"); err != nil {
		t.Fatal(err)
	}
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "unable to scan") || !strings.Contains(response.Body.String(), "Available") || !strings.Contains(response.Body.String(), "Active") {
		t.Fatalf("failed scan did not preserve state: %s", response.Body.String())
	}
	if !strings.Contains(logs.String(), `"msg":"discovery scan failed"`) ||
		!strings.Contains(logs.String(), `"stage":"scan_filesystem"`) ||
		!strings.Contains(logs.String(), `"reason":"not_found"`) ||
		strings.Contains(logs.String(), root) {
		t.Fatalf("unexpected scan diagnostic: %s", logs.String())
	}
	if err := os.Rename(root+"-temporarily-unavailable", root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(compose); err != nil {
		t.Fatal(err)
	}
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Unavailable") || !strings.Contains(response.Body.String(), "Active") {
		t.Fatalf("missing project not retained: %s", response.Body.String())
	}
	if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Available") || !strings.Contains(response.Body.String(), "Active") {
		t.Fatalf("restored project not active: %s", response.Body.String())
	}
	laterToken := tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {t.TempDir()}}, cookies, laterToken)
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "Outside the current discovery root") || !strings.Contains(response.Body.String(), `disabled>Activate</button>`) {
		t.Fatalf("previous-root activation should be disabled: %s", response.Body.String())
	}
}

func TestRuntimeSnapshotIsAuthenticatedOnDemandAndNameScoped(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Unix socket integration is exercised on Unix")
	}
	socketDir, err := os.MkdirTemp("", "cmp-sock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(socketDir); err != nil {
			t.Errorf("remove socket directory: %v", err)
		}
	})
	socket := filepath.Join(socketDir, "docker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var failure atomic.Bool
	var empty atomic.Bool
	dockerServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v1.40/containers/json" || r.URL.Query().Get("all") != "1" {
			t.Errorf("unexpected Docker request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		if failure.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if empty.Load() {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, `[{"Names":["/<script>"],"State":"running","Labels":{"com.docker.compose.project":"shared","com.docker.compose.service":"app"}},{"Id":"abcdef","Names":["/unmanaged"],"State":"exited","Labels":{}}]`)
	}))
	_ = dockerServer.Listener.Close()
	dockerServer.Listener = listener
	dockerServer.Start()
	t.Cleanup(dockerServer.Close)
	handler := testHandlerWithDocker(t, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), runtime.NewClient(socket))
	root := t.TempDir()
	for folder, projectName := range map[string]string{"one": "shared", "two": "shared", "empty": "empty", "broken": "broken"} {
		dir := filepath.Join(root, folder)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("name: "+projectName+"\nservices:\n  app:\n    image: example\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cookies := make(map[string]*http.Cookie)
	for _, path := range []string{"/?runtime=1", "/projects", "/containers"} {
		perform(handler, http.MethodGet, path, nil, cookies, "")
	}
	if calls.Load() != 0 {
		t.Fatal("anonymous request reached Docker")
	}
	response := perform(handler, http.MethodGet, "/login", nil, cookies, "")
	token := tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/login", url.Values{"username": {"operator"}, "password": {"bootstrap-password-123"}}, cookies, token)
	response = perform(handler, http.MethodGet, "/setup/password", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/setup/password", url.Values{"password": {"new-local-password-456"}, "confirm_password": {"new-local-password-456"}}, cookies, token)
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/root", url.Values{"root": {root}}, cookies, token)
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	restored := filepath.Join(root, "two", "compose.yaml")
	if err := os.Remove(restored); err != nil {
		t.Fatal(err)
	}
	response = perform(handler, http.MethodGet, "/", nil, cookies, "")
	token = tokenFromBody(t, response.Body.String())
	perform(handler, http.MethodPost, "/discovery/scan", url.Values{}, cookies, token)
	if err := os.WriteFile(restored, []byte("name: shared\nservices:\n  app:\n    image: example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "compose.yaml"), []byte("services: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	perform(handler, http.MethodGet, "/", nil, cookies, "")
	if calls.Load() != 0 {
		t.Fatal("normal dashboard or discovery reached Docker")
	}
	response = perform(handler, http.MethodGet, "/projects?runtime=1", nil, cookies, "")
	body := response.Body.String()
	if response.Code != http.StatusOK || strings.Count(body, "Docker project: <strong>shared</strong>") != 2 ||
		strings.Count(body, "&lt;script&gt;") != 2 ||
		!strings.Contains(body, ">Unavailable</span>") || !strings.Contains(body, ">Inactive</span>") ||
		strings.Count(body, "No matching containers in this snapshot.") != 1 ||
		strings.Count(body, "Runtime status unknown: unable to resolve Compose inputs.") != 1 || calls.Load() != 1 {
		t.Fatalf("snapshot: code=%d calls=%d body=%s", response.Code, calls.Load(), body)
	}
	response = perform(handler, http.MethodGet, "/projects", nil, cookies, "")
	body = response.Body.String()
	if response.Code != http.StatusOK || calls.Load() != 2 ||
		strings.Count(body, "Docker project: <strong>shared</strong>") != 2 ||
		strings.Count(body, "&lt;script&gt;") != 2 ||
		strings.Count(body, "No matching containers in this snapshot.") != 1 ||
		!strings.Contains(body, "Runtime status unknown: unable to resolve Compose inputs.") ||
		!strings.Contains(body, "<details") || strings.Contains(body, "unmanaged") {
		t.Fatalf("project inventory: code=%d calls=%d body=%s", response.Code, calls.Load(), body)
	}
	response = perform(handler, http.MethodGet, "/containers", nil, cookies, "")
	body = response.Body.String()
	if response.Code != http.StatusOK || calls.Load() != 3 ||
		!strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "unmanaged") ||
		!strings.Contains(body, "exited") || !strings.Contains(body, "No Compose project label") ||
		!strings.Contains(body, "Docker snapshot taken") || strings.Contains(body, "<script>") {
		t.Fatalf("container inventory: code=%d calls=%d body=%s", response.Code, calls.Load(), body)
	}
	response = perform(handler, http.MethodGet, "/containers?state=exited&group=No+Compose+project+label&q=UNMANAGED&sort=name-desc", nil, cookies, "")
	body = response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "1 of 2 containers") ||
		!strings.Contains(body, "unmanaged") || strings.Contains(body, "&lt;script&gt;") ||
		!strings.Contains(body, `value="exited" checked`) {
		t.Fatalf("filtered containers: %d %s", response.Code, body)
	}
	response = perform(handler, http.MethodGet, "/containers?project=shared&state=running&state=exited", nil, cookies, "")
	body = response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "1 of 2 containers") ||
		!strings.Contains(body, "&lt;script&gt;") || strings.Contains(body, ">unmanaged<") {
		t.Fatalf("project-filtered containers: %d %s", response.Code, body)
	}
	response = perform(handler, http.MethodGet, "/projects?q=one&group=Inactive", nil, cookies, "")
	body = response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "1 of 4 projects") ||
		!strings.Contains(body, filepath.Join(root, "one")) || strings.Contains(body, filepath.Join(root, "two")) {
		t.Fatalf("filtered projects: %d %s", response.Code, body)
	}
	response = perform(handler, http.MethodGet, "/containers?q=does-not-exist", nil, cookies, "")
	if !strings.Contains(response.Body.String(), "No containers match these filters.") {
		t.Fatalf("filtered empty inventory: %s", response.Body.String())
	}
	empty.Store(true)
	response = perform(handler, http.MethodGet, "/containers", nil, cookies, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No containers found in this snapshot.") {
		t.Fatalf("empty inventory = %d %s", response.Code, response.Body.String())
	}
	empty.Store(false)
	failure.Store(true)
	for _, path := range []string{"/?runtime=1", "/projects", "/containers"} {
		response = perform(handler, http.MethodGet, path, nil, cookies, "")
		body = response.Body.String()
		if !strings.Contains(body, "Docker status unavailable") || strings.Contains(body, "No matching containers") || strings.Contains(body, "No containers found") {
			t.Fatalf("failed snapshot displayed as empty at %s: %s", path, body)
		}
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
	settings := metrics.NewSettingsStore(database.SQL)
	collector, err := metrics.NewCollector(t.TempDir(), settings, discovery.NewStore(database.SQL), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	handler := Handler(manager, discovery.NewStore(database.SQL), settings, collector, publicURL, proxies, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
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
	return testHandlerWithLogger(t, trustedProxies, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func testHandlerWithLogger(t *testing.T, trustedProxies []netip.Prefix, logger *slog.Logger) http.Handler {
	return testHandlerWithDocker(t, trustedProxies, logger, nil)
}

func testHandlerWithDocker(t *testing.T, trustedProxies []netip.Prefix, logger *slog.Logger, docker *runtime.Client) http.Handler {
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
	settings := metrics.NewSettingsStore(database.SQL)
	collector, err := metrics.NewCollector(t.TempDir(), settings, discovery.NewStore(database.SQL), docker, logger)
	if err != nil {
		t.Fatal(err)
	}
	return Handler(manager, discovery.NewStore(database.SQL), settings, collector, publicURL, trustedProxies, logger, docker)
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
