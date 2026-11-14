package remotemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Credential keys for a remote MCP OAuth login. Adapters that support
// renewal store all of them; others use only CredKeyAccessToken.
const (
	CredKeyAccessToken  = "mcp_access_token"
	CredKeyRefreshToken = "mcp_refresh_token"
	CredKeyTokenExpiry  = "mcp_token_expiry" // RFC 3339, UTC
	CredKeyTokenIssued  = "mcp_token_issued" // RFC 3339, UTC
	CredKeyClientID     = "mcp_client_id"
	CredKeyClientSecret = "mcp_client_secret"
)

// RenewalLead is how long before expiry an access token is renewed. For a
// token that lives less than twice as long, renewal starts halfway through its
// life instead, so a freshly issued short-lived token is not renewed again.
const RenewalLead = time.Hour

// RenewalTarget names the login to renew: the integration's config entry,
// the MCP origin used for OAuth discovery, and the RFC 8707 resource sent at
// sign-in ("" for none).
type RenewalTarget struct {
	Integration string
	ServerURL   string
	Resource    string
}

// TokenRenewal renews a remote MCP OAuth access token shortly before it
// expires. Some servers (Higgsfield) issue single-use refresh tokens, and
// reusing a spent one revokes the whole login, so every renewal runs under the
// config store's cross-process lock, starts from the newest saved tokens, and
// saves the rotated tokens before releasing the lock. The zero value never
// renews; give it a store with SetConfigService.
type TokenRenewal struct {
	mu    sync.Mutex // one renewal at a time in this process
	store mcp.SharedIntegrationConfigUpdater
	now   func() time.Time
}

// SetConfigService enables renewal when the config service can update an
// integration under a cross-process lock. Without one, tokens are never
// renewed, because a rotated refresh token that cannot be saved is lost.
func (r *TokenRenewal) SetConfigService(svc mcp.ConfigService) {
	store, _ := svc.(mcp.SharedIntegrationConfigUpdater)
	r.mu.Lock()
	r.store = store
	r.mu.Unlock()
}

func (r *TokenRenewal) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Fresh returns credentials whose access token has more than RenewalLead
// left, renewing and saving them if needed. renewed reports whether the
// access token differs from the one in creds, in which case the caller must
// reconfigure with the returned credentials. Credentials without a refresh
// token or a known expiry are returned unchanged.
func (r *TokenRenewal) Fresh(ctx context.Context, target RenewalTarget, creds mcp.Credentials) (fresh mcp.Credentials, renewed bool, err error) {
	if !renewalDue(creds, r.clock()) {
		return creds, false, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store == nil {
		return creds, false, nil
	}
	err = r.store.UpdateIntegrationShared(ctx, target.Integration, func(ic *mcp.IntegrationConfig) error {
		// Another process may have renewed already; its tokens are the
		// newest, and its refresh token is the only one still valid.
		if renewalDue(ic.Credentials, r.clock()) {
			grant, err := refreshGrant(ctx, target, ic.Credentials, r.clock())
			if err != nil {
				return err
			}
			ic.Credentials[CredKeyAccessToken] = grant.accessToken
			if grant.refreshToken != "" {
				ic.Credentials[CredKeyRefreshToken] = grant.refreshToken
			}
			ic.Credentials[CredKeyTokenExpiry] = formatExpiry(grant.expiry)
			ic.Credentials[CredKeyTokenIssued] = formatExpiry(r.clock())
		}
		fresh = maps.Clone(ic.Credentials)
		return nil
	})
	if err != nil {
		return creds, false, fmt.Errorf("%s: renew access token: %w; sign in again from the Switchboard setup page if this persists", target.Integration, err)
	}
	return fresh, fresh[CredKeyAccessToken] != creds[CredKeyAccessToken], nil
}

// renewalDue reports whether creds can be renewed and are inside their
// renewal window: RenewalLead before expiry, or the second half of the
// token's life when that is shorter and the issue time is known.
func renewalDue(creds mcp.Credentials, now time.Time) bool {
	if creds[CredKeyRefreshToken] == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, creds[CredKeyTokenExpiry])
	if err != nil {
		return false
	}
	lead := RenewalLead
	if issued, err := time.Parse(time.RFC3339, creds[CredKeyTokenIssued]); err == nil && expiry.After(issued) {
		lead = min(lead, expiry.Sub(issued)/2)
	}
	return !now.Add(lead).Before(expiry)
}

// TokenExpired reports whether creds carry an expiry that has passed. An
// unknown expiry counts as not expired.
func TokenExpired(creds mcp.Credentials, now time.Time) bool {
	expiry, err := time.Parse(time.RFC3339, creds[CredKeyTokenExpiry])
	return err == nil && !now.Before(expiry)
}

type tokenGrant struct {
	accessToken  string
	refreshToken string
	expiry       time.Time
}

func refreshGrant(ctx context.Context, target RenewalTarget, creds mcp.Credentials, now time.Time) (tokenGrant, error) {
	clientID := creds[CredKeyClientID]
	if clientID == "" {
		return tokenGrant{}, fmt.Errorf("no OAuth client ID saved with the login")
	}
	meta, err := discoverOAuth(target.ServerURL)
	if err != nil {
		return tokenGrant{}, err
	}
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {creds[CredKeyRefreshToken]},
		"client_id":     {clientID},
	}
	if secret := creds[CredKeyClientSecret]; secret != "" {
		data.Set("client_secret", secret)
	}
	if target.Resource != "" {
		data.Set("resource", target.Resource)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return tokenGrant{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req) // #nosec G107 -- token endpoint from OAuth metadata discovery
	if err != nil {
		return tokenGrant{}, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return tokenGrant{}, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tokenGrant{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, oauthErrorText(body))
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return tokenGrant{}, fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return tokenGrant{}, fmt.Errorf("no access_token in token response")
	}
	return tokenGrant{
		accessToken:  tokenResp.AccessToken,
		refreshToken: tokenResp.RefreshToken,
		expiry:       expiryAfter(now, tokenResp.ExpiresIn),
	}, nil
}

// oauthErrorText keeps the RFC 6749 error fields of a failed token response
// and drops anything else, so no token can reach an error message or log.
func oauthErrorText(body []byte) string {
	var oauthErr struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(body, &oauthErr) != nil || oauthErr.Error == "" {
		return "unexpected response"
	}
	if oauthErr.Description != "" {
		return oauthErr.Error + ": " + oauthErr.Description
	}
	return oauthErr.Error
}

// expiryAfter turns an expires_in value into an absolute time; zero means the
// server did not say.
func expiryAfter(now time.Time, expiresIn int64) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	return now.Add(time.Duration(expiresIn) * time.Second)
}

func formatExpiry(expiry time.Time) string {
	if expiry.IsZero() {
		return ""
	}
	return expiry.UTC().Format(time.RFC3339)
}
