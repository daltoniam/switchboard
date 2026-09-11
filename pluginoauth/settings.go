package pluginoauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"

	mcp "github.com/daltoniam/switchboard"
)

var settingsKeys = []string{"oauth_issuer", "oauth_client_id", "oauth_token_key", "oauth_subject", "oauth_email", "oauth_scopes"}

func settingsBinding(creds mcp.Credentials) string {
	values := make([]string, 0, len(settingsKeys))
	for _, key := range settingsKeys {
		values = append(values, creds[key])
	}
	data, _ := json.Marshal(values)
	hash := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func ManagedCredential(key string, creds mcp.Credentials) bool {
	switch key {
	case "oauth_access_token", "oauth_refresh_token", "oauth_expires_at", "oauth_settings_binding", "oauth_client_secret":
		return true
	}
	return key != "" && key == creds["oauth_token_key"]
}

func clearTokens(creds mcp.Credentials) {
	for key := range creds {
		if ManagedCredential(key, creds) {
			delete(creds, key)
		}
	}
}

func MergeCredentials(current, updates mcp.Credentials) mcp.Credentials {
	next := maps.Clone(current)
	if next == nil {
		next = mcp.Credentials{}
	}
	for key, value := range updates {
		if !ManagedCredential(key, current) && !ManagedCredential(key, updates) {
			next[key] = value
		}
	}
	if !sameSettings(current, next) || (next["oauth_settings_binding"] != "" && next["oauth_settings_binding"] != settingsBinding(next)) {
		delete(next, current["oauth_token_key"])
		clearTokens(next)
	}
	return next
}

func updateIntegration(store Store, name string, update func(*mcp.IntegrationConfig) error) error {
	if store == nil {
		return ErrPersistence
	}
	if atomic, ok := store.(mcp.IntegrationConfigUpdater); ok {
		return atomic.UpdateIntegration(name, update)
	}
	existing, ok := store.GetIntegration(name)
	if !ok || existing == nil {
		return ErrPersistence
	}
	ic := *existing
	ic.Credentials = maps.Clone(existing.Credentials)
	if err := update(&ic); err != nil {
		return err
	}
	return store.SetIntegration(name, &ic)
}

func (m *Manager) EditCredentials(ctx context.Context, updates mcp.Credentials, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dirty {
		if err := m.finish(ctx); err != nil {
			return err
		}
	}
	var next mcp.Credentials
	err := updateIntegration(m.store, m.name, func(ic *mcp.IntegrationConfig) error {
		next = MergeCredentials(ic.Credentials, updates)
		if m.verified && !m.blocked && m.creds != nil && sameSettings(m.creds, next) {
			for key := range next {
				if ManagedCredential(key, next) {
					delete(next, key)
				}
			}
			for key, value := range m.creds {
				if ManagedCredential(key, m.creds) {
					next[key] = value
				}
			}
		}
		ic.Credentials, ic.Enabled = next, enabled
		return nil
	})
	if err != nil {
		return ErrPersistence
	}
	changed := m.creds == nil || !sameSettings(m.creds, next)
	m.pending = nil
	m.creds = maps.Clone(next)
	m.baseline = maps.Clone(next)
	if changed {
		m.metadata, m.verified, m.blocked = nil, false, false
	}
	if !Configured(next) {
		m.creds = nil
	}
	return nil
}
