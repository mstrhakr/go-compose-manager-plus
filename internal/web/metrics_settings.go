package web

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

type metricsSettingsForm struct {
	Enabled  bool
	Profile  string
	Scope    string
	Interval string
	Limit    string
	Revision string
}

func formFromMetricsSettings(settings metrics.Settings) metricsSettingsForm {
	if settings.Revision == 0 {
		return metricsSettingsForm{Profile: string(metrics.ProfileLow), Scope: string(metrics.ScopeActivated), Interval: "15s", Revision: "0"}
	}
	return metricsSettingsForm{
		Enabled: settings.Enabled, Profile: string(settings.Profile), Scope: string(settings.Scope),
		Interval: settings.Interval.String(), Limit: strconv.FormatInt(settings.StorageLimit, 10),
		Revision: strconv.FormatInt(settings.Revision, 10),
	}
}

func collectionHistoryWarning(evictions metrics.EvictedSamples) string {
	if evictions.Count == 0 {
		return ""
	}
	return "Cumulative storage-pressure evictions have shortened history: " + strconv.FormatInt(evictions.Count, 10) +
		" samples removed between " + evictions.Oldest.Format(time.RFC3339) +
		" and " + evictions.Newest.Format(time.RFC3339) + " (these bounds may not be a continuous gap)."
}

func (server *authServer) metricsSettingsPage(w http.ResponseWriter, r *http.Request) {
	settings, err := server.metricsSettings.Load(r.Context())
	if err != nil {
		server.logger.ErrorContext(r.Context(), "read metrics settings failed", "error", err)
		http.Error(w, "Unable to read metrics settings", http.StatusInternalServerError)
		return
	}
	server.renderMetricsSettings(w, r, formFromMetricsSettings(settings), "", server.manager.SessionManager().PopString(r.Context(), "metrics_notice"))
}

func (server *authServer) renderMetricsSettings(w http.ResponseWriter, r *http.Request, form metricsSettingsForm, problem, notice string) {
	user, _, err := server.manager.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, "Unable to verify session", http.StatusInternalServerError)
		return
	}
	initial := "?"
	if runes := []rune(user.Username); len(runes) != 0 {
		initial = strings.ToUpper(string(runes[0]))
	}
	status := server.collector.Status()
	statusText := "Disabled; no background metrics collection is running."
	if status.Enabled {
		switch {
		case status.Problem != "":
			statusText = "Enabled but not healthy: " + status.Problem
		case status.Collecting && status.EventConnected:
			statusText = "Collecting; Docker lifecycle event stream is connected."
		case status.Collecting:
			statusText = "Collecting; lifecycle event stream is reconnecting, so rates may be unavailable."
		default:
			statusText = "Enabled; collector is starting."
		}
	}
	if !status.LastCycle.IsZero() {
		statusText += " Last collection cycle: " + status.LastCycle.Format(time.RFC3339)
	}
	server.render(w, pageData{Title: "Settings", View: "settings", Username: user.Username, Initial: initial,
		CSRFToken: nosurf.Token(r), MetricsSettings: form, CollectionStatusText: statusText,
		CollectionWarning: collectionHistoryWarning(status.Evictions), Error: problem, Notice: notice})
}

func (server *authServer) saveMetricsSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid settings form", http.StatusBadRequest)
		return
	}
	form := metricsSettingsForm{
		Enabled: r.FormValue("enabled") == "true", Profile: r.FormValue("profile"), Scope: r.FormValue("scope"), Interval: r.FormValue("interval"),
		Limit: r.FormValue("storage_limit"), Revision: r.FormValue("revision"),
	}
	fail := func(message string) { server.renderMetricsSettings(w, r, form, message, "") }
	revision, err := strconv.ParseInt(form.Revision, 10, 64)
	if err != nil || revision < 0 || revision == math.MaxInt64 {
		fail("Invalid settings revision. Reload this page and try again.")
		return
	}
	limit, err := strconv.ParseInt(form.Limit, 10, 64)
	if err != nil || limit <= 0 {
		fail("Enter a positive storage limit in bytes.")
		return
	}
	settings := metrics.Settings{Profile: metrics.Profile(form.Profile), StorageLimit: limit, Revision: revision}
	switch settings.Profile {
	case metrics.ProfileLow:
		settings.Scope, settings.Interval = metrics.ScopeActivated, 15*time.Second
	case metrics.ProfileMedium:
		settings.Scope, settings.Interval = metrics.ScopeActivated, 5*time.Second
	case metrics.ProfileHigh:
		settings.Scope, settings.Interval = metrics.ScopeAll, time.Second
	case metrics.ProfileCustom:
		settings.Scope = metrics.CollectionScope(form.Scope)
		settings.Interval, err = time.ParseDuration(form.Interval)
		if err != nil || settings.Interval <= 0 || (settings.Scope != metrics.ScopeActivated && settings.Scope != metrics.ScopeAll) {
			fail("Choose a valid custom scope and a positive duration such as 5s.")
			return
		}
	default:
		fail("Choose a valid metrics profile.")
		return
	}
	if form.Enabled && server.docker == nil {
		fail("Configure a Docker socket before enabling metrics collection.")
		return
	}
	settings.Enabled = form.Enabled
	if _, err := server.metricsSettings.Save(r.Context(), settings); err != nil {
		if errors.Is(err, metrics.ErrStaleSettings) {
			fail("Settings changed in another session. Reload this page before saving.")
			return
		}
		server.logger.ErrorContext(r.Context(), "save metrics settings failed", "error", err)
		http.Error(w, "Unable to save metrics settings", http.StatusInternalServerError)
		return
	}
	server.manager.SessionManager().Put(r.Context(), "metrics_notice", "Metrics preferences saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
