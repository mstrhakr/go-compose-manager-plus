package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

//go:embed templates static
var content embed.FS

type pageData struct {
	Title                string
	View                 string
	Username             string
	Initial              string
	Error                string
	CSRFToken            string
	Root                 string
	Projects             []projectView
	RuntimeMessage       string
	RuntimeState         string
	Notice               string
	Charts               []chartPanel
	Timeline             dashboardTimeline
	LogHistory           dashboardLogHistory
	ObservedContainers   []observedContainer
	SelectedContainer    string
	MetricsStatus        string
	Metrics              []metricPanel
	Containers           listPanel
	ProjectColumns       listPanel
	Filters              inventoryFilters
	SubviewNav           viewNavigation
	MetricsSettings      metricsSettingsForm
	MetricsPresets       []metricsProfilePreset
	CollectionStatusText string
	CollectionWarning    string
}

type projectView struct {
	discovery.Project
	DisplayName  string
	RuntimeName  string
	RuntimeError string
	Containers   []runtime.Container
}

func (project projectView) DiscoveryStatus() string {
	if !project.Available {
		return "Unavailable"
	}
	if project.ResolutionError != "" {
		return "Inputs need attention"
	}
	return "Available"
}

func (project projectView) ActivationStatus() string {
	if project.Active {
		return "Active"
	}
	return "Inactive"
}

func Handler(manager *auth.Manager, store *discovery.Store, metricsSettings *metrics.SettingsStore, collector *metrics.Collector, publicURL *url.URL, trustedProxies []netip.Prefix, logger *slog.Logger, docker *runtime.Client) http.Handler {
	pages := template.Must(template.ParseFS(content, "templates/*.html", "templates/partials/*.html"))
	assets, err := fs.Sub(content, "static")
	if err != nil {
		panic(err)
	}
	if manager == nil || store == nil || metricsSettings == nil || collector == nil || publicURL == nil || logger == nil {
		panic("authenticated web handler requires auth manager and public origin")
	}
	mux := http.NewServeMux()
	server := &authServer{manager: manager, discovery: store, metricsSettings: metricsSettings, collector: collector, pages: pages, publicURL: publicURL, trustedProxies: trustedProxies, logger: logger, docker: docker}
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
	loggedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() {
			if r.Pattern == "GET /healthz" || r.Pattern == "GET /static/" {
				return
			}
			logger.DebugContext(r.Context(), "HTTP request completed",
				"method", r.Method, "route", r.Pattern, "duration_ms", time.Since(started).Milliseconds())
		}()
		mux.ServeHTTP(w, r)
	})
	protected := http.NewCrossOriginProtection().Handler(loggedMux)
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
