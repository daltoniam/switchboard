package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/gcal"
	"github.com/daltoniam/switchboard/integrations/gchat"
	"github.com/daltoniam/switchboard/integrations/gdocs"
	"github.com/daltoniam/switchboard/integrations/gdrive"
	"github.com/daltoniam/switchboard/integrations/gforms"
	ghInt "github.com/daltoniam/switchboard/integrations/github"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/gmeet"
	"github.com/daltoniam/switchboard/integrations/gpeople"
	"github.com/daltoniam/switchboard/integrations/gsheets"
	"github.com/daltoniam/switchboard/integrations/gslides"
	"github.com/daltoniam/switchboard/integrations/gtasks"
	linearInt "github.com/daltoniam/switchboard/integrations/linear"
	sentryInt "github.com/daltoniam/switchboard/integrations/sentry"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	xInt "github.com/daltoniam/switchboard/integrations/x"
	"github.com/daltoniam/switchboard/marketplace"
	"github.com/daltoniam/switchboard/remotemcp"
	wasmmod "github.com/daltoniam/switchboard/wasm"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// WebServer serves the configuration web UI using templ templates.
type WebServer struct {
	services    *mcp.Services
	port        int
	health      *healthCache
	marketplace *marketplace.Manager
	wasmLoader  pluginLoader
}

// New returns a WebServer that provides a browser-based config UI.
func New(services *mcp.Services, port int, mp *marketplace.Manager, wl *wasmmod.Loader) *WebServer {
	ws := &WebServer{
		services:    services,
		port:        port,
		health:      newHealthCache(services),
		marketplace: mp,
	}
	if wl != nil {
		ws.wasmLoader = wl
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ws.health.refreshAll(ctx)
	return ws
}

// Handler returns an http.Handler that serves the web UI routes.
func (w *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", w.handleDashboard)
	mux.HandleFunc("GET /integrations", w.handleIntegrationsList)
	mux.HandleFunc("GET /integrations/{name}", w.handleIntegrationDetail)
	mux.HandleFunc("POST /integrations/{name}", w.handleIntegrationSave)

	mux.HandleFunc("GET /integrations/slack/setup", w.handleSlackSetup)
	mux.HandleFunc("GET /api/slack/list-workspaces", w.handleSlackListWorkspaces)
	mux.HandleFunc("POST /api/slack/extract-browser", w.handleSlackExtractBrowser)
	mux.HandleFunc("POST /api/slack/save-tokens", w.handleSlackSaveTokens)
	mux.HandleFunc("POST /api/slack/set-default", w.handleSlackSetDefault)

	mux.HandleFunc("GET /integrations/github/setup", w.handleGitHubSetup)
	mux.HandleFunc("POST /api/github/oauth/start", w.handleGitHubOAuthStart)
	mux.HandleFunc("GET /api/github/oauth/poll", w.handleGitHubOAuthPoll)
	mux.HandleFunc("POST /api/github/oauth/save", w.handleGitHubOAuthSave)
	mux.HandleFunc("POST /api/github/save-token", w.handleGitHubSaveToken)

	mux.HandleFunc("GET /integrations/linear/setup", w.handleLinearSetup)
	mux.HandleFunc("POST /api/linear/save-token", w.handleLinearSaveToken)

	mux.HandleFunc("POST /api/remote/{name}/oauth/start", w.handleRemoteMCPOAuthStart)
	mux.HandleFunc("GET /api/remote/{name}/oauth/callback", w.handleRemoteMCPOAuthCallback)
	mux.HandleFunc("GET /api/remote/{name}/oauth/poll", w.handleRemoteMCPOAuthPoll)

	mux.HandleFunc("GET /integrations/sentry/setup", w.handleSentrySetup)
	mux.HandleFunc("POST /api/sentry/oauth/start", w.handleSentryOAuthStart)
	mux.HandleFunc("GET /api/sentry/oauth/poll", w.handleSentryOAuthPoll)
	mux.HandleFunc("POST /api/sentry/oauth/save", w.handleSentryOAuthSave)
	mux.HandleFunc("POST /api/sentry/save-token", w.handleSentrySaveToken)

	mux.HandleFunc("GET /integrations/gmail/setup", w.handleGmailSetup)
	mux.HandleFunc("POST /api/gmail/oauth/start", w.handleGmailOAuthStart)
	mux.HandleFunc("GET /api/gmail/oauth/callback", w.handleGmailOAuthCallback)
	mux.HandleFunc("GET /api/gmail/oauth/poll", w.handleGmailOAuthPoll)
	mux.HandleFunc("POST /api/gmail/save-token", w.handleGmailSaveToken)
	mux.HandleFunc("POST /api/gmail/save-oauth-credentials", w.handleGmailSaveOAuthCredentials)

	mux.HandleFunc("GET /integrations/gcal/setup", w.handleGcalSetup)
	mux.HandleFunc("POST /api/gcal/oauth/start", w.handleGcalOAuthStart)
	mux.HandleFunc("GET /api/gcal/oauth/callback", w.handleGcalOAuthCallback)
	mux.HandleFunc("GET /api/gcal/oauth/poll", w.handleGcalOAuthPoll)
	mux.HandleFunc("POST /api/gcal/save-token", w.handleGcalSaveToken)
	mux.HandleFunc("POST /api/gcal/save-oauth-credentials", w.handleGcalSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gdrive/setup", w.handleGdriveSetup)
	mux.HandleFunc("POST /api/gdrive/oauth/start", w.handleGdriveOAuthStart)
	mux.HandleFunc("GET /api/gdrive/oauth/callback", w.handleGdriveOAuthCallback)
	mux.HandleFunc("GET /api/gdrive/oauth/poll", w.handleGdriveOAuthPoll)
	mux.HandleFunc("POST /api/gdrive/save-token", w.handleGdriveSaveToken)
	mux.HandleFunc("POST /api/gdrive/save-oauth-credentials", w.handleGdriveSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gdocs/setup", w.handleGdocsSetup)
	mux.HandleFunc("POST /api/gdocs/oauth/start", w.handleGdocsOAuthStart)
	mux.HandleFunc("GET /api/gdocs/oauth/callback", w.handleGdocsOAuthCallback)
	mux.HandleFunc("GET /api/gdocs/oauth/poll", w.handleGdocsOAuthPoll)
	mux.HandleFunc("POST /api/gdocs/save-token", w.handleGdocsSaveToken)
	mux.HandleFunc("POST /api/gdocs/save-oauth-credentials", w.handleGdocsSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gsheets/setup", w.handleGsheetsSetup)
	mux.HandleFunc("POST /api/gsheets/oauth/start", w.handleGsheetsOAuthStart)
	mux.HandleFunc("GET /api/gsheets/oauth/callback", w.handleGsheetsOAuthCallback)
	mux.HandleFunc("GET /api/gsheets/oauth/poll", w.handleGsheetsOAuthPoll)
	mux.HandleFunc("POST /api/gsheets/save-token", w.handleGsheetsSaveToken)
	mux.HandleFunc("POST /api/gsheets/save-oauth-credentials", w.handleGsheetsSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gslides/setup", w.handleGslidesSetup)
	mux.HandleFunc("POST /api/gslides/oauth/start", w.handleGslidesOAuthStart)
	mux.HandleFunc("GET /api/gslides/oauth/callback", w.handleGslidesOAuthCallback)
	mux.HandleFunc("GET /api/gslides/oauth/poll", w.handleGslidesOAuthPoll)
	mux.HandleFunc("POST /api/gslides/save-token", w.handleGslidesSaveToken)
	mux.HandleFunc("POST /api/gslides/save-oauth-credentials", w.handleGslidesSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gforms/setup", w.handleGformsSetup)
	mux.HandleFunc("POST /api/gforms/oauth/start", w.handleGformsOAuthStart)
	mux.HandleFunc("GET /api/gforms/oauth/callback", w.handleGformsOAuthCallback)
	mux.HandleFunc("GET /api/gforms/oauth/poll", w.handleGformsOAuthPoll)
	mux.HandleFunc("POST /api/gforms/save-token", w.handleGformsSaveToken)
	mux.HandleFunc("POST /api/gforms/save-oauth-credentials", w.handleGformsSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gtasks/setup", w.handleGtasksSetup)
	mux.HandleFunc("POST /api/gtasks/oauth/start", w.handleGtasksOAuthStart)
	mux.HandleFunc("GET /api/gtasks/oauth/callback", w.handleGtasksOAuthCallback)
	mux.HandleFunc("GET /api/gtasks/oauth/poll", w.handleGtasksOAuthPoll)
	mux.HandleFunc("POST /api/gtasks/save-token", w.handleGtasksSaveToken)
	mux.HandleFunc("POST /api/gtasks/save-oauth-credentials", w.handleGtasksSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gchat/setup", w.handleGchatSetup)
	mux.HandleFunc("POST /api/gchat/oauth/start", w.handleGchatOAuthStart)
	mux.HandleFunc("GET /api/gchat/oauth/callback", w.handleGchatOAuthCallback)
	mux.HandleFunc("GET /api/gchat/oauth/poll", w.handleGchatOAuthPoll)
	mux.HandleFunc("POST /api/gchat/save-token", w.handleGchatSaveToken)
	mux.HandleFunc("POST /api/gchat/save-oauth-credentials", w.handleGchatSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gpeople/setup", w.handleGpeopleSetup)
	mux.HandleFunc("POST /api/gpeople/oauth/start", w.handleGpeopleOAuthStart)
	mux.HandleFunc("GET /api/gpeople/oauth/callback", w.handleGpeopleOAuthCallback)
	mux.HandleFunc("GET /api/gpeople/oauth/poll", w.handleGpeopleOAuthPoll)
	mux.HandleFunc("POST /api/gpeople/save-token", w.handleGpeopleSaveToken)
	mux.HandleFunc("POST /api/gpeople/save-oauth-credentials", w.handleGpeopleSaveOAuthCredentials)
	mux.HandleFunc("GET /integrations/gmeet/setup", w.handleGmeetSetup)
	mux.HandleFunc("POST /api/gmeet/oauth/start", w.handleGmeetOAuthStart)
	mux.HandleFunc("GET /api/gmeet/oauth/callback", w.handleGmeetOAuthCallback)
	mux.HandleFunc("GET /api/gmeet/oauth/poll", w.handleGmeetOAuthPoll)
	mux.HandleFunc("POST /api/gmeet/save-token", w.handleGmeetSaveToken)
	mux.HandleFunc("POST /api/gmeet/save-oauth-credentials", w.handleGmeetSaveOAuthCredentials)

	mux.HandleFunc("GET /integrations/notion/setup", w.handleNotionSetup)
	mux.HandleFunc("POST /api/notion/save-token", w.handleNotionSaveToken)

	mux.HandleFunc("GET /integrations/x/setup", w.handleXSetup)
	mux.HandleFunc("POST /api/x/oauth/start", w.handleXOAuthStart)
	mux.HandleFunc("GET /api/x/oauth/callback", w.handleXOAuthCallback)
	mux.HandleFunc("POST /api/x/save-token", w.handleXSaveToken)
	mux.HandleFunc("POST /api/x/save-oauth-credentials", w.handleXSaveOAuthCredentials)

	mux.HandleFunc("GET /integrations/postgres/setup", w.handlePostgresSetup)

	mux.HandleFunc("PUT /api/integrations/{name}/credentials", w.handleUpdateCredentials)

	mux.HandleFunc("GET /api/health", w.handleHealthAPI)
	mux.HandleFunc("POST /api/health/refresh", w.handleHealthRefresh)
	mux.HandleFunc("GET /api/metrics", w.handleMetricsAPI)

	mux.HandleFunc("GET /settings", w.handleSettings)
	mux.HandleFunc("POST /settings", w.handleSettingsSave)

	mux.HandleFunc("GET /plugins", w.handlePluginMarketplace)
	mux.HandleFunc("POST /plugins/install", w.handlePluginInstall)
	mux.HandleFunc("POST /plugins/install-url", w.handlePluginInstallURL)
	mux.HandleFunc("POST /plugins/upload", w.handlePluginUpload)
	mux.HandleFunc("POST /plugins/uninstall", w.handlePluginUninstall)
	mux.HandleFunc("POST /plugins/update", w.handlePluginUpdate)
	mux.HandleFunc("POST /plugins/check-updates", w.handlePluginCheckUpdates)
	mux.HandleFunc("POST /plugins/auto-update", w.handlePluginAutoUpdate)
	mux.HandleFunc("POST /plugins/add-manifest", w.handlePluginAddManifest)
	mux.HandleFunc("POST /plugins/remove-manifest", w.handlePluginRemoveManifest)
	mux.HandleFunc("POST /plugins/load-path", w.handlePluginLoadPath)

	return mux
}

func (w *WebServer) pageData(r *http.Request, title string, path string) layouts.PageData {
	data := layouts.PageData{
		Title:       title,
		CurrentPath: path,
	}
	if flash := r.URL.Query().Get("success"); flash != "" {
		data.FlashSuccess = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}
	return data
}

func (w *WebServer) integrationSummaries(_ context.Context) []pages.IntegrationSummary {
	var summaries []pages.IntegrationSummary
	for _, a := range w.services.Registry.All() {
		ic, exists := w.services.Config.GetIntegration(a.Name())
		enabled := exists && ic.Enabled
		isRemote := exists && ic.Credentials["mcp_access_token"] != "" && linearInt.MCPServerURL(a) != ""

		var healthy bool
		var lastCheck time.Time
		if entry, ok := w.health.get(a.Name()); ok {
			healthy = entry.Healthy
			if entry.Enabled {
				enabled = true
			}
			lastCheck = entry.CheckedAt
		}

		summaries = append(summaries, pages.IntegrationSummary{
			Name:      a.Name(),
			Enabled:   enabled,
			Healthy:   healthy,
			ToolCount: len(a.Tools()),
			LastCheck: lastCheck,
			IsRemote:  isRemote,
		})
	}
	return summaries
}

func (w *WebServer) handleDashboard(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(rw, r)
		return
	}

	summaries := w.integrationSummaries(r.Context())

	var connectedCount, disabledCount, erroredCount int
	var errored []pages.IntegrationSummary
	for _, s := range summaries {
		if !s.Enabled {
			disabledCount++
		} else if s.Healthy {
			connectedCount++
		} else {
			erroredCount++
			errored = append(errored, s)
		}
	}

	page := w.pageData(r, "Dashboard", "/")
	data := pages.DashboardData{
		ConnectedCount:      connectedCount,
		DisabledCount:       disabledCount,
		ErroredCount:        erroredCount,
		ErroredIntegrations: errored,
	}

	if w.services.Metrics != nil {
		cfg := w.services.Config.Get()
		rate := cfg.DollarsPerMTokInput
		if rate <= 0 {
			rate = mcp.DefaultInputDollarsPerMTok
		}
		snap := w.services.Metrics.SnapshotWithPricing(rate, cfg.ShowDollarEstimate)
		data.Metrics = &snap
		data.TopTools = w.services.Metrics.TopTools(5)
	}

	pages.Dashboard(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleIntegrationsList(rw http.ResponseWriter, r *http.Request) {
	summaries := w.integrationSummaries(r.Context())

	var errored, connected, disabled []pages.IntegrationSummary
	for _, s := range summaries {
		if !s.Enabled {
			disabled = append(disabled, s)
		} else if s.Healthy {
			connected = append(connected, s)
		} else {
			errored = append(errored, s)
		}
	}

	page := w.pageData(r, "Integrations", "/integrations")
	data := pages.IntegrationsListData{
		Errored:   errored,
		Connected: connected,
		Disabled:  disabled,
	}
	pages.IntegrationsList(page, data).Render(r.Context(), rw)
}

const notionExtractionSnippet = `(function(){var c=document.cookie.split(';').find(function(c){return c.trim().startsWith('token_v2=')});if(!c){alert('token_v2 cookie not found. Make sure you are on notion.so and signed in.');return;}var t=c.split('=').slice(1).join('=').trim();prompt('Copy this token_v2 value:',t);})()`

var setupIntegrations = map[string]bool{
	"slack":    true,
	"github":   true,
	"linear":   true,
	"sentry":   true,
	"gmail":    true,
	"gcal":     true,
	"gdrive":   true,
	"gdocs":    true,
	"gsheets":  true,
	"gslides":  true,
	"gforms":   true,
	"gtasks":   true,
	"gchat":    true,
	"gpeople":  true,
	"gmeet":    true,
	"notion":   true,
	"x":        true,
	"postgres": true,
}

func (w *WebServer) handleIntegrationDetail(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	integration, ok := w.services.Registry.Get(name)
	if !ok {
		http.NotFound(rw, r)
		return
	}

	if setupIntegrations[name] {
		http.Redirect(rw, r, "/integrations/"+name+"/setup", http.StatusSeeOther)
		return
	}

	ic, exists := w.services.Config.GetIntegration(name)
	enabled := exists && ic.Enabled

	var healthy bool
	if exists && enabled {
		if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
			healthy = integration.Healthy(r.Context())
		}
	}

	creds := mcp.Credentials{}
	for _, key := range w.services.Config.DefaultCredentialKeys(name) {
		creds[key] = ""
	}
	if exists {
		for k, v := range ic.Credentials {
			creds[k] = v
		}
	}

	plainTextKeys := map[string]bool{}
	if ptc, ok := integration.(mcp.PlainTextCredentials); ok {
		for _, k := range ptc.PlainTextKeys() {
			plainTextKeys[k] = true
		}
	}

	placeholders := map[string]string{}
	if ph, ok := integration.(mcp.PlaceholderHints); ok {
		placeholders = ph.Placeholders()
	}

	optionalKeys := map[string]bool{}
	if oc, ok := integration.(mcp.OptionalCredentials); ok {
		for _, k := range oc.OptionalKeys() {
			optionalKeys[k] = true
		}
	}

	var tools []pages.ToolInfo
	for _, t := range integration.Tools() {
		tools = append(tools, pages.ToolInfo{
			Name:        string(t.Name),
			Description: t.Description,
		})
	}

	page := w.pageData(r, integration.Name(), "/integrations")
	data := pages.IntegrationDetailData{
		Name:          name,
		Enabled:       enabled,
		Healthy:       healthy,
		Credentials:   pages.SortedCredentials(creds),
		PlainTextKeys: plainTextKeys,
		Placeholders:  placeholders,
		OptionalKeys:  optionalKeys,
		Tools:         tools,
	}

	pages.IntegrationDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleIntegrationSave(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	_, ok := w.services.Registry.Get(name)
	if !ok {
		http.NotFound(rw, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/"+name+"?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	enabled := r.FormValue("enabled") == "true"

	creds := mcp.Credentials{}
	for key, values := range r.Form {
		if credKey, ok := strings.CutPrefix(key, "cred_"); ok {
			if len(values) > 0 {
				creds[credKey] = values[0]
			}
		}
	}

	ic := &mcp.IntegrationConfig{
		Enabled:     enabled,
		Credentials: creds,
	}
	if existingIC, ok := w.services.Config.GetIntegration(name); ok {
		ic.ToolGlobs = existingIC.ToolGlobs
	}

	if err := w.services.Config.SetIntegration(name, ic); err != nil {
		redirect := "/integrations/" + name
		if setupIntegrations[name] {
			redirect += "/setup"
		}
		http.Redirect(rw, r, redirect+"?error=Failed+to+save:+"+err.Error(), http.StatusSeeOther)
		return
	}

	redirect := "/integrations/" + name
	if setupIntegrations[name] {
		redirect += "/setup"
	}
	http.Redirect(rw, r, redirect+"?success=Configuration+saved", http.StatusSeeOther)
}

// handleUpdateCredentials is a JSON API for hot-reloading integration credentials
// without restarting Switchboard. The agent supervisor calls this when a token
// is refreshed (e.g. GitHub App installation token rotation).
//
//	PUT /api/integrations/{name}/credentials
//	Body: {"token": "ghp_...", "other_key": "..."}
//	Response: 200 {"ok": true} or 4xx/5xx {"error": "..."}
func (w *WebServer) handleUpdateCredentials(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	integration, ok := w.services.Registry.Get(name)
	if !ok {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "unknown integration: " + name})
		return
	}

	var creds mcp.Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Merge with existing credentials so callers can send partial updates
	// (e.g. only the rotated token, keeping client_id etc.).
	ic, exists := w.services.Config.GetIntegration(name)
	if exists {
		merged := mcp.Credentials{}
		for k, v := range ic.Credentials {
			merged[k] = v
		}
		for k, v := range creds {
			merged[k] = v
		}
		creds = merged
	}

	if err := integration.Configure(r.Context(), creds); err != nil {
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "configure failed: " + err.Error()})
		return
	}

	// Persist so the config file stays in sync.
	if ic == nil {
		ic = &mcp.IntegrationConfig{}
	}
	ic.Enabled = true
	ic.Credentials = creds
	_ = w.services.Config.SetIntegration(name, ic)

	writeJSON(rw, http.StatusOK, map[string]string{"ok": "true"})
}

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	json.NewEncoder(rw).Encode(v)
}

func (w *WebServer) handleHealthAPI(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(`{"status":"healthy"}`))
}

func (w *WebServer) handleHealthRefresh(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	w.health.refreshAll(ctx)
	http.Redirect(rw, r, r.Referer(), http.StatusSeeOther)
}

func (w *WebServer) handleSlackSetup(rw http.ResponseWriter, r *http.Request) {
	info := slackInt.GetTokenInfoForWeb()

	ic, exists := w.services.Config.GetIntegration("slack")

	var healthy bool
	if info.HasToken {
		integration, ok := w.services.Registry.Get("slack")
		if ok && exists {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}
	if tokenSource == "" && info.Source != "" {
		tokenSource = info.Source
	}

	tokenStatus := info.Status
	if healthy && tokenStatus != "healthy" {
		tokenStatus = "healthy"
	}

	page := w.pageData(r, "Slack Setup", "/integrations")
	data := pages.SlackSetupData{
		HasToken:       info.HasToken,
		HasCookie:      info.HasCookie,
		TokenStatus:    tokenStatus,
		TokenAge:       info.AgeHours,
		TokenSource:    tokenSource,
		CanAutoExtract: slackInt.CanExtractFromBrowser(),
		ExtractSnippet: slackInt.ExtractionSnippet(),
		Healthy:        healthy,
		WorkspaceCount: info.WorkspaceCount,
		DefaultTeamID:  info.TeamID,
	}

	// Populate workspace list for the default workspace selector.
	cwsList, _ := slackInt.GetConfiguredWorkspacesForWeb()
	for _, cws := range cwsList {
		data.Workspaces = append(data.Workspaces, pages.SlackWorkspaceItem{
			TeamID:    cws.TeamID,
			TeamName:  cws.TeamName,
			IsDefault: cws.IsDefault,
		})
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.SlackSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleSlackListWorkspaces(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	workspaces, err := slackInt.ListWorkspacesFromBrowsers()
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]any{"error": err.Error()})
		return
	}
	json.NewEncoder(rw).Encode(map[string]any{"workspaces": workspaces})
}

func (w *WebServer) handleSlackExtractBrowser(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	count, err := slackInt.ExtractAllFromBrowsersForWeb()
	if err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	// Enable the integration in config.
	ic, _ := w.services.Config.GetIntegration("slack")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials[mcp.CredKeyTokenSource] = "browser"
	_ = w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, fmt.Sprintf("/integrations/slack/setup?result=Extracted+%d+workspaces+from+browser", count), http.StatusSeeOther)
}

func (w *WebServer) handleGitHubSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("github")
	hasToken := exists && ic.Credentials["token"] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("github")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	page := w.pageData(r, "GitHub Setup", "/integrations")
	data := pages.GitHubSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		ClientID:    clientID,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GitHubSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGitHubOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("github")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "GitHub OAuth client_id is not configured"})
		return
	}

	dcr, err := ghInt.StartOAuthFlow(ic.Credentials[mcp.CredKeyClientID])
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(dcr)
}

func (w *WebServer) handleGitHubOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := ghInt.PollOAuthFlow(r.Context())
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGitHubOAuthSave(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		rw.WriteHeader(400)
		json.NewEncoder(rw).Encode(map[string]string{"error": "Invalid token"})
		return
	}

	ic, _ := w.services.Config.GetIntegration("github")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token"] = body.Token
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("github", ic)

	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (w *WebServer) handleGitHubSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/github/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Redirect(rw, r, "/integrations/github/setup?error=Token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("github")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token"] = token
	ic.Credentials[mcp.CredKeyTokenSource] = "pat"
	_ = w.services.Config.SetIntegration("github", ic)

	http.Redirect(rw, r, "/integrations/github/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleLinearSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("linear")

	integration, integrationOK := w.services.Registry.Get("linear")
	mcpServerURL := ""
	if integrationOK {
		mcpServerURL = linearInt.MCPServerURL(integration)
	}
	hasRemoteMCP := mcpServerURL != ""

	hasAPIKey := exists && ic.Credentials["api_key"] != ""
	hasMCPToken := exists && ic.Credentials["mcp_access_token"] != ""

	var apiKeyHealthy, remoteMCPHealthy bool

	if hasAPIKey && integrationOK {
		apiKeyCreds := mcp.Credentials{"api_key": ic.Credentials["api_key"]}
		if err := integration.Configure(r.Context(), apiKeyCreds); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			apiKeyHealthy = integration.Healthy(ctx)
			cancel()
		}
	}

	if hasMCPToken && hasRemoteMCP {
		mcpCreds := mcp.Credentials{"mcp_access_token": ic.Credentials["mcp_access_token"]}
		if err := integration.Configure(r.Context(), mcpCreds); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			remoteMCPHealthy = integration.Healthy(ctx)
			cancel()
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	page := w.pageData(r, "Linear Setup", "/integrations")
	data := pages.LinearSetupData{
		HasRemoteMCP:     hasRemoteMCP,
		RemoteMCPHealthy: remoteMCPHealthy,
		HasAPIKey:        hasAPIKey,
		APIKeyHealthy:    apiKeyHealthy,
		TokenSource:      tokenSource,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.LinearSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleLinearSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/linear/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	if apiKey == "" {
		http.Redirect(rw, r, "/integrations/linear/setup?error=API+key+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("linear")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["api_key"] = apiKey
	ic.Credentials["mcp_access_token"] = ""
	ic.Credentials[mcp.CredKeyTokenSource] = "api_key"
	_ = w.services.Config.SetIntegration("linear", ic)

	http.Redirect(rw, r, "/integrations/linear/setup?result=API+key+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleSentrySetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("sentry")
	hasToken := exists && ic.Credentials["auth_token"] != ""
	clientID := ""
	organization := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
		organization = ic.Credentials["organization"]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("sentry")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	page := w.pageData(r, "Sentry Setup", "/integrations")
	data := pages.SentrySetupData{
		HasToken:     hasToken,
		Healthy:      healthy,
		TokenSource:  tokenSource,
		Organization: organization,
		ClientID:     clientID,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.SentrySetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleSentryOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("sentry")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Sentry OAuth client_id is not configured"})
		return
	}

	dcr, err := sentryInt.StartOAuthFlow(ic.Credentials[mcp.CredKeyClientID])
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(dcr)
}

func (w *WebServer) handleSentryOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := sentryInt.PollOAuthFlow()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleSentryOAuthSave(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	var body struct {
		Token        string `json:"token"`
		Organization string `json:"organization"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" || body.Organization == "" {
		rw.WriteHeader(400)
		json.NewEncoder(rw).Encode(map[string]string{"error": "Token and organization are required"})
		return
	}

	ic, _ := w.services.Config.GetIntegration("sentry")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["auth_token"] = body.Token
	ic.Credentials["organization"] = body.Organization
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("sentry", ic)

	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (w *WebServer) handleSentrySaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/sentry/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	authToken := strings.TrimSpace(r.FormValue("auth_token"))
	organization := strings.TrimSpace(r.FormValue("organization"))
	if authToken == "" {
		http.Redirect(rw, r, "/integrations/sentry/setup?error=Auth+token+is+required", http.StatusSeeOther)
		return
	}
	if organization == "" {
		http.Redirect(rw, r, "/integrations/sentry/setup?error=Organization+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("sentry")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["auth_token"] = authToken
	ic.Credentials["organization"] = organization
	ic.Credentials[mcp.CredKeyTokenSource] = "token"
	_ = w.services.Config.SetIntegration("sentry", ic)

	http.Redirect(rw, r, "/integrations/sentry/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleSlackSaveTokens(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	cookie := strings.TrimSpace(r.FormValue("cookie"))
	extractedJSON := strings.TrimSpace(r.FormValue("extracted_json"))

	// If user pasted the JSON blob from the browser snippet, parse it.
	if extractedJSON != "" {
		var parsed struct {
			Token  string `json:"token"`
			Cookie string `json:"cookie"`
		}
		if err := json.Unmarshal([]byte(extractedJSON), &parsed); err != nil {
			http.Redirect(rw, r, "/integrations/slack/setup?error=Invalid+JSON.+Make+sure+you+copied+the+entire+value+from+the+prompt.", http.StatusSeeOther)
			return
		}
		token = parsed.Token
		cookie = parsed.Cookie
	}

	if token == "" {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Token+is+required", http.StatusSeeOther)
		return
	}

	_, err := slackInt.SaveTokensForWeb(token, cookie, "")
	if err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Failed+to+save:+"+err.Error(), http.StatusSeeOther)
		return
	}

	// Also update config.
	ic, _ := w.services.Config.GetIntegration("slack")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token"] = token
	ic.Credentials["cookie"] = cookie
	ic.Credentials[mcp.CredKeyTokenSource] = "browser"
	_ = w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, "/integrations/slack/setup?result=Tokens+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleSlackSetDefault(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	teamID := strings.TrimSpace(r.FormValue("team_id"))
	if teamID == "" {
		http.Redirect(rw, r, "/integrations/slack/setup?error=No+workspace+selected", http.StatusSeeOther)
		return
	}

	if err := slackInt.SetDefaultWorkspaceForWeb(teamID); err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	// Also update config.
	ic, _ := w.services.Config.GetIntegration("slack")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials["team_id"] = teamID
	_ = w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, "/integrations/slack/setup?result=Default+workspace+updated", http.StatusSeeOther)
}

func (w *WebServer) handleNotionSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("notion")
	hasToken := exists && ic.Credentials["token_v2"] != ""

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("notion")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	page := w.pageData(r, "Notion Setup", "/integrations")
	data := pages.NotionSetupData{
		HasToken:       hasToken,
		Healthy:        healthy,
		ExtractSnippet: notionExtractionSnippet,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.NotionSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleNotionSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/notion/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	tokenV2 := strings.TrimSpace(r.FormValue("token_v2"))
	if tokenV2 == "" {
		http.Redirect(rw, r, "/integrations/notion/setup?error=token_v2+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("notion")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token_v2"] = tokenV2
	_ = w.services.Config.SetIntegration("notion", ic)

	http.Redirect(rw, r, "/integrations/notion/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handlePostgresSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("postgres")
	enabled := exists && ic.Enabled

	// Avoid calling Configure() on every page load — with multi-postgres each
	// invocation re-opens connections to all configured databases and pings them
	// (5s timeout each), which can hang the setup page noticeably. The integration
	// is already configured at startup; just probe the existing connection's health.
	var healthy bool
	if enabled {
		integration, ok := w.services.Registry.Get("postgres")
		if ok {
			ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
			healthy = integration.Healthy(ctx)
			cancel()
		}
	}

	data := pages.PostgresSetupData{
		Enabled: enabled,
		Healthy: healthy,
	}

	if exists && ic.Credentials != nil {
		data.Default = pages.PostgresConnection{
			ConnectionString: ic.Credentials["connection_string"],
			Host:             ic.Credentials["host"],
			Port:             ic.Credentials["port"],
			User:             ic.Credentials["user"],
			Password:         ic.Credentials["password"],
			Database:         ic.Credentials["database"],
			SSLMode:          ic.Credentials["sslmode"],
			ReadOnly:         ic.Credentials["read_only"],
		}
		if raw := ic.Credentials["connections"]; raw != "" {
			var conns []pages.PostgresConnection
			if err := json.Unmarshal([]byte(raw), &conns); err == nil {
				data.Connections = conns
			}
		}
	}

	integration, ok := w.services.Registry.Get("postgres")
	if ok {
		for _, t := range integration.Tools() {
			data.Tools = append(data.Tools, pages.ToolInfo{
				Name:        string(t.Name),
				Description: t.Description,
			})
		}
	}

	page := w.pageData(r, "Postgres Setup", "/integrations")
	pages.PostgresSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleRemoteMCPOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	name := r.PathValue("name")

	integration, ok := w.services.Registry.Get(name)
	if !ok {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Unknown integration: " + name})
		return
	}

	serverURL := remotemcp.ServerURL(integration)
	if serverURL == "" {
		serverURL = linearInt.MCPServerURL(integration)
	}
	if serverURL == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Not a remote MCP integration"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/remote/%s/oauth/callback", w.port, name)
	authorizeURL, err := remotemcp.StartOAuth(name, serverURL, redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(map[string]string{"authorize_url": authorizeURL})
}

func (w *WebServer) handleRemoteMCPOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	setupPath := "/integrations/" + name + "/setup"

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, setupPath+"?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := remotemcp.HandleOAuthCallback(name, code, state); err != nil {
		http.Redirect(rw, r, setupPath+"?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	status, token, errStr := remotemcp.PollOAuth(name)
	if status != "complete" || token == "" {
		msg := "Failed to get access token"
		if errStr != "" {
			msg = errStr
		}
		http.Redirect(rw, r, setupPath+"?error="+strings.ReplaceAll(msg, " ", "+"), http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration(name)
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["mcp_access_token"] = token
	ic.Credentials["api_key"] = ""
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration(name, ic)

	http.Redirect(rw, r, setupPath+"?result=Connected+via+MCP+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleRemoteMCPOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	name := r.PathValue("name")
	status, token, errStr := remotemcp.PollOAuth(name)
	json.NewEncoder(rw).Encode(map[string]string{"status": status, "token": token, "error": errStr})
}

func (w *WebServer) handleGmailSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gmail")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gmail")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gmail/oauth/callback", w.port)

	page := w.pageData(r, "Gmail Setup", "/integrations")
	data := pages.GmailSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GmailSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGmailOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gmail")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Gmail OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gmail/oauth/callback", w.port)
	result, err := gmail.StartGmailOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGmailOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gmail/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gmail.HandleGmailCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gmail/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gmail.PollGmailOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gmail/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmail")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gmail", ic)

	http.Redirect(rw, r, "/integrations/gmail/setup?result=Connected+to+Gmail+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGmailOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gmail.PollGmailOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGmailSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gmail/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gmail/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmail")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gmail", ic)

	http.Redirect(rw, r, "/integrations/gmail/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGmailSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gmail/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gmail/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmail")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gmail", ic)

	http.Redirect(rw, r, "/integrations/gmail/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGcalSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gcal")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gcal")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gcal/oauth/callback", w.port)

	page := w.pageData(r, "Google Calendar Setup", "/integrations")
	data := pages.GcalSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GcalSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGcalOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gcal")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Calendar OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gcal/oauth/callback", w.port)
	result, err := gcal.StartGcalOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGcalOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gcal/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gcal.HandleGcalCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gcal/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gcal.PollGcalOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gcal/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gcal")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gcal", ic)

	http.Redirect(rw, r, "/integrations/gcal/setup?result=Connected+to+Google+Calendar+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGcalOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gcal.PollGcalOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGcalSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gcal/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gcal/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gcal")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gcal", ic)

	http.Redirect(rw, r, "/integrations/gcal/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGcalSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gcal/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gcal/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gcal")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gcal", ic)

	http.Redirect(rw, r, "/integrations/gcal/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGdriveSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gdrive")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gdrive")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gdrive/oauth/callback", w.port)

	page := w.pageData(r, "Google Drive Setup", "/integrations")
	data := pages.GdriveSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GdriveSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGdriveOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gdrive")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Drive OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gdrive/oauth/callback", w.port)
	result, err := gdrive.StartGdriveOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGdriveOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gdrive/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gdrive.HandleGdriveCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gdrive.PollGdriveOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdrive")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gdrive", ic)

	http.Redirect(rw, r, "/integrations/gdrive/setup?result=Connected+to+Google+Drive+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGdriveOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gdrive.PollGdriveOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGdriveSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdrive")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gdrive", ic)

	http.Redirect(rw, r, "/integrations/gdrive/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGdriveSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gdrive/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdrive")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gdrive", ic)

	http.Redirect(rw, r, "/integrations/gdrive/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGdocsSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gdocs")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gdocs")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gdocs/oauth/callback", w.port)

	page := w.pageData(r, "Google Docs Setup", "/integrations")
	data := pages.GdocsSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GdocsSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGdocsOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gdocs")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Docs OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gdocs/oauth/callback", w.port)
	result, err := gdocs.StartGdocsOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGdocsOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gdocs/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gdocs.HandleGdocsCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gdocs.PollGdocsOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdocs")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gdocs", ic)

	http.Redirect(rw, r, "/integrations/gdocs/setup?result=Connected+to+Google+Docs+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGdocsOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gdocs.PollGdocsOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGdocsSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdocs")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gdocs", ic)

	http.Redirect(rw, r, "/integrations/gdocs/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGdocsSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gdocs/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gdocs")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gdocs", ic)

	http.Redirect(rw, r, "/integrations/gdocs/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGsheetsSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gsheets")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gsheets")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gsheets/oauth/callback", w.port)

	page := w.pageData(r, "Google Sheets Setup", "/integrations")
	data := pages.GsheetsSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GsheetsSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGsheetsOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gsheets")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Sheets OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gsheets/oauth/callback", w.port)
	result, err := gsheets.StartGsheetsOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGsheetsOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gsheets/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gsheets.HandleGsheetsCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gsheets.PollGsheetsOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gsheets")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gsheets", ic)

	http.Redirect(rw, r, "/integrations/gsheets/setup?result=Connected+to+Google+Sheets+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGsheetsOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gsheets.PollGsheetsOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGsheetsSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gsheets")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gsheets", ic)

	http.Redirect(rw, r, "/integrations/gsheets/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGsheetsSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gsheets/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gsheets")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gsheets", ic)

	http.Redirect(rw, r, "/integrations/gsheets/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGslidesSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gslides")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gslides")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gslides/oauth/callback", w.port)

	page := w.pageData(r, "Google Slides Setup", "/integrations")
	data := pages.GslidesSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GslidesSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGslidesOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gslides")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Slides OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gslides/oauth/callback", w.port)
	result, err := gslides.StartGslidesOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGslidesOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gslides/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gslides.HandleGslidesCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gslides/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gslides.PollGslidesOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gslides/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gslides")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gslides", ic)

	http.Redirect(rw, r, "/integrations/gslides/setup?result=Connected+to+Google+Slides+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGslidesOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gslides.PollGslidesOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGslidesSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gslides/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gslides/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gslides")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gslides", ic)

	http.Redirect(rw, r, "/integrations/gslides/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGslidesSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gslides/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gslides/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gslides")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gslides", ic)

	http.Redirect(rw, r, "/integrations/gslides/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGformsSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gforms")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gforms")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gforms/oauth/callback", w.port)

	page := w.pageData(r, "Google Forms Setup", "/integrations")
	data := pages.GformsSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GformsSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGformsOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gforms")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Forms OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gforms/oauth/callback", w.port)
	result, err := gforms.StartGformsOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGformsOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gforms/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gforms.HandleGformsCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gforms/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gforms.PollGformsOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gforms/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gforms")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gforms", ic)

	http.Redirect(rw, r, "/integrations/gforms/setup?result=Connected+to+Google+Forms+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGformsOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gforms.PollGformsOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGformsSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gforms/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gforms/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gforms")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gforms", ic)

	http.Redirect(rw, r, "/integrations/gforms/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGformsSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gforms/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gforms/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gforms")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gforms", ic)

	http.Redirect(rw, r, "/integrations/gforms/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGtasksSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gtasks")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gtasks")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gtasks/oauth/callback", w.port)

	page := w.pageData(r, "Google Tasks Setup", "/integrations")
	data := pages.GtasksSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GtasksSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGtasksOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gtasks")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Tasks OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gtasks/oauth/callback", w.port)
	result, err := gtasks.StartGtasksOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGtasksOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gtasks/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gtasks.HandleGtasksCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gtasks.PollGtasksOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gtasks")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gtasks", ic)

	http.Redirect(rw, r, "/integrations/gtasks/setup?result=Connected+to+Google+Tasks+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGtasksOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gtasks.PollGtasksOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGtasksSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gtasks")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gtasks", ic)

	http.Redirect(rw, r, "/integrations/gtasks/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGtasksSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gtasks/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gtasks")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gtasks", ic)

	http.Redirect(rw, r, "/integrations/gtasks/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGchatSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gchat")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gchat")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gchat/oauth/callback", w.port)

	page := w.pageData(r, "Google Chat Setup", "/integrations")
	data := pages.GchatSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GchatSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGchatOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gchat")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Chat OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gchat/oauth/callback", w.port)
	result, err := gchat.StartGchatOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGchatOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gchat/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gchat.HandleGchatCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gchat/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gchat.PollGchatOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gchat/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gchat")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gchat", ic)

	http.Redirect(rw, r, "/integrations/gchat/setup?result=Connected+to+Google+Chat+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGchatOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gchat.PollGchatOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGchatSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gchat/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gchat/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gchat")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gchat", ic)

	http.Redirect(rw, r, "/integrations/gchat/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGchatSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gchat/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gchat/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gchat")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gchat", ic)

	http.Redirect(rw, r, "/integrations/gchat/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGpeopleSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gpeople")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gpeople")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gpeople/oauth/callback", w.port)

	page := w.pageData(r, "Google People Setup", "/integrations")
	data := pages.GpeopleSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GpeopleSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGpeopleOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gpeople")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google People OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gpeople/oauth/callback", w.port)
	result, err := gpeople.StartGpeopleOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGpeopleOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gpeople/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gpeople.HandleGpeopleCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gpeople.PollGpeopleOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gpeople")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gpeople", ic)

	http.Redirect(rw, r, "/integrations/gpeople/setup?result=Connected+to+Google+People+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGpeopleOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gpeople.PollGpeopleOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGpeopleSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gpeople")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gpeople", ic)

	http.Redirect(rw, r, "/integrations/gpeople/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGpeopleSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gpeople/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gpeople")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gpeople", ic)

	http.Redirect(rw, r, "/integrations/gpeople/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleGmeetSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("gmeet")
	hasToken := exists && ic.Credentials["access_token"] != ""
	hasOAuth := exists && ic.Credentials[mcp.CredKeyClientID] != "" && ic.Credentials[mcp.CredKeyClientSecret] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials[mcp.CredKeyClientID]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("gmeet")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials[mcp.CredKeyTokenSource] != "" {
		tokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gmeet/oauth/callback", w.port)

	page := w.pageData(r, "Google Meet Setup", "/integrations")
	data := pages.GmeetSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.GmeetSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleGmeetOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("gmeet")
	if !exists || ic.Credentials[mcp.CredKeyClientID] == "" || ic.Credentials[mcp.CredKeyClientSecret] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Google Meet OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/gmeet/oauth/callback", w.port)
	result, err := gmeet.StartGmeetOAuth(ic.Credentials[mcp.CredKeyClientID], ic.Credentials[mcp.CredKeyClientSecret], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGmeetOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/gmeet/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := gmeet.HandleGmeetCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := gmeet.PollGmeetOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmeet")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials[mcp.CredKeyTokenSource] = "oauth"
	_ = w.services.Config.SetIntegration("gmeet", ic)

	http.Redirect(rw, r, "/integrations/gmeet/setup?result=Connected+to+Google+Meet+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleGmeetOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := gmeet.PollGmeetOAuth()
	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleGmeetSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	accessToken := strings.TrimSpace(r.FormValue("access_token"))
	if accessToken == "" {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error=Access+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmeet")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["access_token"] = accessToken
	ic.Credentials[mcp.CredKeyTokenSource] = "manual"
	_ = w.services.Config.SetIntegration("gmeet", ic)

	http.Redirect(rw, r, "/integrations/gmeet/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleGmeetSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/gmeet/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("gmeet")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials[mcp.CredKeyClientID] = clientID
	ic.Credentials[mcp.CredKeyClientSecret] = clientSecret
	_ = w.services.Config.SetIntegration("gmeet", ic)

	http.Redirect(rw, r, "/integrations/gmeet/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+Google.", http.StatusSeeOther)
}

func (w *WebServer) handleMetricsAPI(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	if w.services.Metrics == nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
		_, _ = rw.Write([]byte(`{"error":"metrics not initialized"}`))
		return
	}
	cfg := w.services.Config.Get()
	rate := cfg.DollarsPerMTokInput
	if rate <= 0 {
		rate = mcp.DefaultInputDollarsPerMTok
	}
	snap := w.services.Metrics.SnapshotWithPricing(rate, cfg.ShowDollarEstimate)
	json.NewEncoder(rw).Encode(snap)
}

func (w *WebServer) handleSettings(rw http.ResponseWriter, r *http.Request) {
	cfg := w.services.Config.Get()
	page := w.pageData(r, "Settings", "/settings")
	data := pages.SettingsData{
		SessionStore:        cfg.SessionStore,
		ShowDollarEstimate:  cfg.ShowDollarEstimate,
		DollarsPerMTokInput: cfg.DollarsPerMTokInput,
	}
	if data.SessionStore == "" {
		data.SessionStore = "memory"
	}
	pages.Settings(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleSettingsSave(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/settings?error=Invalid+form+data", http.StatusSeeOther)
		return
	}
	sessionStore := r.FormValue("session_store")
	if sessionStore != "memory" && sessionStore != "file" {
		sessionStore = "memory"
	}
	showDollar := r.FormValue("show_dollar_estimate") == "true"
	var dollarsPerMTok float64
	if raw := strings.TrimSpace(r.FormValue("dollars_per_mtok_input")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			dollarsPerMTok = v
		}
	}
	cfg := w.services.Config.Get()
	cfg.SessionStore = sessionStore
	cfg.ShowDollarEstimate = showDollar
	cfg.DollarsPerMTokInput = dollarsPerMTok
	if err := w.services.Config.Update(cfg); err != nil {
		http.Redirect(rw, r, "/settings?error=Failed+to+save:+"+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(rw, r, "/settings?success=Settings+saved.", http.StatusSeeOther)
}

func (w *WebServer) handleXSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("x")
	hasToken := exists && ic.Credentials["bearer_token"] != ""
	hasOAuth := exists && ic.Credentials["client_id"] != "" && ic.Credentials["client_secret"] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials["client_id"]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("x")
		if ok {
			if err := integration.Configure(r.Context(), ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials["token_source"] != "" {
		tokenSource = ic.Credentials["token_source"]
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/x/oauth/callback", w.port)

	var tools []pages.ToolInfo
	if integration, ok := w.services.Registry.Get("x"); ok {
		for _, t := range integration.Tools() {
			tools = append(tools, pages.ToolInfo{
				Name:        string(t.Name),
				Description: t.Description,
			})
		}
	}

	page := w.pageData(r, "X Setup", "/integrations")
	data := pages.XSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Tools:       tools,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.XSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleXOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("x")
	if !exists || ic.Credentials["client_id"] == "" || ic.Credentials["client_secret"] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "X OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/x/oauth/callback", w.port)
	result, err := xInt.StartXOAuth(ic.Credentials["client_id"], ic.Credentials["client_secret"], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleXOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/x/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := xInt.HandleXCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/x/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := xInt.PollXOAuth()
	if result.Status != "complete" || result.AccessToken == "" {
		http.Redirect(rw, r, "/integrations/x/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("x")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["bearer_token"] = result.AccessToken
	if result.RefreshToken != "" {
		ic.Credentials["refresh_token"] = result.RefreshToken
	}
	ic.Credentials["token_source"] = "oauth"
	_ = w.services.Config.SetIntegration("x", ic)

	http.Redirect(rw, r, "/integrations/x/setup?result=Connected+to+X+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleXSaveToken(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/x/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	bearerToken := strings.TrimSpace(r.FormValue("bearer_token"))
	if bearerToken == "" {
		http.Redirect(rw, r, "/integrations/x/setup?error=Bearer+token+is+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("x")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["bearer_token"] = bearerToken
	ic.Credentials["token_source"] = "manual"
	_ = w.services.Config.SetIntegration("x", ic)

	http.Redirect(rw, r, "/integrations/x/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleXSaveOAuthCredentials(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/integrations/x/setup?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		http.Redirect(rw, r, "/integrations/x/setup?error=Client+ID+and+Client+Secret+are+required", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("x")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Credentials["client_id"] = clientID
	ic.Credentials["client_secret"] = clientSecret
	_ = w.services.Config.SetIntegration("x", ic)

	http.Redirect(rw, r, "/integrations/x/setup?result=OAuth+credentials+saved.+You+can+now+sign+in+with+X.", http.StatusSeeOther)
}
