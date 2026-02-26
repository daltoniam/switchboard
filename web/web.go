package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	ghInt "github.com/daltoniam/switchboard/github"
	linearInt "github.com/daltoniam/switchboard/linear"
	sentryInt "github.com/daltoniam/switchboard/sentry"
	slackInt "github.com/daltoniam/switchboard/slack"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// WebServer serves the configuration web UI using templ templates.
type WebServer struct {
	services *mcp.Services
	port     int
}

// New returns a WebServer that provides a browser-based config UI.
func New(services *mcp.Services, port int) *WebServer {
	return &WebServer{
		services: services,
		port:     port,
	}
}

// Handler returns an http.Handler that serves the web UI routes.
func (w *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", w.handleDashboard)
	mux.HandleFunc("GET /integrations", w.handleIntegrationsList)
	mux.HandleFunc("GET /integrations/{name}", w.handleIntegrationDetail)
	mux.HandleFunc("POST /integrations/{name}", w.handleIntegrationSave)

	mux.HandleFunc("GET /integrations/slack/setup", w.handleSlackSetup)
	mux.HandleFunc("POST /api/slack/extract-chrome", w.handleSlackExtractChrome)
	mux.HandleFunc("POST /api/slack/save-tokens", w.handleSlackSaveTokens)

	mux.HandleFunc("GET /integrations/github/setup", w.handleGitHubSetup)
	mux.HandleFunc("POST /api/github/oauth/start", w.handleGitHubOAuthStart)
	mux.HandleFunc("GET /api/github/oauth/poll", w.handleGitHubOAuthPoll)
	mux.HandleFunc("POST /api/github/oauth/save", w.handleGitHubOAuthSave)
	mux.HandleFunc("POST /api/github/save-token", w.handleGitHubSaveToken)

	mux.HandleFunc("GET /integrations/linear/setup", w.handleLinearSetup)
	mux.HandleFunc("POST /api/linear/oauth/start", w.handleLinearOAuthStart)
	mux.HandleFunc("GET /api/linear/oauth/callback", w.handleLinearOAuthCallback)
	mux.HandleFunc("GET /api/linear/oauth/poll", w.handleLinearOAuthPoll)
	mux.HandleFunc("POST /api/linear/save-token", w.handleLinearSaveToken)

	mux.HandleFunc("GET /integrations/sentry/setup", w.handleSentrySetup)
	mux.HandleFunc("POST /api/sentry/oauth/start", w.handleSentryOAuthStart)
	mux.HandleFunc("GET /api/sentry/oauth/poll", w.handleSentryOAuthPoll)
	mux.HandleFunc("POST /api/sentry/oauth/save", w.handleSentryOAuthSave)
	mux.HandleFunc("POST /api/sentry/save-token", w.handleSentrySaveToken)

	mux.HandleFunc("POST /api/slack/oauth/start", w.handleSlackOAuthStart)
	mux.HandleFunc("GET /api/slack/oauth/callback", w.handleSlackOAuthCallback)
	mux.HandleFunc("GET /api/slack/oauth/poll", w.handleSlackOAuthPoll)

	mux.HandleFunc("GET /api/health", w.handleHealthAPI)

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

func (w *WebServer) integrationSummaries(ctx context.Context) []pages.IntegrationSummary {
	var summaries []pages.IntegrationSummary
	for _, a := range w.services.Registry.All() {
		ic, exists := w.services.Config.GetIntegration(a.Name())
		enabled := exists && ic.Enabled

		var healthy bool
		if exists {
			if err := a.Configure(ic.Credentials); err == nil {
				healthy = a.Healthy(ctx)
				if healthy && !enabled {
					enabled = true
					ic.Enabled = true
					w.services.Config.SetIntegration(a.Name(), ic)
				}
			}
		}

		summaries = append(summaries, pages.IntegrationSummary{
			Name:      a.Name(),
			Enabled:   enabled,
			Healthy:   healthy,
			ToolCount: len(a.Tools()),
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

	var enabledCount, healthyCount, totalTools int
	for _, s := range summaries {
		if s.Enabled {
			enabledCount++
		}
		if s.Healthy {
			healthyCount++
		}
		totalTools += s.ToolCount
	}

	page := w.pageData(r, "Dashboard", "/")
	data := pages.DashboardData{
		TotalIntegrations: len(summaries),
		EnabledCount:      enabledCount,
		HealthyCount:      healthyCount,
		TotalTools:        totalTools,
		Integrations:      summaries,
	}

	pages.Dashboard(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleIntegrationsList(rw http.ResponseWriter, r *http.Request) {
	summaries := w.integrationSummaries(r.Context())
	page := w.pageData(r, "Integrations", "/integrations")
	pages.IntegrationsList(page, summaries).Render(r.Context(), rw)
}

var setupIntegrations = map[string]bool{
	"slack":  true,
	"github": true,
	"linear": true,
	"sentry": true,
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
	var creds mcp.Credentials
	if exists {
		creds = ic.Credentials
		if enabled {
			if err := integration.Configure(ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}
	if creds == nil {
		creds = mcp.Credentials{}
	}

	var tools []string
	for _, t := range integration.Tools() {
		tools = append(tools, t.Name)
	}

	page := w.pageData(r, integration.Name(), "/integrations")
	data := pages.IntegrationDetailData{
		Name:        name,
		Enabled:     enabled,
		Healthy:     healthy,
		Credentials: creds,
		Tools:       tools,
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

	if err := w.services.Config.SetIntegration(name, ic); err != nil {
		http.Redirect(rw, r, "/integrations/"+name+"?error=Failed+to+save:+"+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, "/integrations/"+name+"?success=Configuration+saved", http.StatusSeeOther)
}

func (w *WebServer) handleHealthAPI(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(`{"status":"healthy"}`))
}

func (w *WebServer) handleSlackSetup(rw http.ResponseWriter, r *http.Request) {
	info := slackInt.GetTokenInfoForWeb()

	ic, exists := w.services.Config.GetIntegration("slack")
	hasOAuth := exists && ic.Credentials["client_id"] != "" && ic.Credentials["client_secret"] != ""

	var healthy bool
	if info.HasToken {
		integration, ok := w.services.Registry.Get("slack")
		if ok && exists {
			if err := integration.Configure(ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists {
		tokenSource = ic.Credentials["token_source"]
	}
	if tokenSource == "" && info.Source != "" {
		tokenSource = info.Source
	}

	page := w.pageData(r, "Slack Setup", "/integrations")
	data := pages.SlackSetupData{
		HasToken:       info.HasToken,
		HasCookie:      info.HasCookie,
		TokenStatus:    info.Status,
		TokenAge:       info.AgeHours,
		TokenSource:    tokenSource,
		CanAutoExtract: slackInt.CanExtractFromChrome(),
		ExtractSnippet: slackInt.ExtractionSnippet(),
		HasOAuth:       hasOAuth,
		Healthy:        healthy,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.SlackSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleSlackExtractChrome(rw http.ResponseWriter, r *http.Request) {
	result := slackInt.ExtractFromChromeForWeb()
	if !result.Success {
		http.Redirect(rw, r, "/integrations/slack/setup?error="+strings.ReplaceAll(result.Error, " ", "+"), http.StatusSeeOther)
		return
	}

	_, err := slackInt.SaveTokensForWeb(result.Token, result.Cookie)
	if err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Failed+to+save:+"+err.Error(), http.StatusSeeOther)
		return
	}

	// Also save to config so the integration picks it up.
	ic, _ := w.services.Config.GetIntegration("slack")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token"] = result.Token
	ic.Credentials["cookie"] = result.Cookie
	ic.Credentials["token_source"] = "chrome"
	_ = w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, "/integrations/slack/setup?result=Tokens+extracted+from+Chrome+and+saved", http.StatusSeeOther)
}

func (w *WebServer) handleGitHubSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("github")
	hasToken := exists && ic.Credentials["token"] != ""
	clientID := ""
	if exists {
		clientID = ic.Credentials["client_id"]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("github")
		if ok {
			if err := integration.Configure(ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials["token_source"] != "" {
		tokenSource = ic.Credentials["token_source"]
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
	if !exists || ic.Credentials["client_id"] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "GitHub OAuth client_id is not configured"})
		return
	}

	dcr, err := ghInt.StartOAuthFlow(ic.Credentials["client_id"])
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
	ic.Credentials["token_source"] = "oauth"
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
	ic.Credentials["token_source"] = "pat"
	_ = w.services.Config.SetIntegration("github", ic)

	http.Redirect(rw, r, "/integrations/github/setup?result=Token+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleLinearSetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("linear")
	hasToken := exists && ic.Credentials["api_key"] != ""
	hasOAuth := exists && ic.Credentials["client_id"] != "" && ic.Credentials["client_secret"] != ""

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("linear")
		if ok {
			if err := integration.Configure(ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials["token_source"] != "" {
		tokenSource = ic.Credentials["token_source"]
	}

	page := w.pageData(r, "Linear Setup", "/integrations")
	data := pages.LinearSetupData{
		HasToken:    hasToken,
		Healthy:     healthy,
		TokenSource: tokenSource,
		HasOAuth:    hasOAuth,
	}

	if flash := r.URL.Query().Get("result"); flash != "" {
		data.FlashResult = flash
	}
	if flash := r.URL.Query().Get("error"); flash != "" {
		data.FlashError = flash
	}

	pages.LinearSetup(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleLinearOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("linear")
	if !exists || ic.Credentials["client_id"] == "" || ic.Credentials["client_secret"] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Linear OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/linear/oauth/callback", w.port)
	result, err := linearInt.StartLinearOAuth(ic.Credentials["client_id"], ic.Credentials["client_secret"], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleLinearOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/linear/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := linearInt.HandleLinearCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/linear/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := linearInt.PollLinearOAuth()
	if result.Status != "complete" || result.Token == "" {
		http.Redirect(rw, r, "/integrations/linear/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("linear")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["api_key"] = result.Token
	ic.Credentials["token_source"] = "oauth"
	_ = w.services.Config.SetIntegration("linear", ic)

	http.Redirect(rw, r, "/integrations/linear/setup?result=Connected+to+Linear+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleLinearOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := linearInt.PollLinearOAuth()
	json.NewEncoder(rw).Encode(result)
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
	ic.Credentials["token_source"] = "api_key"
	_ = w.services.Config.SetIntegration("linear", ic)

	http.Redirect(rw, r, "/integrations/linear/setup?result=API+key+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleSentrySetup(rw http.ResponseWriter, r *http.Request) {
	ic, exists := w.services.Config.GetIntegration("sentry")
	hasToken := exists && ic.Credentials["auth_token"] != ""
	clientID := ""
	organization := ""
	if exists {
		clientID = ic.Credentials["client_id"]
		organization = ic.Credentials["organization"]
	}

	var healthy bool
	if hasToken {
		integration, ok := w.services.Registry.Get("sentry")
		if ok {
			if err := integration.Configure(ic.Credentials); err == nil {
				healthy = integration.Healthy(r.Context())
			}
		}
	}

	tokenSource := ""
	if exists && ic.Credentials["token_source"] != "" {
		tokenSource = ic.Credentials["token_source"]
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
	if !exists || ic.Credentials["client_id"] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Sentry OAuth client_id is not configured"})
		return
	}

	dcr, err := sentryInt.StartOAuthFlow(ic.Credentials["client_id"])
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
	ic.Credentials["token_source"] = "oauth"
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
	ic.Credentials["token_source"] = "token"
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

	_, err := slackInt.SaveTokensForWeb(token, cookie)
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
	ic.Credentials["token_source"] = "browser"
	_ = w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, "/integrations/slack/setup?result=Tokens+saved+successfully", http.StatusSeeOther)
}

func (w *WebServer) handleSlackOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	ic, exists := w.services.Config.GetIntegration("slack")
	if !exists || ic.Credentials["client_id"] == "" || ic.Credentials["client_secret"] == "" {
		json.NewEncoder(rw).Encode(map[string]string{"error": "Slack OAuth client_id/client_secret not configured"})
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/api/slack/oauth/callback", w.port)
	result, err := slackInt.StartSlackOAuth(ic.Credentials["client_id"], ic.Credentials["client_secret"], redirectURI)
	if err != nil {
		json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(rw).Encode(result)
}

func (w *WebServer) handleSlackOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "No authorization code received"
		}
		http.Redirect(rw, r, "/integrations/slack/setup?error="+strings.ReplaceAll(errMsg, " ", "+"), http.StatusSeeOther)
		return
	}

	if err := slackInt.HandleSlackCallback(code, state); err != nil {
		http.Redirect(rw, r, "/integrations/slack/setup?error="+strings.ReplaceAll(err.Error(), " ", "+"), http.StatusSeeOther)
		return
	}

	result := slackInt.PollSlackOAuth()
	if result.Status != "complete" || result.Token == "" {
		http.Redirect(rw, r, "/integrations/slack/setup?error=Failed+to+get+access+token", http.StatusSeeOther)
		return
	}

	ic, _ := w.services.Config.GetIntegration("slack")
	if ic == nil {
		ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
	}
	ic.Enabled = true
	ic.Credentials["token"] = result.Token
	ic.Credentials["cookie"] = ""
	ic.Credentials["token_source"] = "oauth"
	w.services.Config.SetIntegration("slack", ic)

	http.Redirect(rw, r, "/integrations/slack/setup?result=Connected+to+Slack+via+OAuth", http.StatusSeeOther)
}

func (w *WebServer) handleSlackOAuthPoll(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	result := slackInt.PollSlackOAuth()
	json.NewEncoder(rw).Encode(result)
}
