package teams

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Microsoft Entra device-code OAuth flow.
//
// Endpoint shape (v2.0):
//
//	POST {login_base_url}/{tenant_or_common}/oauth2/v2.0/devicecode
//	     client_id={client_id}
//	     scope={space-separated scopes}
//
//	-> { "device_code", "user_code", "verification_uri", "expires_in", "interval", "message" }
//
//	POST {login_base_url}/{tenant_or_common}/oauth2/v2.0/token
//	     client_id={client_id}
//	     grant_type=urn:ietf:params:oauth:grant-type:device_code
//	     device_code={device_code}
//
//	-> { "access_token", "refresh_token", "expires_in", "token_type", "id_token", "scope" }
//	   or { "error": "authorization_pending" | "slow_down" | "expired_token" | "access_denied" | ... }
//
// We deliberately do NOT require a client_secret — the default Azure CLI client_id is
// a "public client" (no secret). Customer apps that DO have a secret should set it
// via credentials["client_secret"] and we'll add it to the form payload.

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type accessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	ExpiresIn        int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// oauthFlow tracks an in-progress device-code authorization on behalf of a
// single tenant (or "common"). Only one flow can be in progress at a time,
// matching the Sentry adapter's pattern.
type oauthFlow struct {
	mu           sync.Mutex
	integration  *teamsIntegration
	clientID     string
	clientSecret string
	tenantHint   string // "common" or a specific tenant id; sent in the URL
	scopes       string
	loginBaseURL string

	deviceResp *deviceCodeResponse
	startedAt  time.Time

	done     bool
	tenantID string // resolved from id_token after success
	userOID  string
	userUPN  string
	userName string
	errMsg   string
}

var activeFlow struct {
	mu   sync.Mutex
	flow *oauthFlow
}

// startOAuth kicks off a device-code flow. The returned response includes the
// user_code + verification_uri the human needs.
func (t *teamsIntegration) startOAuth(ctx context.Context, tenantHint string) (*deviceCodeResponse, error) {
	if tenantHint == "" {
		tenantHint = "common"
	}
	t.mu.RLock()
	loginBase := t.loginBaseURL
	clientID := t.clientID
	clientSecret := "" // We'll allow override via creds later; not stored on the struct yet.
	scopes := t.scopes
	t.mu.RUnlock()

	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/devicecode", loginBase, tenantHint)

	form := url.Values{
		"client_id": {clientID},
		"scope":     {scopes},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var dcr deviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return nil, fmt.Errorf("parse device code response: %w", err)
	}

	flow := &oauthFlow{
		integration:  t,
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantHint:   tenantHint,
		scopes:       scopes,
		loginBaseURL: loginBase,
		deviceResp:   &dcr,
		startedAt:    time.Now(),
	}

	activeFlow.mu.Lock()
	activeFlow.flow = flow
	activeFlow.mu.Unlock()

	go flow.poll()

	return &dcr, nil
}

func (f *oauthFlow) poll() {
	interval := time.Duration(f.deviceResp.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := f.startedAt.Add(time.Duration(f.deviceResp.ExpiresIn) * time.Second)

	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", f.loginBaseURL, f.tenantHint)

	for {
		time.Sleep(interval)

		if time.Now().After(deadline) {
			f.mu.Lock()
			f.errMsg = "Authorization timed out. Please try teams_login again."
			f.done = true
			f.mu.Unlock()
			return
		}

		form := url.Values{
			"client_id":   {f.clientID},
			"device_code": {f.deviceResp.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}
		if f.clientSecret != "" {
			form.Set("client_secret", f.clientSecret)
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := f.integration.httpClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var atr accessTokenResponse
		if err := json.Unmarshal(body, &atr); err != nil {
			continue
		}

		switch atr.Error {
		case "":
			if atr.AccessToken == "" {
				continue
			}
			tn := &tenant{
				AccessToken:  atr.AccessToken,
				RefreshToken: atr.RefreshToken,
				ExpiresAt:    time.Now().Add(time.Duration(atr.ExpiresIn) * time.Second),
				Source:       "device_code",
			}
			// id_token carries the tenant + user identity.
			if atr.IDToken != "" {
				if claims, err := parseIDToken(atr.IDToken); err == nil {
					tn.TenantID = claims.TID
					tn.UserOID = claims.OID
					tn.UserUPN = claims.UPN
					if tn.UserUPN == "" {
						tn.UserUPN = claims.PreferredUsername
					}
					tn.UserDisplay = claims.Name
				}
			}
			if tn.TenantID == "" {
				// As a last resort, hit /me with the new token to discover tenantId.
				tn.TenantID = f.tenantHint
			}
			f.integration.store.upsert(tn)
			f.integration.store.setDefault(tn.TenantID)
			_ = f.integration.store.saveToFile()

			f.mu.Lock()
			f.done = true
			f.tenantID = tn.TenantID
			f.userOID = tn.UserOID
			f.userUPN = tn.UserUPN
			f.userName = tn.UserDisplay
			f.mu.Unlock()
			return

		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			f.mu.Lock()
			f.errMsg = "Device code expired. Please try teams_login again."
			f.done = true
			f.mu.Unlock()
			return
		case "access_denied":
			f.mu.Lock()
			f.errMsg = "Authorization was denied."
			f.done = true
			f.mu.Unlock()
			return
		default:
			msg := atr.Error
			if atr.ErrorDescription != "" {
				msg = atr.Error + ": " + atr.ErrorDescription
			}
			f.mu.Lock()
			f.errMsg = msg
			f.done = true
			f.mu.Unlock()
			return
		}
	}
}

type pollResult struct {
	Status      string `json:"status"`
	TenantID    string `json:"tenant_id,omitempty"`
	UserOID     string `json:"user_oid,omitempty"`
	UserUPN     string `json:"user_upn,omitempty"`
	UserDisplay string `json:"user_display,omitempty"`
	Error       string `json:"error,omitempty"`
}

// pollOAuth returns the current state of the in-progress flow.
func pollOAuth() pollResult {
	activeFlow.mu.Lock()
	flow := activeFlow.flow
	activeFlow.mu.Unlock()
	if flow == nil {
		return pollResult{Status: "no_flow", Error: "No OAuth flow in progress. Run teams_login first."}
	}
	flow.mu.Lock()
	defer flow.mu.Unlock()
	if !flow.done {
		return pollResult{Status: "pending"}
	}
	if flow.errMsg != "" {
		return pollResult{Status: "error", Error: flow.errMsg}
	}
	return pollResult{
		Status:      "complete",
		TenantID:    flow.tenantID,
		UserOID:     flow.userOID,
		UserUPN:     flow.userUPN,
		UserDisplay: flow.userName,
	}
}

// idTokenClaims captures the fields we care about from an id_token JWT.
type idTokenClaims struct {
	TID               string `json:"tid"`
	OID               string `json:"oid"`
	UPN               string `json:"upn"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
}

func parseIDToken(idToken string) (*idTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed id_token")
	}
	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, err
	}
	var c idTokenClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func base64URLDecode(s string) ([]byte, error) {
	// JWT spec uses base64url without padding.
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// refreshTenant exchanges a refresh token for a fresh access token. Called by
// graphRequestInner on 401 and proactively before expiry.
func (t *teamsIntegration) refreshTenant(ctx context.Context, tn *tenant) error {
	if tn == nil || tn.RefreshToken == "" {
		return fmt.Errorf("no refresh_token available for tenant %s", tn.TenantID)
	}

	t.mu.RLock()
	loginBase := t.loginBaseURL
	clientID := t.clientID
	scopes := t.scopes
	t.mu.RUnlock()

	tenantHint := tn.TenantID
	if tenantHint == "" || tenantHint == "_config" {
		tenantHint = "common"
	}

	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", loginBase, tenantHint)
	form := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {tn.RefreshToken},
		"scope":         {scopes},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var atr accessTokenResponse
	if err := json.Unmarshal(body, &atr); err != nil {
		return err
	}
	if atr.Error != "" {
		return fmt.Errorf("refresh error: %s — %s", atr.Error, atr.ErrorDescription)
	}
	if atr.AccessToken == "" {
		return fmt.Errorf("refresh returned no access_token")
	}

	updated := &tenant{
		TenantID:     tn.TenantID,
		TenantName:   tn.TenantName,
		UserOID:      tn.UserOID,
		UserUPN:      tn.UserUPN,
		UserDisplay:  tn.UserDisplay,
		AccessToken:  atr.AccessToken,
		RefreshToken: tn.RefreshToken, // Microsoft may rotate; prefer new when present.
		ExpiresAt:    time.Now().Add(time.Duration(atr.ExpiresIn) * time.Second),
		Source:       tn.Source,
	}
	if atr.RefreshToken != "" {
		updated.RefreshToken = atr.RefreshToken
	}
	t.store.upsert(updated)
	_ = t.store.saveToFile()
	return nil
}
