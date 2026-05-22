package remotemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RefreshAccessToken exchanges a refresh_token for a fresh access token (and,
// per OAuth 2.1 best practice, potentially a rotated refresh_token). It is the
// counterpart to HandleOAuthCallback for use cases where the integration
// already holds a refresh_token from a previous flow — no browser, no PKCE.
//
// serverURL is the same base URL used for the initial OAuth discovery (e.g.
// "https://api.smith.langchain.com"). RefreshAccessToken re-discovers the
// token endpoint each call rather than caching it — token endpoints almost
// never change, but this keeps the function stateless and avoids depending on
// any per-server cache initialized by StartOAuth.
//
// The returned TokenSet's RefreshToken field is non-empty only if the upstream
// rotated the refresh token; callers that rotate should persist the new value.
func RefreshAccessToken(ctx context.Context, serverURL, clientID, clientSecret, refreshToken string) (TokenSet, error) {
	if refreshToken == "" {
		return TokenSet{}, fmt.Errorf("refresh_token is required")
	}
	if clientID == "" {
		return TokenSet{}, fmt.Errorf("client_id is required for refresh")
	}

	meta, err := discoverOAuth(serverURL)
	if err != nil {
		return TokenSet{}, fmt.Errorf("discover token endpoint: %w", err)
	}
	if meta.TokenEndpoint == "" {
		return TokenSet{}, fmt.Errorf("server did not advertise a token_endpoint")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
		"resource":      {mcpResourceURL(serverURL)},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", meta.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenSet{}, fmt.Errorf("refresh request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenSet{}, fmt.Errorf("read refresh response: %w", err)
	}
	if resp.StatusCode != 200 {
		return TokenSet{}, fmt.Errorf("refresh endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return TokenSet{}, fmt.Errorf("parse refresh response: %w", err)
	}
	if tokenResp.Error != "" {
		return TokenSet{}, fmt.Errorf("refresh error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return TokenSet{}, fmt.Errorf("refresh response missing access_token")
	}

	return TokenSet{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}
