package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
)

//go:embed templates static
var content embed.FS

type pageData struct {
	Title     string
	View      string
	Username  string
	Initial   string
	Error     string
	CSRFToken string
	Root      string
	Projects  []discovery.Project
	Notice    string
}

func Handler(manager *auth.Manager, store *discovery.Store, publicURL *url.URL, trustedProxies []netip.Prefix) http.Handler {
	pages := template.Must(template.ParseFS(content, "templates/*.html", "templates/partials/*.html"))
	assets, err := fs.Sub(content, "static")
	if err != nil {
		panic(err)
	}
	if manager == nil || store == nil || publicURL == nil {
		panic("authenticated web handler requires auth manager and public origin")
	}
	mux := http.NewServeMux()
	server := &authServer{manager: manager, discovery: store, pages: pages, publicURL: publicURL, trustedProxies: trustedProxies}
	server.registerRoutes(mux)
	static := http.StripPrefix("/static/", http.FileServerFS(assets))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		static.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	protected := http.NewCrossOriginProtection().Handler(mux)
	withSessions := manager.SessionManager().LoadAndSave(protected)
	csrfProtected := nosurf.New(withSessions)
	cookieName := "compose_manager_csrf"
	if publicURL.Scheme == "https" {
		cookieName = "__Host-compose_manager_csrf"
	}
	csrfProtected.SetBaseCookie(http.Cookie{Name: cookieName, Path: "/", HttpOnly: true, Secure: publicURL.Scheme == "https", SameSite: http.SameSiteLaxMode})
	csrfProtected.SetIsTLSFunc(func(*http.Request) bool { return publicURL.Scheme == "https" })
	csrfProtected.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Request could not be verified", http.StatusForbidden)
	}))
	withHealthException := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			withSessions.ServeHTTP(w, r)
			return
		}
		csrfProtected.ServeHTTP(w, r)
	})
	withCanonicalHost := canonicalHost(publicURL, trustedProxies, withHealthException)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		}
		withCanonicalHost.ServeHTTP(w, r)
	})
}
