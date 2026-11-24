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
	Enabled                 bool
	Profile                 string
	Scope                   string
	Interval                string
	Limit                   string
	LimitUnit               string
	Revision                string
	StateEnabled            bool
	StateScope              string
	StateSavedScope         string
	StateRevision           string
	LogsEnabled             bool
	LogsConfigured          bool
	LogsProfile             string
	LogsScope               string
	LogsDeliveryPreset      string
	LogsLimit               string
	LogsLimitUnit           string
	LogsRevision            string
	OverallProfile          string
	StatsSavedProfile       string
	LogsSavedProfile        string
	CollectionRevision      string
	RestoreStats            bool
	RestoreState            bool
	RestoreLogs             bool
	StatsScopeDescription   string
	LogsDeliveryDescription string
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

func formFromMetricsSettings(settings metrics.Settings, sources metrics.SourceSettings) metricsSettingsForm {
	if settings.Revision == 0 {
		settings = metrics.Settings{
			Profile: metrics.ProfileLow, Scope: metrics.ScopeActivated, Interval: 15 * time.Second,
			StorageLimit: metrics.DefaultMetricsStorageLimit,
		}
	}
	limit, unit := formatStorageLimit(settings.StorageLimit)
	logLimit, logUnit := formatStorageLimit(sources.Logs.StorageLimit)
	form := metricsSettingsForm{
		Enabled: settings.Enabled, Profile: string(settings.Profile), Scope: string(settings.Scope),
		Interval: settings.Interval.String(), Limit: limit, LimitUnit: unit,
		Revision:           strconv.FormatInt(settings.Revision, 10),
		StatsSavedProfile:  string(settings.Profile),
		LogsSavedProfile:   string(sources.Logs.Profile),
		CollectionRevision: strconv.FormatInt(sources.CollectionRevision, 10),
		RestoreStats:       sources.RestoreStats, RestoreState: sources.RestoreState, RestoreLogs: sources.RestoreLogs,
		StateEnabled: sources.StateEnabled, StateScope: string(sources.StateScope),
		StateSavedScope: string(sources.StateScope),
		StateRevision:   strconv.FormatInt(sources.StateRevision, 10),
		LogsEnabled:     sources.Logs.Enabled, LogsConfigured: sources.Logs.Configured,
		LogsProfile: string(sources.Logs.Profile), LogsScope: string(sources.Logs.Scope),
		LogsDeliveryPreset: string(sources.Logs.DeliveryPreset),
		LogsLimit:          logLimit, LogsLimitUnit: logUnit,
		LogsRevision: strconv.FormatInt(sources.Logs.Revision, 10),
	}
	if !form.StateEnabled {
		form.StateScope = "off"
	}
	if !form.Enabled {
		form.Profile = "off"
	}
	if !form.LogsEnabled {
		form.LogsProfile = "off"
	}
	form.OverallProfile = overallCollectionProfile(settings, sources)
	if form.OverallProfile != "off" {
		form.RestoreStats, form.RestoreState, form.RestoreLogs = form.Enabled, form.StateEnabled, form.LogsEnabled
	}
	form.StatsScopeDescription = "Activated stacks · " + settings.Interval.String()
	if settings.Scope == metrics.ScopeAll {
		form.StatsScopeDescription = "All running containers · " + settings.Interval.String()
	}
	switch sources.Logs.DeliveryPreset {
	case metrics.ProfileLow:
		form.LogsDeliveryDescription = "Activated stacks · periodic backlog every 60 seconds"
	case metrics.ProfileMedium:
		form.LogsDeliveryDescription = "Activated stacks · periodic backlog every 30 seconds"
	case metrics.ProfileHigh:
		form.LogsDeliveryDescription = "All containers · live follow"
	}
	if sources.Logs.Configured {
		scope := "Activated stacks"
		if sources.Logs.Scope == metrics.ScopeAll {
			scope = "All containers"
		}
		switch sources.Logs.DeliveryPreset {
		case metrics.ProfileLow:
			form.LogsDeliveryDescription = scope + " · periodic backlog every 60 seconds"
		case metrics.ProfileMedium:
			form.LogsDeliveryDescription = scope + " · periodic backlog every 30 seconds"
		case metrics.ProfileHigh:
			form.LogsDeliveryDescription = scope + " · live follow"
		}
	}
	return form
}

func overallCollectionProfile(stats metrics.Settings, sources metrics.SourceSettings) string {
	if sources.OverallProfile != "" {
		return string(sources.OverallProfile)
	}
	return string(metrics.InferCollectionProfile(stats, sources))
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

func logCollectionHistoryWarning(evictions metrics.EvictedLogs) string {
	if evictions.Count == 0 {
		return ""
	}
	return "Storage pressure has shortened retained logs: " + strconv.FormatInt(evictions.Count, 10) +
		" records removed between " + evictions.Oldest.Format(time.RFC3339) +
		" and " + evictions.Newest.Format(time.RFC3339) + " (these bounds may not be a continuous gap)."
}

func (server *authServer) metricsSettingsPage(w http.ResponseWriter, r *http.Request) {
	settings, sources, err := server.metricsSettings.LoadConfiguration(r.Context())
	if err != nil {
		server.logger.ErrorContext(r.Context(), "read metrics settings failed", "error", err)
		http.Error(w, "Unable to read metrics settings", http.StatusInternalServerError)
		return
	}
	server.renderMetricsSettings(w, r, formFromMetricsSettings(settings, sources), "", server.manager.SessionManager().PopString(r.Context(), "metrics_notice"))
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
	if status.StateEnabled {
		switch {
		case status.StateProblem != "":
			statusText += " State/event capture is not healthy: " + status.StateProblem
		case status.StateCollecting:
			statusText += " State/event capture is active."
		default:
			statusText += " State/event capture is starting."
		}
	}
	if status.LogsEnabled {
		switch {
		case status.LogsProblem != "":
			statusText += " Log capture is not healthy: " + status.LogsProblem
		case status.LogsCollecting:
			statusText += " Log capture is active."
		default:
			statusText += " Log capture is starting."
		}
	}
	warning := collectionHistoryWarning(status.Evictions)
	if logWarning := logCollectionHistoryWarning(status.LogEvictions); logWarning != "" {
		warning = strings.TrimSpace(warning + " " + logWarning)
	}
	server.render(w, pageData{Title: "Settings", View: "settings", Username: user.Username, Initial: initial,
		CSRFToken: nosurf.Token(r), MetricsSettings: form, CollectionStatusText: statusText,
		MetricsPresets:    metricsProfilePresets[:],
		CollectionWarning: warning, Error: problem, Notice: notice})
}

func (server *authServer) saveMetricsSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid settings form", http.StatusBadRequest)
		return
	}
	currentSettings, currentSources, err := server.metricsSettings.LoadConfiguration(r.Context())
	if err != nil {
		server.logger.ErrorContext(r.Context(), "read metrics settings before save", "error", err)
		http.Error(w, "Unable to read metrics settings", http.StatusInternalServerError)
		return
	}
	stateScopeValue := r.FormValue("state_scope")
	if stateScopeValue == "" {
		stateScopeValue = string(currentSources.StateScope)
		if !currentSources.StateEnabled {
			stateScopeValue = "off"
		}
	}
	logsProfileValue := r.FormValue("logs_profile")
	if logsProfileValue == "" {
		logsProfileValue = string(currentSources.Logs.Profile)
		if !currentSources.Logs.Enabled {
			logsProfileValue = "off"
		}
	}
	logsLimitValue, logsUnitValue := r.FormValue("logs_storage_limit"), r.FormValue("logs_storage_unit")
	if logsLimitValue == "" {
		logsLimitValue, logsUnitValue = formatStorageLimit(currentSources.Logs.StorageLimit)
	}
	form := metricsSettingsForm{
		Enabled: r.FormValue("enabled") == "true", Profile: r.FormValue("profile"), Scope: r.FormValue("scope"), Interval: r.FormValue("interval"),
		Limit: r.FormValue("storage_limit"), LimitUnit: r.FormValue("storage_unit"), Revision: r.FormValue("revision"),
		StateEnabled: stateScopeValue != "off", StateScope: stateScopeValue,
		StateSavedScope: r.FormValue("state_saved_scope"), StateRevision: r.FormValue("state_revision"),
		LogsEnabled: logsProfileValue != "off",
		LogsProfile: logsProfileValue, LogsScope: r.FormValue("logs_scope"),
		LogsDeliveryPreset: r.FormValue("logs_delivery_preset"),
		LogsLimit:          logsLimitValue, LogsLimitUnit: logsUnitValue,
		LogsRevision:      r.FormValue("logs_revision"),
		StatsSavedProfile: r.FormValue("stats_saved_profile"), LogsSavedProfile: r.FormValue("logs_saved_profile"),
		CollectionRevision: r.FormValue("collection_revision"), OverallProfile: r.FormValue("overall_profile"),
		RestoreStats: currentSources.RestoreStats, RestoreState: currentSources.RestoreState, RestoreLogs: currentSources.RestoreLogs,
	}
	newForm := r.Form.Has("stats_saved_profile")
	if form.Scope == "" {
		form.Scope = r.FormValue("stats_saved_scope")
		if form.Scope == "" && r.FormValue("client_managed") != "true" {
			form.Scope = string(currentSettings.Scope)
			if currentSettings.Revision == 0 {
				form.Scope = string(metrics.ScopeActivated)
			}
		}
	}
	if newForm {
		form.Enabled = form.Profile != "off"
	}
	if form.StatsSavedProfile == "" {
		form.StatsSavedProfile = string(currentSettings.Profile)
		if currentSettings.Revision == 0 {
			form.StatsSavedProfile = "low"
		}
	}
	if form.LogsSavedProfile == "" {
		form.LogsSavedProfile = string(currentSources.Logs.Profile)
	}
	if form.CollectionRevision == "" {
		form.CollectionRevision = strconv.FormatInt(currentSources.CollectionRevision, 10)
	}
	if r.FormValue("client_managed") == "true" {
		form.RestoreStats = r.FormValue("restore_stats") == "true"
		form.RestoreState = r.FormValue("restore_state") == "true"
		form.RestoreLogs = r.FormValue("restore_logs") == "true"
	} else if form.OverallProfile == "off" && currentSources.OverallProfile != metrics.ProfileOff {
		form.RestoreStats, form.RestoreState, form.RestoreLogs = form.Enabled, form.StateEnabled, form.LogsEnabled
	}
	switch r.FormValue("overall_profile") {
	case "off":
		form.Enabled = false
		if form.Profile != "off" {
			form.StatsSavedProfile = form.Profile
		}
		if form.LogsProfile != "off" {
			form.LogsSavedProfile = form.LogsProfile
		}
		form.Profile = "off"
		form.StateScope = "off"
		form.LogsProfile = "off"
	case "custom":
		restore := currentSources.OverallProfile == metrics.ProfileOff &&
			(r.FormValue("client_managed") != "true" || r.FormValue("overall_restore") == "true")
		if restore {
			form.Enabled = form.RestoreStats
			form.Profile = form.StatsSavedProfile
			if !form.Enabled {
				form.Profile = "off"
			}
			if form.RestoreState {
				form.StateScope = form.StateSavedScope
				if form.StateScope == "" {
					form.StateScope = string(currentSources.StateScope)
				}
			}
			if form.RestoreLogs {
				form.LogsProfile = form.LogsSavedProfile
			}
		}
	case string(metrics.ProfileLow), string(metrics.ProfileMedium), string(metrics.ProfileHigh):
		preset := metrics.Profile(r.FormValue("overall_profile"))
		form.Enabled, form.Profile = true, string(preset)
		switch preset {
		case metrics.ProfileLow:
			form.Scope, form.Interval = string(metrics.ScopeActivated), (15 * time.Second).String()
			form.StateScope, form.StateSavedScope = string(metrics.ScopeActivated), string(metrics.ScopeActivated)
			form.LogsProfile, form.LogsScope, form.LogsDeliveryPreset = string(preset), string(metrics.ScopeActivated), string(preset)
		case metrics.ProfileMedium:
			form.Scope, form.Interval = string(metrics.ScopeActivated), (5 * time.Second).String()
			form.StateScope, form.StateSavedScope = string(metrics.ScopeActivated), string(metrics.ScopeActivated)
			form.LogsProfile, form.LogsScope, form.LogsDeliveryPreset = string(preset), string(metrics.ScopeActivated), string(preset)
		case metrics.ProfileHigh:
			form.Scope, form.Interval = string(metrics.ScopeAll), time.Second.String()
			form.StateScope, form.StateSavedScope = string(metrics.ScopeAll), string(metrics.ScopeAll)
			form.LogsProfile, form.LogsScope, form.LogsDeliveryPreset = string(preset), string(metrics.ScopeAll), string(preset)
		}
	}
	if form.StateSavedScope == "" {
		form.StateSavedScope = string(currentSources.StateScope)
	}
	if form.StateRevision == "" {
		form.StateRevision = strconv.FormatInt(currentSources.StateRevision, 10)
	}
	if form.LogsRevision == "" {
		form.LogsRevision = strconv.FormatInt(currentSources.Logs.Revision, 10)
	}
	if form.LogsScope == "" {
		form.LogsScope = string(currentSources.Logs.Scope)
	}
	if form.LogsDeliveryPreset == "" {
		form.LogsDeliveryPreset = string(currentSources.Logs.DeliveryPreset)
	}
	fail := func(message string) { server.renderMetricsSettings(w, r, form, message, "") }
	switch form.OverallProfile {
	case "", "off", "low", "medium", "high", "custom":
	default:
		fail("Choose a valid overall collection profile.")
		return
	}
	switch form.StateScope {
	case "off", "activated", "all":
	default:
		fail("Choose a valid state/event collection scope.")
		return
	}
	switch form.LogsProfile {
	case "off", "low", "medium", "high", "custom":
	default:
		fail("Choose a valid log collection profile.")
		return
	}
	revision, err := strconv.ParseInt(form.Revision, 10, 64)
	if err != nil || revision < 0 || revision == math.MaxInt64 {
		fail("Invalid settings revision. Reload this page and try again.")
		return
	}
	stateRevision, stateRevisionErr := strconv.ParseInt(form.StateRevision, 10, 64)
	logRevision, logRevisionErr := strconv.ParseInt(form.LogsRevision, 10, 64)
	collectionRevision, collectionRevisionErr := strconv.ParseInt(form.CollectionRevision, 10, 64)
	if stateRevisionErr != nil || stateRevision < 0 || logRevisionErr != nil || logRevision < 0 ||
		collectionRevisionErr != nil || collectionRevision < 1 || collectionRevision == math.MaxInt64 {
		fail("Invalid source settings revision. Reload this page and try again.")
		return
	}
	limit, err := parseStorageLimit(form.Limit, form.LimitUnit)
	if err != nil {
		fail("Enter a positive storage limit using B, KB, MB or GB. KB, MB and GB use decimal units.")
		return
	}
	logLimit, err := parseStorageLimit(form.LogsLimit, form.LogsLimitUnit)
	if err != nil {
		fail("Enter a positive log storage target using B, KB, MB or GB.")
		return
	}
	if currentSettings.Revision != revision || currentSources.StateRevision != stateRevision || currentSources.Logs.Revision != logRevision ||
		currentSources.CollectionRevision != collectionRevision {
		fail("Settings changed in another session. Reload this page before saving.")
		return
	}
	profile := metrics.Profile(form.Profile)
	if profile == metrics.ProfileOff {
		profile = metrics.Profile(form.StatsSavedProfile)
	}
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
	currentMetricsLimit := currentSettings.StorageLimit
	if currentSettings.Revision == 0 {
		currentMetricsLimit = metrics.DefaultMetricsStorageLimit
	}
	if limit != currentMetricsLimit {
		settings.Profile = metrics.ProfileCustom
	}
	stateEnabled := form.StateScope != "off"
	if (form.Enabled || form.LogsEnabled || stateEnabled) && server.docker == nil {
		fail("Configure a Docker socket before enabling collection.")
		return
	}
	settings.Enabled = form.Enabled
	stateScope := metrics.CollectionScope(form.StateScope)
	stateEnabled = stateScope != "off"
	if !stateEnabled {
		stateScope = metrics.CollectionScope(form.StateSavedScope)
	}
	sources := metrics.SourceSettings{
		StateEnabled: stateEnabled, StateScope: stateScope, StateRevision: stateRevision,
		Logs:           currentSources.Logs,
		OverallProfile: metrics.Profile(form.OverallProfile), CollectionRevision: collectionRevision,
		RestoreStats: form.RestoreStats, RestoreState: form.RestoreState, RestoreLogs: form.RestoreLogs,
	}
	if form.LogsProfile == "off" {
		sources.Logs.Enabled = false
		sources.Logs.StorageLimit = logLimit
		sources.Logs.Revision = logRevision
		if form.LogsSavedProfile != "" && form.LogsDeliveryPreset != "" {
			sources.Logs.Configured = true
			sources.Logs.Profile = metrics.Profile(form.LogsSavedProfile)
			sources.Logs.Scope = metrics.CollectionScope(form.LogsScope)
			sources.Logs.DeliveryPreset = metrics.Profile(form.LogsDeliveryPreset)
		}
	} else {
		logProfile := metrics.Profile(form.LogsProfile)
		deliveryPreset := metrics.Profile(form.LogsDeliveryPreset)
		if logProfile == metrics.ProfileLow || logProfile == metrics.ProfileMedium || logProfile == metrics.ProfileHigh {
			deliveryPreset = logProfile
		}
		scope := metrics.CollectionScope(form.LogsScope)
		switch logProfile {
		case metrics.ProfileLow, metrics.ProfileMedium:
			scope = metrics.ScopeActivated
		case metrics.ProfileHigh:
			scope = metrics.ScopeAll
		}
		if scope != metrics.ScopeActivated && scope != metrics.ScopeAll {
			fail("Choose a valid log collection scope.")
			return
		}
		if !currentSources.Logs.Configured && logProfile == metrics.ProfileCustom && deliveryPreset == "" {
			fail("Choose an explicit Low, Medium or High log preset before Custom.")
			return
		}
		sources.Logs = metrics.LogSettings{
			Configured: true, Enabled: true, Profile: logProfile, Scope: scope,
			DeliveryPreset: deliveryPreset, StorageLimit: logLimit, Revision: logRevision,
		}
	}
	if _, _, err := server.metricsSettings.SaveConfiguration(r.Context(), settings, sources); err != nil {
		if errors.Is(err, metrics.ErrStaleSettings) {
			fail("Settings changed in another session. Reload this page before saving.")
			return
		}
		server.logger.ErrorContext(r.Context(), "save metrics settings failed", "error", err)
		http.Error(w, "Unable to save metrics settings", http.StatusInternalServerError)
		return
	}
	server.manager.SessionManager().Put(r.Context(), "metrics_notice", "Observability preferences saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
