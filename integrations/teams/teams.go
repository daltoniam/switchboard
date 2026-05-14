// Package teams provides a Microsoft Teams integration that impersonates the
// authenticated user via OAuth (device code flow). Tokens are obtained against
// a public Microsoft Entra client (Azure CLI by default) and persisted to
// ~/.teams-mcp-tokens.json. Calls go to Microsoft Graph (graph.microsoft.com).
//
// Auth model parallels the Slack adapter's "act as the real user" guarantee,
// but the underlying mechanism is OAuth + refresh tokens rather than browser
// cookie extraction. A future phase can add Electron/Edge token extraction for
// zero-prompt setup; Phase 1 ships device-code OAuth only.
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*teamsIntegration)(nil)
	_ mcp.FieldCompactionIntegration = (*teamsIntegration)(nil)
	_ mcp.PlainTextCredentials       = (*teamsIntegration)(nil)
	_ mcp.OptionalCredentials        = (*teamsIntegration)(nil)
	_ mcp.PlaceholderHints           = (*teamsIntegration)(nil)
)

const (
	// Default Microsoft Graph endpoint. Override via credentials["graph_base_url"]
	// for sovereign clouds (GCC-High, DOD, China).
	defaultGraphBaseURL = "https://graph.microsoft.com/v1.0"

	// Default Microsoft Entra (Azure AD) authorize/token endpoint host.
	// Override via credentials["login_base_url"] for sovereign clouds.
	defaultLoginBaseURL = "https://login.microsoftonline.com"

	// Default public client_id (Microsoft Azure CLI). Pre-consented in nearly
	// every Entra tenant for delegated Microsoft Graph access. Override via
	// credentials["client_id"] when a tenant requires a customer-registered
	// app instead.
	defaultClientID = "04b07795-8ddb-461a-bbee-02f9e1bf7b46"

	// Default OAuth scopes — chosen to require NO admin consent in most tenants.
	// "ChannelMessage.Read.All" is deliberately excluded because it requires
	// admin consent; channel message reads should be added per-tenant when
	// available. Chat reads (1:1, group, meeting chats) work without admin
	// consent and cover the primary "impersonate the user" use case.
	defaultScopes = "offline_access User.Read User.ReadBasic.All Chat.ReadWrite Team.ReadBasic.All Channel.ReadBasic.All ChannelMessage.Send Presence.Read"

	// Refresh tokens are typically valid 90 days with rolling renewal; access
	// tokens are typically valid 60-90 minutes. We refresh proactively when
	// the cached access token is within this window of expiry.
	refreshSkew = 2 * time.Minute

	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

type teamsIntegration struct {
	mu            sync.RWMutex
	store         *tokenStore
	httpClient    *http.Client
	clientID      string
	clientSecret  string
	loginBaseURL  string
	graphBaseURL  string
	scopes        string
	defaultTenant string // configured default; empty means "use whatever is in the store"
	stopBg        chan struct{}
}

// New constructs a Teams integration.
func New() mcp.Integration {
	return &teamsIntegration{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *teamsIntegration) Name() string { return "teams" }

func (t *teamsIntegration) PlainTextKeys() []string {
	return []string{"tenant_id", "client_id", "graph_base_url", "login_base_url", "scopes"}
}

func (t *teamsIntegration) OptionalKeys() []string {
	return []string{"tenant_id", "client_id", "client_secret", "graph_base_url", "login_base_url", "scopes", "access_token", "refresh_token", "expires_at"}
}

func (t *teamsIntegration) Placeholders() map[string]string {
	return map[string]string{
		"tenant_id":      "common (multi-tenant) or a specific tenant GUID/domain",
		"client_id":      "Public client ID (defaults to Azure CLI: " + defaultClientID + ")",
		"graph_base_url": defaultGraphBaseURL,
		"login_base_url": defaultLoginBaseURL,
		"scopes":         "Space-separated OAuth scopes (default omits admin-consent-only scopes)",
		"access_token":   "Filled by the OAuth flow — usually leave blank",
		"refresh_token":  "Filled by the OAuth flow — usually leave blank",
	}
}

func (t *teamsIntegration) Configure(ctx context.Context, creds mcp.Credentials) error {
	t.mu.Lock()

	t.clientID = strings.TrimSpace(creds["client_id"])
	if t.clientID == "" {
		t.clientID = defaultClientID
	}
	t.clientSecret = strings.TrimSpace(creds["client_secret"])
	t.loginBaseURL = strings.TrimRight(strings.TrimSpace(creds["login_base_url"]), "/")
	if t.loginBaseURL == "" {
		t.loginBaseURL = defaultLoginBaseURL
	}
	t.graphBaseURL = strings.TrimRight(strings.TrimSpace(creds["graph_base_url"]), "/")
	if t.graphBaseURL == "" {
		t.graphBaseURL = defaultGraphBaseURL
	}
	t.scopes = strings.TrimSpace(creds["scopes"])
	if t.scopes == "" {
		t.scopes = defaultScopes
	}
	t.defaultTenant = strings.TrimSpace(creds["tenant_id"])

	t.store = newTokenStore()
	t.store.loadFromFile()

	// If credentials carry an explicit token (typical hosted/OAuth-in-config
	// deployments), inject it as a tenant entry. The tenant key in that case
	// falls back to "_config" when not provided.
	if at := strings.TrimSpace(creds["access_token"]); at != "" {
		tid := t.defaultTenant
		if tid == "" {
			tid = "_config"
		}
		t.store.upsert(&tenant{
			TenantID:     tid,
			AccessToken:  at,
			RefreshToken: strings.TrimSpace(creds["refresh_token"]),
			Source:       "config",
		})
		if t.defaultTenant != "" {
			t.store.setDefault(t.defaultTenant)
		}
	}

	// Manage the background-refresh stop channel under t.mu so that a
	// concurrent Configure cannot race with the goroutine's select.
	if t.stopBg != nil {
		close(t.stopBg)
	}
	t.stopBg = make(chan struct{})
	stopCh := t.stopBg

	t.mu.Unlock()

	// Resolve user identity for each loaded tenant. Failures are logged but
	// non-fatal — tokens may legitimately be expired and refreshable.
	t.resolveTenantIdentities(ctx)

	// Background refresh: skips for config-injected tokens since they're
	// managed externally. Detach from the request-scoped ctx — the goroutine
	// outlives Configure. WithoutCancel preserves values but drops cancellation.
	bgCtx := context.WithoutCancel(ctx)
	go t.backgroundRefresh(bgCtx, stopCh)

	return nil
}

func (t *teamsIntegration) Tools() []mcp.ToolDefinition { return tools }

func (t *teamsIntegration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (t *teamsIntegration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, t, args)
}

func (t *teamsIntegration) Healthy(ctx context.Context) bool {
	tn := t.activeTenant()
	if tn == nil {
		return false
	}
	_, err := t.graphGet(ctx, tn.TenantID, "/me")
	return err == nil
}

// activeTenant returns the configured default tenant, or the first stored
// tenant when no default is set. Returns nil when no tenant is configured.
func (t *teamsIntegration) activeTenant() *tenant {
	t.mu.RLock()
	def := t.defaultTenant
	t.mu.RUnlock()
	if def != "" {
		if tn := t.store.get(def); tn != nil {
			return tn
		}
	}
	return t.store.getDefault()
}

// tenantFromArgs picks a tenant by explicit args["tenant_id"], falling back to
// the active tenant. Returns a wrapped error suitable for ErrResult when the
// tenant is missing.
func (t *teamsIntegration) tenantFromArgs(args map[string]any) (*tenant, error) {
	tid, _ := mcp.ArgStr(args, "tenant_id")
	if tid != "" {
		if tn := t.store.get(tid); tn != nil {
			return tn, nil
		}
		return nil, fmt.Errorf("unknown tenant: %s — use teams_list_tenants to see configured tenants, or teams_login to add one", tid)
	}
	if tn := t.activeTenant(); tn != nil {
		return tn, nil
	}
	return nil, fmt.Errorf("no teams tenant configured — run teams_login to authenticate")
}

func (t *teamsIntegration) resolveTenantIdentities(ctx context.Context) {
	for _, tn := range t.store.all() {
		if tn.AccessToken == "" {
			continue
		}
		raw, err := t.graphGet(ctx, tn.TenantID, "/me")
		if err != nil {
			log.Printf("teams: identity probe failed for tenant %s: %v", tn.TenantID, err)
			continue
		}
		var me struct {
			ID                string `json:"id"`
			DisplayName       string `json:"displayName"`
			UserPrincipalName string `json:"userPrincipalName"`
			TenantID          string `json:"tenantId"`
		}
		if err := json.Unmarshal(raw, &me); err != nil {
			continue
		}
		// Some tenants strip tenantId from /me; rely on cached value otherwise.
		if me.UserPrincipalName != "" && tn.UserUPN == "" {
			tn.UserUPN = me.UserPrincipalName
		}
		if me.DisplayName != "" && tn.UserDisplay == "" {
			tn.UserDisplay = me.DisplayName
		}
		if me.ID != "" && tn.UserOID == "" {
			tn.UserOID = me.ID
		}
		t.store.upsert(tn)
	}
	_ = t.store.saveToFile()
}

// --- Graph HTTP helpers ---

// graphGet issues a GET against the Graph API for the given tenant.
func (t *teamsIntegration) graphGet(ctx context.Context, tenantID, path string) (json.RawMessage, error) {
	return t.graphRequest(ctx, tenantID, http.MethodGet, path, nil)
}

// graphPost issues a POST against the Graph API for the given tenant.
func (t *teamsIntegration) graphPost(ctx context.Context, tenantID, path string, body any) (json.RawMessage, error) {
	return t.graphRequest(ctx, tenantID, http.MethodPost, path, body)
}

// graphDelete issues a DELETE against the Graph API for the given tenant.
func (t *teamsIntegration) graphDelete(ctx context.Context, tenantID, path string) (json.RawMessage, error) {
	return t.graphRequest(ctx, tenantID, http.MethodDelete, path, nil)
}

func (t *teamsIntegration) graphRequest(ctx context.Context, tenantID, method, path string, body any) (json.RawMessage, error) {
	return t.graphRequestInner(ctx, tenantID, method, path, body, nil, true)
}

// graphGetWithHeaders issues a GET with extra headers (e.g. ConsistencyLevel:
// eventual for $search) and returns the response wrapped in a ToolResult.
func (t *teamsIntegration) graphGetWithHeaders(ctx context.Context, tenantID, path string, headers http.Header) (*mcp.ToolResult, error) {
	data, err := t.graphRequestInner(ctx, tenantID, http.MethodGet, path, nil, headers, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func (t *teamsIntegration) graphRequestInner(ctx context.Context, tenantID, method, path string, body any, extraHeaders http.Header, canRetry bool) (json.RawMessage, error) {
	tn := t.store.get(tenantID)
	if tn == nil {
		return nil, fmt.Errorf("unknown tenant: %s", tenantID)
	}

	// Proactive refresh: avoid a guaranteed 401 round-trip when we already
	// know the access token is expired (or about to be).
	if canRetry && t.shouldProactivelyRefresh(tn) {
		if err := t.refreshTenant(ctx, tn); err == nil {
			tn = t.store.get(tenantID)
		}
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	endpoint := t.graphBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tn.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vv := range extraHeaders {
		for _, v := range vv {
			req.Header.Set(k, v)
		}
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized && canRetry && tn.RefreshToken != "" {
		if rerr := t.refreshTenant(ctx, tn); rerr == nil {
			return t.graphRequestInner(ctx, tenantID, method, path, body, extraHeaders, false)
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("teams graph API error (%d): %s", resp.StatusCode, string(data)),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("teams graph API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// shouldProactivelyRefresh reports whether the access token for tn is
// expired-or-about-to-expire and a refresh token is available.
func (t *teamsIntegration) shouldProactivelyRefresh(tn *tenant) bool {
	if tn.RefreshToken == "" {
		return false
	}
	if tn.ExpiresAt.IsZero() {
		return false
	}
	return time.Until(tn.ExpiresAt) <= refreshSkew
}

func (t *teamsIntegration) backgroundRefresh(ctx context.Context, stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.refreshAllExpiring(ctx)
		case <-stop:
			return
		}
	}
}

func (t *teamsIntegration) refreshAllExpiring(ctx context.Context) {
	for _, tn := range t.store.all() {
		if !t.shouldProactivelyRefresh(tn) {
			continue
		}
		if err := t.refreshTenant(ctx, tn); err != nil {
			log.Printf("teams: background refresh failed for tenant %s: %v", tn.TenantID, err)
		}
	}
}

// --- handler dispatch ---

type handlerFunc func(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Auth + tenants
	mcp.ToolName("teams_login"):          teamsLogin,
	mcp.ToolName("teams_login_poll"):     teamsLoginPoll,
	mcp.ToolName("teams_token_status"):   teamsTokenStatus,
	mcp.ToolName("teams_refresh_tokens"): teamsRefreshTokens,
	mcp.ToolName("teams_list_tenants"):   teamsListTenants,
	mcp.ToolName("teams_remove_tenant"):  teamsRemoveTenant,
	mcp.ToolName("teams_set_default"):    teamsSetDefault,
	mcp.ToolName("teams_get_me"):         teamsGetMe,

	// Chats
	mcp.ToolName("teams_list_chats"):         listChats,
	mcp.ToolName("teams_get_chat"):           getChat,
	mcp.ToolName("teams_list_chat_messages"): listChatMessages,
	mcp.ToolName("teams_get_chat_message"):   getChatMessage,
	mcp.ToolName("teams_send_chat_message"):  sendChatMessage,
	mcp.ToolName("teams_list_chat_members"):  listChatMembers,

	// Teams + channels
	mcp.ToolName("teams_list_joined_teams"):        listJoinedTeams,
	mcp.ToolName("teams_list_channels"):            listChannels,
	mcp.ToolName("teams_get_channel"):              getChannel,
	mcp.ToolName("teams_list_channel_messages"):    listChannelMessages,
	mcp.ToolName("teams_get_channel_message"):      getChannelMessage,
	mcp.ToolName("teams_list_message_replies"):     listMessageReplies,
	mcp.ToolName("teams_send_channel_message"):     sendChannelMessage,
	mcp.ToolName("teams_reply_to_channel_message"): replyToChannelMessage,

	// Users
	mcp.ToolName("teams_list_users"):   listUsers,
	mcp.ToolName("teams_get_user"):     getUser,
	mcp.ToolName("teams_search_users"): searchUsers,
	mcp.ToolName("teams_get_presence"): getPresence,
}
