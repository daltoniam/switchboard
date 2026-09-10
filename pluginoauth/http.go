package pluginoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxBody = 1 << 20

type metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func secureURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, ErrConfiguration
	}
	return u, nil
}

func (m *Manager) discover(ctx context.Context, issuer string) (*metadata, error) {
	var md metadata
	base := strings.TrimRight(issuer, "/")
	if err := m.request(ctx, http.MethodGet, base+"/.well-known/oauth-authorization-server", nil, "", &md); err != nil {
		return nil, ErrDiscovery
	}
	if err := validateMetadata(issuer, md, false); err != nil {
		return nil, err
	}
	if md.UserinfoEndpoint == "" || md.JWKSURI == "" {
		var oidc metadata
		if err := m.request(ctx, http.MethodGet, base+"/.well-known/openid-configuration", nil, "", &oidc); err != nil {
			return nil, ErrDiscovery
		}
		if err := validateMetadata(issuer, oidc, true); err != nil {
			return nil, err
		}
		if oidc.AuthorizationEndpoint != md.AuthorizationEndpoint || oidc.TokenEndpoint != md.TokenEndpoint {
			return nil, ErrDiscovery
		}
		md.UserinfoEndpoint, md.JWKSURI = oidc.UserinfoEndpoint, oidc.JWKSURI
	}
	return &md, validateMetadata(issuer, md, true)
}

func validateMetadata(issuer string, md metadata, complete bool) error {
	base, err := secureURL(issuer)
	if err != nil || md.Issuer != issuer {
		return ErrDiscovery
	}
	endpoints := []string{md.AuthorizationEndpoint, md.TokenEndpoint}
	for _, endpoint := range []string{md.UserinfoEndpoint, md.JWKSURI} {
		if endpoint != "" || complete {
			endpoints = append(endpoints, endpoint)
		}
	}
	for _, endpoint := range endpoints {
		u, err := secureURL(endpoint)
		if err != nil || !strings.EqualFold(u.Host, base.Host) {
			return ErrDiscovery
		}
	}
	return nil
}

func (m *Manager) exchange(ctx context.Context, endpoint string, form url.Values) (tokenResponse, error) {
	var token tokenResponse
	err := m.request(ctx, http.MethodPost, endpoint, form, "", &token)
	if err != nil {
		return tokenResponse{}, err
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn <= 0 || token.ExpiresIn > (1<<63-1)/1_000_000_000 {
		return tokenResponse{}, ErrRequest
	}
	return token, nil
}

func (m *Manager) request(ctx context.Context, method, endpoint string, form url.Values, bearer string, result any) error {
	if _, err := secureURL(endpoint); err != nil {
		return ErrRequest
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return ErrRequest
	}
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return ErrRequest
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(data) > maxBody {
		return ErrRequest
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		if method == http.MethodPost && json.Unmarshal(data, &failure) == nil && failure.Error == "invalid_grant" {
			return ErrReauthorization
		}
		return ErrRequest
	}
	if err := json.Unmarshal(data, result); err != nil {
		return ErrRequest
	}
	return nil
}
