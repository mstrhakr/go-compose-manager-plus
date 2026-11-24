package web

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/justinas/nosurf"
	"github.com/mstrhakr/go-compose-manager-plus/internal/metrics"
)

type metricsSettingsForm struct {
	Enabled   bool
	Profile   string
	Scope     string
	Interval  string
	Limit     string
	LimitUnit string
	Revision  string
}

type metricsProfilePreset struct {
	Profile    metrics.Profile
	Label      string
	Scope      metrics.CollectionScope
	ScopeLabel string
	Interval   time.Duration
}

var metricsProfilePresets = [...]metricsProfilePreset{
	{metrics.ProfileLow, "Low", metrics.ScopeActivated, "Activated stacks", 15 * time.Second},
	{metrics.ProfileMedium, "Medium", metrics.ScopeActivated, "Activated stacks", 5 * time.Second},
	{metrics.ProfileHigh, "High", metrics.ScopeAll, "All running containers", time.Second},
}

func formFromMetricsSettings(settings metrics.Settings) metricsSettingsForm {
	if settings.Revision == 0 {
		return metricsSettingsForm{Profile: string(metrics.ProfileLow), Scope: string(metrics.ScopeActivated), Interval: "15s", LimitUnit: "MB", Revision: "0"}
	}
	limit, unit := formatStorageLimit(settings.StorageLimit)
	return metricsSettingsForm{
		Enabled: settings.Enabled, Profile: string(settings.Profile), Scope: string(settings.Scope),
		Interval: settings.Interval.String(), Limit: limit, LimitUnit: unit,
		Revision: strconv.FormatInt(settings.Revision, 10),
	}
}

var storageUnits = map[string]int64{
	"B":  1,
	"KB": 1_000,
	"MB": 1_000_000,
	"GB": 1_000_000_000,
}

var storageAmountPattern = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func parseStorageLimit(value, unit string) (int64, error) {
	if unit == "" {
		unit = "B"
	}
	multiplier, ok := storageUnits[unit]
	if !ok || len(value) == 0 || len(value) > 64 {
		return 0, errors.New("invalid storage limit or unit")
	}
	value = strings.TrimSpace(value)
	if !storageAmountPattern.MatchString(value) {
		return 0, errors.New("invalid storage amount")
	}
	amount, ok := new(big.Rat).SetString(value)
	if !ok || amount.Sign() <= 0 {
		return 0, errors.New("storage limit must be positive")
	}
	bytes := new(big.Rat).Mul(amount, new(big.Rat).SetInt64(multiplier))
	if bytes.Denom().Cmp(big.NewInt(1)) != 0 || !bytes.Num().IsInt64() {
		return 0, errors.New("storage limit must resolve to a positive whole number of bytes")
	}
	return bytes.Num().Int64(), nil
}

func formatStorageLimit(bytes int64) (string, string) {
	for _, unit := range []struct {
		name       string
		multiplier int64
		precision  int
	}{
		{"GB", storageUnits["GB"], 9},
		{"MB", storageUnits["MB"], 6},
		{"KB", storageUnits["KB"], 3},
	} {
		if bytes < unit.multiplier {
			continue
		}
		value := strconv.FormatInt(bytes/unit.multiplier, 10)
		remainder := bytes % unit.multiplier
		if remainder == 0 {
			return value, unit.name
		}
		fraction := strings.TrimRight(fmt.Sprintf("%0*d", unit.precision, remainder), "0")
		return value + "." + fraction, unit.name
	}
	return strconv.FormatInt(bytes, 10), "B"
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
	for _, preset := range metricsProfilePresets {
		if string(preset.Profile) == form.Profile {
			form.Scope, form.Interval = string(preset.Scope), preset.Interval.String()
			break
		}
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
		MetricsPresets:    metricsProfilePresets[:],
		CollectionWarning: collectionHistoryWarning(status.Evictions), Error: problem, Notice: notice})
}

func (server *authServer) saveMetricsSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid settings form", http.StatusBadRequest)
		return
	}
	form := metricsSettingsForm{
		Enabled: r.FormValue("enabled") == "true", Profile: r.FormValue("profile"), Scope: r.FormValue("scope"), Interval: r.FormValue("interval"),
		Limit: r.FormValue("storage_limit"), LimitUnit: r.FormValue("storage_unit"), Revision: r.FormValue("revision"),
	}
	fail := func(message string) { server.renderMetricsSettings(w, r, form, message, "") }
	revision, err := strconv.ParseInt(form.Revision, 10, 64)
	if err != nil || revision < 0 || revision == math.MaxInt64 {
		fail("Invalid settings revision. Reload this page and try again.")
		return
	}
	limit, err := parseStorageLimit(form.Limit, form.LimitUnit)
	if err != nil {
		fail("Enter a positive storage limit using B, KB, MB or GB. KB, MB and GB use decimal units.")
		return
	}
	current, err := server.metricsSettings.Load(r.Context())
	if err != nil {
		server.logger.ErrorContext(r.Context(), "read metrics settings before save", "error", err)
		http.Error(w, "Unable to read metrics settings", http.StatusInternalServerError)
		return
	}
	if current.Revision != revision {
		fail("Settings changed in another session. Reload this page before saving.")
		return
	}
	profile := metrics.Profile(form.Profile)
	settings := metrics.Settings{Profile: profile, StorageLimit: limit, Revision: revision}
	if profile == metrics.ProfileCustom {
		settings.Scope = metrics.CollectionScope(form.Scope)
		settings.Interval, err = time.ParseDuration(form.Interval)
		if err != nil || settings.Interval <= 0 || (settings.Scope != metrics.ScopeActivated && settings.Scope != metrics.ScopeAll) {
			fail("Choose a valid custom scope and a positive duration such as 5s.")
			return
		}
	} else {
		found := false
		for _, preset := range metricsProfilePresets {
			if preset.Profile == profile {
				settings.Scope, settings.Interval = preset.Scope, preset.Interval
				found = true
				break
			}
		}
		if !found {
			fail("Choose a valid metrics profile.")
			return
		}
	}
	if limit != current.StorageLimit {
		settings.Profile = metrics.ProfileCustom
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
