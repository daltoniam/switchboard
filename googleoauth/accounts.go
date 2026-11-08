package googleoauth

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

// Multi-account support for the Google Workspace adapters.
//
// An adapter's top-level credentials remain its "default" account, so
// existing single-account configs keep working unchanged. Additional Google
// accounts live in the integration's named identities:
//
//	"identities": {
//	  "work": {
//	    "credentials": {"access_token": "...", "refresh_token": "..."},
//	    "metadata":    {"label": "Work", "email": "me@work.example"}
//	  }
//	}
//
// Named accounts inherit client_id/client_secret from the top-level
// credentials (one OAuth client serves every account) unless they set their
// own. When at least one named account exists, every tool gains an optional
// "account" argument and a "<integration>_list_accounts" discovery tool is
// added. With no named accounts the tool catalog is unchanged.

// AccountParam is the optional tool argument that selects a named account.
const AccountParam = "account"

// DefaultAccountID names the account held in the integration's top-level
// credentials. It is reserved and cannot be used as an identity name.
const DefaultAccountID = "default"

// IdentityCredentialKeys are the secret fields edited per named account.
func IdentityCredentialKeys() []string { return []string{"access_token", "refresh_token"} }

// IdentityMetadataKeys are the non-secret fields shown per named account.
func IdentityMetadataKeys() []string { return []string{"label", "email"} }

// Account is one named Google account. Its access token is rotated in place
// on refresh and persisted back to the identity's config entry.
type Account struct {
	id    string
	owner *AccountSet

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	metadata     map[string]string
}

// ID returns the account's identity name.
func (a *Account) ID() string { return a.id }

// AccessToken returns the account's current access token.
func (a *Account) AccessToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accessToken
}

// Refresh exchanges the refresh token for a new access token when stale is
// still the current token. When another request already rotated the token,
// the current one is returned without calling Google.
func (a *Account) Refresh(ctx context.Context, stale string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accessToken != stale {
		return a.accessToken, nil
	}
	if a.refreshToken == "" || a.clientID == "" || a.clientSecret == "" {
		return "", fmt.Errorf("account %q: refresh_token and OAuth client are required to refresh", a.id)
	}
	tok, err := RefreshAccessToken(ctx, a.clientID, a.clientSecret, a.refreshToken)
	if err != nil {
		return "", err
	}
	a.accessToken = tok
	if a.owner != nil {
		a.owner.persist(a.id, tok)
	}
	return tok, nil
}

type accountCtxKey struct{}

// WithAccount returns ctx carrying the selected account.
func WithAccount(ctx context.Context, a *Account) context.Context {
	return context.WithValue(ctx, accountCtxKey{}, a)
}

// AccountFromContext returns the account selected for this call, or nil when
// the call uses the default (top-level) account.
func AccountFromContext(ctx context.Context) *Account {
	a, _ := ctx.Value(accountCtxKey{}).(*Account)
	return a
}

// AccountSet holds an adapter's named Google accounts. A nil *AccountSet
// behaves as an empty set so adapters built in tests without one still work.
type AccountSet struct {
	integration string
	base        []mcp.ToolDefinition

	mu        sync.RWMutex
	order     []string
	accounts  map[string]*Account
	tools     []mcp.ToolDefinition
	configSvc mcp.ConfigService
}

// NewAccountSet creates an empty set for integration, whose static tool
// definitions are base.
func NewAccountSet(integration string, base []mcp.ToolDefinition) *AccountSet {
	return &AccountSet{integration: integration, base: base, tools: base}
}

// SetConfigService lets refreshed tokens be persisted to the config file.
func (s *AccountSet) SetConfigService(svc mcp.ConfigService) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.configSvc = svc
	s.mu.Unlock()
}

// ListToolName is the discovery tool exposed while named accounts exist.
func (s *AccountSet) ListToolName() mcp.ToolName {
	return mcp.ToolName(s.integration + "_list_accounts")
}

// Configure replaces the named accounts. clientID/clientSecret are the
// top-level OAuth client, inherited by accounts that do not set their own.
// On error the previous accounts are kept.
func (s *AccountSet) Configure(identities map[string]mcp.IntegrationIdentity, clientID, clientSecret string) error {
	if s == nil {
		return nil
	}
	ids := make([]string, 0, len(identities))
	next := make(map[string]*Account, len(identities))
	for rawID, ident := range identities {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return fmt.Errorf("%s: account id cannot be empty", s.integration)
		}
		// The ID must equal its config key: refreshed tokens are persisted
		// under the ID, and trimming could merge two keys into one account.
		if id != rawID {
			return fmt.Errorf("%s: account id %q must not have leading or trailing spaces", s.integration, rawID)
		}
		if id == DefaultAccountID {
			return fmt.Errorf("%s: account id %q is reserved for the top-level credentials", s.integration, DefaultAccountID)
		}
		tok := strings.TrimSpace(ident.Credentials["access_token"])
		if tok == "" {
			return fmt.Errorf("%s: account %q: access_token is required", s.integration, id)
		}
		a := &Account{
			id:           id,
			owner:        s,
			accessToken:  tok,
			refreshToken: strings.TrimSpace(ident.Credentials["refresh_token"]),
			clientID:     clientID,
			clientSecret: clientSecret,
			metadata:     copyStrings(ident.Metadata),
		}
		if v := strings.TrimSpace(ident.Credentials[mcp.CredKeyClientID]); v != "" {
			a.clientID = v
		}
		if v := strings.TrimSpace(ident.Credentials[mcp.CredKeyClientSecret]); v != "" {
			a.clientSecret = v
		}
		ids = append(ids, id)
		next[id] = a
	}
	sort.Strings(ids)

	tools := s.base
	if len(ids) > 0 {
		tools = s.decorate()
	}

	s.mu.Lock()
	s.order = ids
	s.accounts = next
	s.tools = tools
	s.mu.Unlock()
	return nil
}

// decorate returns copies of the base tools with the optional account
// argument, plus the list-accounts discovery tool.
func (s *AccountSet) decorate() []mcp.ToolDefinition {
	listName := s.ListToolName()
	desc := fmt.Sprintf("Google account to use (from %s). Omit for the %q account.", listName, DefaultAccountID)
	out := make([]mcp.ToolDefinition, 0, len(s.base)+1)
	for _, t := range s.base {
		params := make(map[string]string, len(t.Parameters)+1)
		for k, v := range t.Parameters {
			params[k] = v
		}
		params[AccountParam] = desc
		t.Parameters = params
		out = append(out, t)
	}
	display := s.integration
	if svc, ok := serviceByName[s.integration]; ok {
		display = svc.DisplayName
	}
	out = append(out, mcp.ToolDefinition{
		Name: listName,
		Description: fmt.Sprintf("List the Google accounts configured for %s: account IDs, labels and emails (never tokens). "+
			"Use it when the user has more than one Google account, then pass the ID as `account` to any %s_* tool.",
			display, s.integration),
		Parameters: map[string]string{},
	})
	return out
}

// Len returns the number of named (non-default) accounts.
func (s *AccountSet) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.order)
}

// Tools returns the adapter's tool definitions for the current accounts.
func (s *AccountSet) Tools() []mcp.ToolDefinition {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tools
}

func (s *AccountSet) lookup(id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.accounts[id]; ok {
		return a, nil
	}
	avail := append([]string{DefaultAccountID}, s.order...)
	return nil, fmt.Errorf("unknown account %q; available: %s", id, strings.Join(avail, ", "))
}

// Route prepares a tool call. It answers the list-accounts tool itself
// (returning a non-nil result), and otherwise strips the account argument
// and stores the selected account in the returned context. A nil result
// means the adapter should dispatch the call normally.
func (s *AccountSet) Route(ctx context.Context, toolName mcp.ToolName, args map[string]any) (context.Context, map[string]any, *mcp.ToolResult, error) {
	if s == nil {
		return ctx, args, nil, nil
	}
	if toolName == s.ListToolName() && s.Len() > 0 {
		res, err := s.list()
		return ctx, args, res, err
	}
	if _, ok := args[AccountParam]; !ok {
		return ctx, args, nil, nil
	}
	r := mcp.NewArgs(args)
	id := strings.TrimSpace(r.Str(AccountParam))
	if err := r.Err(); err != nil {
		res, rerr := mcp.ErrResult(err)
		return ctx, args, res, rerr
	}
	fwd := make(map[string]any, len(args))
	for k, v := range args {
		if k != AccountParam {
			fwd[k] = v
		}
	}
	if id == "" || id == DefaultAccountID {
		return ctx, fwd, nil, nil
	}
	a, err := s.lookup(id)
	if err != nil {
		res, rerr := mcp.ErrResult(err)
		return ctx, args, res, rerr
	}
	return WithAccount(ctx, a), fwd, nil, nil
}

func (s *AccountSet) list() (*mcp.ToolResult, error) {
	type info struct {
		ID       string            `json:"id"`
		Default  bool              `json:"default,omitempty"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	s.mu.RLock()
	out := []info{{ID: DefaultAccountID, Default: true}}
	for _, id := range s.order {
		out = append(out, info{ID: id, Metadata: copyStrings(s.accounts[id].metadata)})
	}
	s.mu.RUnlock()
	return mcp.JSONResult(map[string]any{
		"default_account": DefaultAccountID,
		"accounts":        out,
	})
}

// persist writes a refreshed access token back to the named identity.
func (s *AccountSet) persist(id, token string) {
	s.mu.RLock()
	svc := s.configSvc
	s.mu.RUnlock()
	if svc == nil {
		return
	}
	update := func(ic *mcp.IntegrationConfig) error {
		ident, ok := ic.Identities[id]
		if !ok {
			return fmt.Errorf("account %q no longer configured", id)
		}
		creds := make(mcp.Credentials, len(ident.Credentials)+1)
		for k, v := range ident.Credentials {
			creds[k] = v
		}
		creds["access_token"] = token
		ident.Credentials = creds
		ic.Identities[id] = ident
		return nil
	}
	if u, ok := svc.(mcp.IntegrationConfigUpdater); ok {
		_ = u.UpdateIntegration(s.integration, update)
		return
	}
	ic, ok := svc.GetIntegration(s.integration)
	if !ok || ic == nil {
		return
	}
	if update(ic) == nil {
		_ = svc.SetIntegration(s.integration, ic)
	}
}

func copyStrings(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
