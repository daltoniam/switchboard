package pluginoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
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

var (
	ErrConfiguration   = errors.New("oauth: invalid configuration")
	ErrDiscovery       = errors.New("oauth: issuer discovery failed")
	ErrState           = errors.New("oauth: invalid or expired authorization session")
	ErrReauthorization = errors.New("oauth: authorization required; connect again")
	ErrRequest         = errors.New("oauth: provider request failed")
	ErrPersistence     = errors.New("oauth: token persistence failed; retry before restarting")
)

type Store interface {
	GetIntegration(string) (*mcp.IntegrationConfig, bool)
	SetIntegration(string, *mcp.IntegrationConfig) error
}

type authorization struct {
	state    string
	browser  [32]byte
	verifier string
	redirect string
	expires  time.Time
	creds    mcp.Credentials
	metadata metadata
	updates  mcp.Credentials
	baseline mcp.Credentials
}

type Manager struct {
	mu         sync.Mutex
	name       string
	store      Store
	client     *http.Client
	randReader io.Reader
	now        func() time.Time
	creds      mcp.Credentials
	metadata   *metadata
	pending    *authorization
	dirty      bool
	verified   bool
	blocked    bool
	updates    mcp.Credentials
	previous   mcp.Credentials
	baseline   mcp.Credentials
}

func New(name string, store Store) *Manager {
	return &Manager{name: name, store: store, now: time.Now, randReader: rand.Reader, client: &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func Configured(creds mcp.Credentials) bool {
	for key, value := range creds {
		if strings.HasPrefix(key, "oauth_") && value != "" {
			return true
		}
	}
	return false
}

func validate(creds mcp.Credentials) error {
	if _, err := secureURL(creds["oauth_issuer"]); err != nil {
		return ErrConfiguration
	}
	for _, key := range []string{"oauth_client_id", "oauth_token_key", "oauth_subject", "oauth_email"} {
		if strings.TrimSpace(creds[key]) == "" {
			return ErrConfiguration
		}
	}
	if strings.HasPrefix(creds["oauth_token_key"], "oauth_") || creds["oauth_client_secret"] != "" {
		return ErrConfiguration
	}
	if raw := creds["oauth_expires_at"]; raw != "" {
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return ErrConfiguration
		}
	}
	return nil
}

func sameSettings(a, b mcp.Credentials) bool {
	for _, key := range settingsKeys {
		if a[key] != b[key] {
			return false
		}
	}
	return true
}

func (m *Manager) Load(creds mcp.Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.load(creds)
}

func (m *Manager) load(creds mcp.Credentials) error {
	if err := validate(creds); err != nil {
		return err
	}
	if m.dirty && m.previous != nil && sameSettings(m.previous, creds) {
		return nil
	}
	if m.creds != nil && sameSettings(m.creds, creds) {
		if m.creds["oauth_settings_binding"] != "" && m.creds["oauth_settings_binding"] != settingsBinding(m.creds) {
			clearTokens(m.creds)
			m.verified = false
		}
		for key, value := range creds {
			if !strings.HasPrefix(key, "oauth_") && !ManagedCredential(key, m.creds) {
				m.creds[key] = value
			}
		}
		return nil
	}
	if m.dirty {
		return ErrPersistence
	}
	if m.creds != nil {
		m.pending = nil
	}
	next := maps.Clone(creds)
	if (m.creds != nil && !sameSettings(m.creds, creds)) || (creds["oauth_settings_binding"] != "" && creds["oauth_settings_binding"] != settingsBinding(creds)) {
		clearTokens(next)
	}
	m.creds = next
	if m.store != nil {
		if ic, ok := m.store.GetIntegration(m.name); ok && ic != nil {
			m.baseline = maps.Clone(ic.Credentials)
		}
	}
	m.metadata = nil
	m.verified, m.blocked = false, false
	return nil
}

func (m *Manager) Start(ctx context.Context, creds mcp.Credentials, redirect, browser string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil {
		return "", ErrPersistence
	}
	if err := validate(creds); err != nil {
		return "", err
	}
	u, err := url.Parse(redirect)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/api/integrations/"+m.name+"/oauth/callback" || browser == "" {
		return "", ErrConfiguration
	}
	md, err := m.discover(ctx, creds["oauth_issuer"])
	if err != nil {
		return "", err
	}
	creds = maps.Clone(creds)
	state, err := randomValue(m.randReader)
	if err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	verifier, err := randomValue(m.randReader)
	if err != nil {
		return "", fmt.Errorf("oauth: generate PKCE verifier: %w", err)
	}
	hash := sha256.Sum256([]byte(verifier))
	u, _ = url.Parse(md.AuthorizationEndpoint)
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", creds["oauth_client_id"])
	q.Set("redirect_uri", redirect)
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(hash[:]))
	q.Set("code_challenge_method", "S256")
	scopes := creds["oauth_scopes"]
	if scopes == "" {
		scopes = "openid profile email offline_access"
	}
	q.Set("scope", scopes)
	u.RawQuery = q.Encode()
	updates := mcp.Credentials{}
	existing, ok := m.store.GetIntegration(m.name)
	for key, value := range creds {
		if !ManagedCredential(key, creds) && (!ok || existing == nil || existing.Credentials[key] != value) {
			updates[key] = value
		}
	}
	baseline := mcp.Credentials{}
	if ok && existing != nil {
		baseline = maps.Clone(existing.Credentials)
	}
	if m.dirty {
		m.dirty, m.verified, m.blocked = false, false, true
		clearTokens(m.creds)
		m.previous, m.updates = nil, nil
	}
	clearTokens(creds)
	m.pending = &authorization{baseline: baseline, updates: updates, state: state, browser: sha256.Sum256([]byte(browser)), verifier: verifier, redirect: redirect, expires: m.now().Add(10 * time.Minute), creds: maps.Clone(creds), metadata: *md}
	return u.String(), nil
}

func randomValue(reader io.Reader) (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (m *Manager) Callback(ctx context.Context, code, state, browser, issuer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.pending
	if p == nil || subtle.ConstantTimeCompare([]byte(state), []byte(p.state)) != 1 {
		return ErrState
	}
	browserHash := sha256.Sum256([]byte(browser))
	if !m.now().Before(p.expires) || subtle.ConstantTimeCompare(browserHash[:], p.browser[:]) != 1 || (issuer != "" && issuer != p.metadata.Issuer) {
		return ErrState
	}
	m.pending = nil
	if code == "" {
		return ErrReauthorization
	}
	token, err := m.exchange(ctx, p.metadata.TokenEndpoint, url.Values{
		"grant_type": {"authorization_code"}, "client_id": {p.creds["oauth_client_id"]},
		"code": {code}, "code_verifier": {p.verifier}, "redirect_uri": {p.redirect},
	})
	if err != nil {
		return err
	}
	if token.RefreshToken == "" {
		return ErrRequest
	}
	m.previous = m.creds
	m.creds, m.metadata = p.creds, &p.metadata
	m.updates, m.baseline = p.updates, p.baseline
	m.blocked = false
	m.accept(token, "")
	return m.finish(ctx)
}

func (m *Manager) Credentials(ctx context.Context) (mcp.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.blocked || m.creds == nil {
		return nil, ErrReauthorization
	}
	if m.dirty {
		if err := m.finish(ctx); err != nil {
			return nil, err
		}
	}
	expiry, _ := time.Parse(time.RFC3339, m.creds["oauth_expires_at"])
	if m.creds["oauth_access_token"] == "" || expiry.Before(m.now().Add(time.Minute)) {
		if err := m.refresh(ctx); err != nil {
			return nil, err
		}
	} else if !m.verified {
		if err := m.verify(ctx); err != nil {
			return nil, err
		}
	}
	guest := make(mcp.Credentials, len(m.creds))
	for key, value := range m.creds {
		if !strings.HasPrefix(key, "oauth_") {
			guest[key] = value
		}
	}
	guest[m.creds["oauth_token_key"]] = m.creds["oauth_access_token"]
	return guest, nil
}

func (m *Manager) refresh(ctx context.Context) error {
	if m.creds["oauth_refresh_token"] == "" {
		return ErrReauthorization
	}
	if err := m.ensureMetadata(ctx); err != nil {
		return err
	}
	token, err := m.exchange(ctx, m.metadata.TokenEndpoint, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {m.creds["oauth_client_id"]}, "refresh_token": {m.creds["oauth_refresh_token"]},
	})
	if err != nil {
		if errors.Is(err, ErrReauthorization) {
			m.blocked = true
		}
		return err
	}
	m.accept(token, m.creds["oauth_refresh_token"])
	return m.finish(ctx)
}

func (m *Manager) accept(token tokenResponse, previousRefresh string) {
	if token.RefreshToken == "" {
		token.RefreshToken = previousRefresh
	}
	m.creds["oauth_access_token"] = token.AccessToken
	m.creds["oauth_refresh_token"] = token.RefreshToken
	m.creds["oauth_expires_at"] = m.now().Add(time.Duration(token.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	m.creds["oauth_settings_binding"] = settingsBinding(m.creds)
	m.dirty, m.verified = true, false
}

func (m *Manager) finish(ctx context.Context) error {
	if m.blocked {
		return ErrReauthorization
	}
	expiry, _ := time.Parse(time.RFC3339, m.creds["oauth_expires_at"])
	if !m.verified && !m.now().Before(expiry) {
		return m.refresh(ctx)
	}
	if !m.verified {
		if err := m.verify(ctx); err != nil {
			return err
		}
	}
	return m.persist()
}

func (m *Manager) ensureMetadata(ctx context.Context) error {
	if m.metadata != nil {
		return nil
	}
	md, err := m.discover(ctx, m.creds["oauth_issuer"])
	if err != nil {
		return err
	}
	m.metadata = md
	return nil
}

func (m *Manager) verify(ctx context.Context) error {
	if err := m.ensureMetadata(ctx); err != nil {
		return err
	}
	var user struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
	}
	if err := m.request(ctx, http.MethodGet, m.metadata.UserinfoEndpoint, nil, m.creds["oauth_access_token"], &user); err != nil {
		return ErrRequest
	}
	if user.Subject != m.creds["oauth_subject"] || user.Email != m.creds["oauth_email"] {
		m.blocked = true
		m.dirty = false
		return ErrReauthorization
	}
	m.verified = true
	return nil
}

func (m *Manager) persist() error {
	if m.store == nil {
		return ErrPersistence
	}
	var saved mcp.Credentials
	err := updateIntegration(m.store, m.name, func(ic *mcp.IntegrationConfig) error {
		if m.baseline != nil && !sameSettings(m.baseline, ic.Credentials) {
			return ErrConfiguration
		}
		if ic.Credentials == nil {
			ic.Credentials = mcp.Credentials{}
		}
		if !sameSettings(ic.Credentials, m.creds) {
			clearTokens(ic.Credentials)
			delete(ic.Credentials, m.creds["oauth_token_key"])
		}
		maps.Copy(ic.Credentials, m.updates)
		for key, value := range m.creds {
			if strings.HasPrefix(key, "oauth_") {
				ic.Credentials[key] = value
			}
		}
		saved = maps.Clone(ic.Credentials)
		return nil
	})
	if err != nil {
		return ErrPersistence
	}
	m.creds = saved
	m.baseline = maps.Clone(saved)
	m.dirty, m.updates, m.previous = false, nil, nil
	return nil
}

func (m *Manager) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty {
		return nil
	}
	return m.finish(ctx)
}

func (m *Manager) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.creds != nil
}
