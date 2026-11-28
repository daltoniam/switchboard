package onepassword

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

// Several 1Password accounts can be configured as named identities. The
// top-level credentials stay the "default" identity, so single-account
// configs keep working unchanged:
//
//	"credentials": {"account_name": "My Team"},
//	"identities": {
//	  "work": {
//	    "credentials": {"service_account_token": "ops_..."},
//	    "metadata":    {"label": "Work vaults"}
//	  }
//	}

const (
	identityParam      = "identity_id"
	defaultIdentityID  = "default"
	listIdentitiesTool = mcp.ToolName("onepassword_list_identities")
)

var (
	_ mcp.MultiIdentityIntegration = (*onepassword)(nil)
	_ mcp.IdentityConfigHints      = (*onepassword)(nil)
)

// identity is one configured sign-in.
type identity struct {
	id    string
	label string
	auth  auth
}

func (*onepassword) IdentityCredentialKeys() []string {
	return []string{"account_name", "service_account_token"}
}

func (*onepassword) IdentityMetadataKeys() []string { return []string{"label"} }

// ConfigureIdentities replaces the named identities and checks the whole set,
// including the default identity from Configure. On error the previous named
// identities are kept, so a rejected addition does not break working ones.
func (a *onepassword) ConfigureIdentities(_ context.Context, identities map[string]mcp.IntegrationIdentity) error {
	named, err := a.parseIdentities(identities)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		return err
	}
	all := named
	if a.top != (auth{}) {
		all = append([]identity{{id: defaultIdentityID, auth: a.top}}, named...)
	}
	if len(all) == 0 {
		return fmt.Errorf("onepassword: set account_name to sign in through the 1Password desktop app, or service_account_token for a service account")
	}
	if err := checkDesktopLimit(all); err != nil {
		return err
	}
	a.named = named
	a.pruneSessions()
	return nil
}

func (a *onepassword) parseIdentities(identities map[string]mcp.IntegrationIdentity) ([]identity, error) {
	ids := make([]string, 0, len(identities))
	for id := range identities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	named := make([]identity, 0, len(ids))
	for _, id := range ids {
		switch trimmed := strings.TrimSpace(id); {
		case trimmed == "":
			return nil, fmt.Errorf("onepassword: identity id cannot be empty")
		case trimmed != id:
			return nil, fmt.Errorf("onepassword: identity id %q must not have leading or trailing spaces", id)
		case id == defaultIdentityID:
			return nil, fmt.Errorf("onepassword: identity id %q is reserved for the top-level credentials", defaultIdentityID)
		}
		ident := identities[id]
		au, err := a.parseAuth(ident.Credentials)
		if err == nil && au == (auth{}) {
			err = fmt.Errorf("set account_name to sign in through the 1Password desktop app, or service_account_token for a service account")
		}
		if err != nil {
			return nil, fmt.Errorf("onepassword: identity %q: %w", id, err)
		}
		named = append(named, identity{id: id, label: strings.TrimSpace(ident.Metadata["label"]), auth: au})
	}
	return named, nil
}

// identitiesLocked returns the default identity first, then the named ones
// by id. The caller holds a.mu.
func (a *onepassword) identitiesLocked() []identity {
	all := make([]identity, 0, len(a.named)+1)
	if a.top != (auth{}) {
		all = append(all, identity{id: defaultIdentityID, auth: a.top})
	}
	return append(all, a.named...)
}

// resolveLocked finds the identity a call selects. An empty id or "default"
// selects the top-level credentials, or the first named identity when there
// are none. The caller holds a.mu.
func (a *onepassword) resolveLocked(id string) (identity, error) {
	all := a.identitiesLocked()
	if len(all) == 0 {
		return identity{}, fmt.Errorf("onepassword: integration is not configured")
	}
	if id == "" || id == defaultIdentityID {
		return all[0], nil
	}
	ids := make([]string, 0, len(all))
	for _, ident := range all {
		if ident.id == id {
			return ident, nil
		}
		ids = append(ids, ident.id)
	}
	if ids[0] != defaultIdentityID {
		ids = append([]string{defaultIdentityID}, ids...)
	}
	return identity{}, fmt.Errorf("onepassword: unknown identity %q; available: %s (see %s)", id, strings.Join(ids, ", "), listIdentitiesTool)
}

// identityError names the identity a failure belongs to when several are
// configured, so the caller can tell which account to fix.
type identityError struct {
	id  string
	err error
}

func (e identityError) Error() string {
	return fmt.Sprintf("onepassword: identity %q: %s", e.id, strings.TrimPrefix(e.err.Error(), "onepassword: "))
}

func (e identityError) Unwrap() error { return e.err }

// identityArg removes the identity argument from args and returns its value.
func identityArg(args map[string]any) (string, map[string]any, error) {
	if _, ok := args[identityParam]; !ok {
		return "", args, nil
	}
	r := mcp.NewArgs(args)
	id := strings.TrimSpace(r.Str(identityParam))
	if err := r.Err(); err != nil {
		return "", nil, err
	}
	rest := make(map[string]any, len(args)-1)
	for k, v := range args {
		if k != identityParam {
			rest[k] = v
		}
	}
	return id, rest, nil
}

type identityInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Method  string `json:"method"`
	Account string `json:"account"`
	Default bool   `json:"default"`
}

// listIdentities answers without signing in, so it never shows a 1Password
// prompt. Tokens are never returned; desktop account names are not secrets.
func (a *onepassword) listIdentities() (*mcp.ToolResult, error) {
	a.mu.Lock()
	all := a.identitiesLocked()
	a.mu.Unlock()
	if len(all) == 0 {
		return mcp.ErrResult(fmt.Errorf("onepassword: integration is not configured"))
	}
	out := make([]identityInfo, 0, len(all))
	for i, ident := range all {
		info := identityInfo{ID: ident.id, Label: ident.label, Method: "service_account", Default: i == 0}
		if ident.auth.account != "" {
			info.Method, info.Account = "desktop_app", ident.auth.account
		}
		out = append(out, info)
	}
	return mcp.JSONResult(map[string]any{"default_identity": all[0].id, "identities": out})
}

// The 1Password SDK (v0.4.1) loads the desktop app's library once per
// process and binds it to the first account name it is given
// (internal.GetSharedLibCore), so every later desktop client silently talks
// to that account. Service account clients use a separate core per client
// and are unaffected. checkDesktopLimit and desktopBinding are the only
// places that enforce this; remove both once the SDK binds the account per
// client.

const desktopLimitReason = "the 1Password SDK keeps using the first account it signs in to through the desktop app, so a second desktop app account would silently read and write the first account's vaults"

// checkDesktopLimit allows at most one identity that signs in through the
// desktop app.
func checkDesktopLimit(all []identity) error {
	var first string
	for _, ident := range all {
		if ident.auth.account == "" {
			continue
		}
		if first != "" {
			return fmt.Errorf("onepassword: only one identity can sign in through the 1Password desktop app, but identities %q and %q both do: %s. Add other accounts with a service account token instead", first, ident.id, desktopLimitReason)
		}
		first = ident.id
	}
	return nil
}

// desktopBinding remembers the desktop app account this process signed in
// to, so changing it without a restart fails instead of using the old one.
type desktopBinding struct {
	mu      sync.Mutex
	account string
}

var processDesktop desktopBinding

func (b *desktopBinding) bind(account string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.account == "" {
		b.account = account
		return nil
	}
	if b.account != account {
		return fmt.Errorf("this Switchboard process already signed in to the 1Password desktop app as %q, and %s. Restart Switchboard to sign in as %q", b.account, desktopLimitReason, account)
	}
	return nil
}
