package web

import (
	"bytes"
	"errors"
	"html/template"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/auth"
)

const maxFormBytes = 8192

type authServer struct {
	manager        *auth.Manager
	pages          *template.Template
	publicURL      *url.URL
	trustedProxies []netip.Prefix
}

func (server *authServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", server.loginPage)
	mux.HandleFunc("POST /login", server.login)
	mux.Handle("GET /setup/password", server.requireUser(true, http.HandlerFunc(server.setupPage)))
	mux.Handle("POST /setup/password", server.requireUser(true, http.HandlerFunc(server.changePassword)))
	mux.Handle("POST /logout", server.requireUser(true, http.HandlerFunc(server.logout)))
	mux.Handle("GET /{$}", server.requireUser(false, http.HandlerFunc(server.dashboard)))
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
	server.render(w, pageData{Title: "Overview", View: "dashboard", Username: user.Username, Initial: initial, CSRFToken: nosurf.Token(r)})
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
