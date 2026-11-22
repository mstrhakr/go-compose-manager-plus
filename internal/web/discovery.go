package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
)

func (server *authServer) discoveryMessage(w http.ResponseWriter, r *http.Request, key, message string) {
	server.manager.SessionManager().Put(r.Context(), key, message)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (server *authServer) saveRoot(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid discovery form", http.StatusBadRequest)
		return
	}
	if err := server.discovery.SetRoot(r.Context(), r.FormValue("root")); err != nil {
		message := "Unable to save discovery root due to a storage error."
		if errors.Is(err, discovery.ErrInvalidRoot) {
			message = "Enter an absolute application-visible discovery root."
		}
		server.discoveryMessage(w, r, "discovery_error", message)
		return
	}
	server.discoveryMessage(w, r, "discovery_notice", "Discovery root saved. Request a scan to find projects.")
}

func (server *authServer) runScan(r *http.Request) (discovery.Settings, discovery.ScanResult, error) {
	settings, err := server.discovery.Settings(r.Context())
	if err != nil {
		return discovery.Settings{}, discovery.ScanResult{}, err
	}
	if settings.Root == "" {
		return discovery.Settings{}, discovery.ScanResult{}, fmt.Errorf("save a discovery root before scanning")
	}
	result, err := discovery.Scan(r.Context(), settings.Root)
	if err != nil {
		return discovery.Settings{}, discovery.ScanResult{}, fmt.Errorf("unable to scan the discovery root; check its path and permissions")
	}
	if err := server.discovery.ApplyScan(r.Context(), settings, result.Projects); err != nil {
		return discovery.Settings{}, discovery.ScanResult{}, err
	}
	settings.Revision++
	return settings, result, nil
}

func (server *authServer) scanProjects(w http.ResponseWriter, r *http.Request) {
	_, result, err := server.runScan(r)
	if err != nil {
		server.discoveryMessage(w, r, "discovery_error", err.Error())
		return
	}
	message := fmt.Sprintf("Scan complete: %d project(s) found.", len(result.Projects))
	if len(result.SkippedSymlink) > 0 {
		message += " Skipped symlinks: " + strings.Join(result.SkippedSymlink, ", ") + "."
	}
	server.discoveryMessage(w, r, "discovery_notice", message)
}

func (server *authServer) activateProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid activation form", http.StatusBadRequest)
		return
	}
	settings, err := server.discovery.Settings(r.Context())
	path := r.FormValue("path")
	if err == nil {
		err = discovery.ValidateProject(r.Context(), settings.Root, path)
	}
	if err == nil {
		err = server.discovery.Activate(r.Context(), settings, path)
	}
	if err != nil {
		server.discoveryMessage(w, r, "discovery_error", err.Error())
		return
	}
	server.discoveryMessage(w, r, "discovery_notice", "Project activated. No Compose operation was performed.")
}
