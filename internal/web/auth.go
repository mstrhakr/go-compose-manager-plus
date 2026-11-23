package web

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

const maxFormBytes = 8192

type authServer struct {
	manager         *auth.Manager
	discovery       *discovery.Store
	metricsSettings *metrics.SettingsStore
	collector       dashboardMetricsCollector
	pages           *template.Template
	publicURL       *url.URL
	trustedProxies  []netip.Prefix
	logger          *slog.Logger
	docker          *runtime.Client
}

func (server *authServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", server.loginPage)
	mux.HandleFunc("POST /login", server.login)
	mux.Handle("GET /setup/password", server.requireUser(true, http.HandlerFunc(server.setupPage)))
	mux.Handle("POST /setup/password", server.requireUser(true, http.HandlerFunc(server.changePassword)))
	mux.Handle("POST /logout", server.requireUser(true, http.HandlerFunc(server.logout)))
	mux.Handle("GET /{$}", server.requireUser(false, http.HandlerFunc(server.dashboard)))
	mux.Handle("GET /projects", server.requireUser(false, http.HandlerFunc(server.projectsPage)))
	mux.Handle("GET /containers", server.requireUser(false, http.HandlerFunc(server.containersPage)))
	mux.Handle("GET /settings", server.requireUser(false, http.HandlerFunc(server.metricsSettingsPage)))
	mux.Handle("POST /settings/metrics", server.requireUser(false, http.HandlerFunc(server.saveMetricsSettings)))
	mux.Handle("POST /discovery/root", server.requireUser(false, http.HandlerFunc(server.saveRoot)))
	mux.Handle("POST /discovery/scan", server.requireUser(false, http.HandlerFunc(server.scanProjects)))
	mux.Handle("POST /discovery/activate", server.requireUser(false, http.HandlerFunc(server.activateProject)))
}

func (server *authServer) loginPage(w http.ResponseWriter, r *http.Request) {
	user, authenticated, err := server.manager.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "Unable to verify session", http.StatusInternalServerError)
		return
	}
	if authenticated {
		if user.MustChange {
			http.Redirect(w, r, "/setup/password", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		return
	}
	server.render(w, pageData{Title: "Log in", View: "login", CSRFToken: nosurf.Token(r)})
}

func (server *authServer) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		server.render(w, pageData{Title: "Log in", View: "login", Error: "Enter a valid username and password.", CSRFToken: nosurf.Token(r)})
		return
	}
	clientKey := auth.ClientKey(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), server.trustedProxies)
	user, err := server.manager.Authenticate(r.Context(), r.FormValue("username"), r.FormValue("password"), clientKey)
	if err != nil {
		message := "Unable to log in. Check your username and password, then try again."
		if errors.Is(err, auth.ErrRateLimited) {
			message = "Too many attempts. Wait briefly, then try again."
		}
		server.render(w, pageData{Title: "Log in", View: "login", Error: message, CSRFToken: nosurf.Token(r)})
		return
	}
	if err := server.manager.StartSession(r.Context(), user); err != nil {
		http.Error(w, "Unable to start session", http.StatusInternalServerError)
		return
	}
	if user.MustChange {
		http.Redirect(w, r, "/setup/password", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (server *authServer) setupPage(w http.ResponseWriter, r *http.Request) {
	user, _, err := server.manager.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "Unable to verify session", http.StatusInternalServerError)
		return
	}
	if !user.MustChange {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	server.render(w, pageData{Title: "Secure your account", View: "setup", Username: user.Username, CSRFToken: nosurf.Token(r)})
}

func (server *authServer) changePassword(w http.ResponseWriter, r *http.Request) {
	user, _, err := server.manager.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "Unable to verify session", http.StatusInternalServerError)
		return
	}
	if !user.MustChange {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		server.render(w, pageData{Title: "Secure your account", View: "setup", Username: user.Username, Error: "Enter a valid new password.", CSRFToken: nosurf.Token(r)})
		return
	}
	password := r.FormValue("password")
	if password != r.FormValue("confirm_password") {
		server.render(w, pageData{Title: "Secure your account", View: "setup", Username: user.Username, Error: "The passwords do not match.", CSRFToken: nosurf.Token(r)})
		return
	}
	updated, err := server.manager.ChangePassword(r.Context(), user, password)
	if err != nil {
		message := "Unable to save that password. Choose a different password and try again."
		if errors.Is(err, auth.ErrInvalidPassword) {
			message = "Use at least 15 characters for your new password."
		} else if errors.Is(err, auth.ErrPasswordUnchanged) {
			message = "Choose a password different from the temporary password."
		}
		server.render(w, pageData{Title: "Secure your account", View: "setup", Username: user.Username, Error: message, CSRFToken: nosurf.Token(r)})
		return
	}
	if err := server.manager.StartSession(r.Context(), updated); err != nil {
		http.Error(w, "Password changed; please log in again", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (server *authServer) logout(w http.ResponseWriter, r *http.Request) {
	if err := server.manager.SessionManager().Destroy(r.Context()); err != nil {
		http.Error(w, "Unable to log out", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (server *authServer) dashboard(w http.ResponseWriter, r *http.Request) {
	server.workspace(w, r, "dashboard")
}

func (server *authServer) projectsPage(w http.ResponseWriter, r *http.Request) {
	server.workspace(w, r, "projects")
}

func (server *authServer) containersPage(w http.ResponseWriter, r *http.Request) {
	server.workspace(w, r, "containers")
}

func (server *authServer) workspace(w http.ResponseWriter, r *http.Request, view string) {
	user, _, err := server.manager.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "Unable to verify session", http.StatusInternalServerError)
		return
	}
	runes := []rune(user.Username)
	initial := "?"
	if len(runes) > 0 {
		initial = strings.ToUpper(string(runes[0]))
	}
	settings, err := server.discovery.Settings(r.Context())
	if err != nil {
		http.Error(w, "Unable to read discovery settings", http.StatusInternalServerError)
		return
	}
	projects, err := server.discovery.Projects(r.Context())
	if err != nil {
		http.Error(w, "Unable to read discovered projects", http.StatusInternalServerError)
		return
	}
	views := make([]projectView, 0, len(projects))
	for _, project := range projects {
		views = append(views, projectView{Project: project, DisplayName: filepath.Base(project.Path)})
	}
	requested := r.URL.Query().Get("runtime") == "1" || view == "projects" || view == "containers"
	containerList := listPanel{ID: "containers-title", Title: "Containers", Customize: true, Columns: []string{"Name", "State", "Compose project", "Service"}, Empty: "No containers found in this snapshot."}
	var containers []runtime.Container
	runtimeMessage := "Docker integration pending. Container status, logs, and operations are not connected."
	runtimeState := "Not configured"
	if server.docker != nil {
		runtimeMessage = "Docker status not requested."
		runtimeState = "Not requested"
	}
	if requested && server.docker == nil {
		runtimeMessage = "Docker status unavailable: no socket configured."
		runtimeState = "Unavailable"
	} else if requested {
		snapshot, snapshotErr := server.docker.Snapshot(r.Context())
		if snapshotErr != nil {
			server.logger.WarnContext(r.Context(), "Docker status snapshot failed", "reason", scanFailureReason(snapshotErr))
			runtimeMessage = "Docker status unavailable. Check socket permissions and daemon compatibility."
			runtimeState = "Unavailable"
		} else {
			runtimeMessage = "Docker snapshot taken " + time.Now().UTC().Format("2006-01-02 15:04:05 UTC") + "."
			runtimeState = "Snapshot"
			if view != "containers" {
				for i := range views {
					project := &views[i]
					name, resolveErr := discovery.ResolveProjectName(r.Context(), project.Root, project.Path)
					if resolveErr != nil {
						project.RuntimeError = "Runtime status unknown: unable to resolve Compose inputs."
						continue
					}
					project.RuntimeName = name
					project.Containers = snapshot.ByProject[name]
				}
			}
			containers = snapshot.Containers
		}
	}
	var filters inventoryFilters
	if view == "projects" || view == "containers" {
		filters, views, containers = filterInventory(view, r.URL.Query(), views, containers)
		if runtimeState == "Snapshot" && filters.Total > 0 && filters.Shown == 0 {
			containerList.Empty = "No containers match these filters."
		}
	}
	if view == "containers" {
		for _, container := range containers {
			name := container.Name
			if name == "" {
				name = container.ID
			}
			if name == "" {
				name = "Unknown"
			}
			project := container.Project
			if project == "" {
				project = "No Compose project label"
			}
			service := container.Service
			if service == "" {
				service = "—"
			}
			indicator := "unknown"
			switch container.State {
			case "running":
				indicator = "running"
			case "created", "exited", "dead":
				indicator = "stopped"
			}
			containerList.Rows = append(containerList.Rows, listRow{Cells: []string{name, container.State, project, service}, Indicator: indicator})
		}
	}
	if runtimeState != "Snapshot" {
		containerList.Empty = runtimeMessage
	}
	containerList.Status = runtimeState
	title := "Overview"
	subviews := viewNavigation{Label: "Overview views", Tabs: []viewTab{{Label: "Dashboard", Href: "/", Current: true}}}
	if view == "projects" {
		title = "Compose projects"
		subviews = viewNavigation{Label: "Project views", Tabs: []viewTab{
			{Label: "Projects", Href: "/projects", Current: true},
			{Label: "Activity"},
		}}
	} else if view == "containers" {
		title = "Containers"
		subviews = viewNavigation{Label: "Container views", Tabs: []viewTab{
			{Label: "Containers", Href: "/containers", Current: true},
			{Label: "Observability"},
		}}
	}
	var charts []chartPanel
	var metricsStatus string
	if view == "dashboard" {
		charts, metricsStatus = server.dashboardCharts(r.Context())
	}
	server.render(w, pageData{Title: title, View: view, Username: user.Username, Initial: initial, Containers: containerList, ProjectColumns: listPanel{ID: "projects-title", Title: "Projects", Columns: []string{"Project", "Discovery", "Activation", "Containers"}}, Filters: filters, SubviewNav: subviews,
		CSRFToken: nosurf.Token(r), Root: settings.Root, Projects: views, RuntimeMessage: runtimeMessage, RuntimeState: runtimeState,
		Charts: charts, MetricsStatus: metricsStatus,
		Metrics: []metricPanel{
			{ID: "restart-title", Title: "Container restart & OOM rate", Status: "Unavailable", Value: "--", Detail: "No runtime data"},
			{ID: "health-title", Title: "Stack health & readiness", Status: "Unavailable", Value: "--", Detail: "No runtime data"},
		},
		Error:  server.manager.SessionManager().PopString(r.Context(), "discovery_error"),
		Notice: server.manager.SessionManager().PopString(r.Context(), "discovery_notice")})
}

func (server *authServer) requireUser(allowPasswordChange bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, authenticated, err := server.manager.CurrentUser(r.Context())
		if err != nil {
			http.Error(w, "Unable to verify session", http.StatusInternalServerError)
			return
		}
		if !authenticated {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if user.MustChange && !allowPasswordChange {
			http.Redirect(w, r, "/setup/password", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (server *authServer) render(w http.ResponseWriter, data pageData) {
	var result bytes.Buffer
	if err := server.pages.ExecuteTemplate(&result, "page", data); err != nil {
		http.Error(w, "Unable to render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = result.WriteTo(w)
}

func canonicalHost(publicURL *url.URL, trustedProxies []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if strings.EqualFold(r.Host, publicURL.Host) {
			next.ServeHTTP(w, r)
			return
		}
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		peerIP, parseErr := netip.ParseAddr(peer)
		forwardedHost := r.Header.Get("X-Forwarded-Host")
		if err == nil && parseErr == nil && isTrustedProxy(peerIP, trustedProxies) && !strings.Contains(forwardedHost, ",") && strings.EqualFold(forwardedHost, publicURL.Host) {
			r.Host = publicURL.Host
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Misdirected request", http.StatusMisdirectedRequest)
	})
}

func isTrustedProxy(address netip.Addr, ranges []netip.Prefix) bool {
	address = address.Unmap()
	for _, prefix := range ranges {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
