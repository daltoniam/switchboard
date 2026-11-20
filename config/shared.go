package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/gofrs/flock"
)

// fileLockTimeout bounds the wait for another process's write. It is longer
// than a token request, which a process may make while holding the lock.
const fileLockTimeout = 45 * time.Second

// withFileLock runs fn holding the lock file every Switchboard process takes
// before writing config.json. Call with m.mu held; nested calls reuse the lock.
func (m *manager) withFileLock(ctx context.Context, fn func() error) error {
	if m.fileLockHeld {
		return fn()
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, fileLockTimeout)
	defer cancel()
	lock := flock.New(m.filePath + ".lock")
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return fmt.Errorf("lock config: %w", err)
	}
	if !locked {
		return fmt.Errorf("lock config: timed out waiting for another Switchboard process")
	}
	defer func() { _ = lock.Unlock() }()
	m.fileLockHeld = true
	defer func() { m.fileLockHeld = false }()
	return fn()
}

// adoptSavedCredentials copies credentials another process saved since this
// process last read or wrote the file. A credential this process changed in
// the meantime keeps its own value. Call with m.mu and the file lock held.
func (m *manager) adoptSavedCredentials() error {
	if m.base == nil || m.persisted == nil {
		return nil
	}
	data, err := os.ReadFile(m.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var file mcp.Config
	if json.Unmarshal(data, &file) != nil {
		return nil // an unreadable file is overwritten, as before
	}
	saved := mergeWithDefaults(&file)
	for name, savedIC := range saved.Integrations {
		baseIC, ours := m.base.Integrations[name], m.persisted.Integrations[name]
		if baseIC == nil || ours == nil {
			continue
		}
		runtime := m.cfg.Integrations[name]
		for key := range credentialKeys(baseIC.Credentials, savedIC.Credentials) {
			value, saved := savedIC.Credentials[key]
			baseValue, inBase := baseIC.Credentials[key]
			ourValue, inOurs := ours.Credentials[key]
			if (saved == inBase && value == baseValue) || inOurs != inBase || ourValue != baseValue {
				continue // unchanged on disk, or changed here: keep ours
			}
			envVar, mapped := envMapping[name][key]
			fromFile := runtime != nil && (!mapped || m.envLookup(envVar) == "")
			if !saved {
				delete(ours.Credentials, key)
				if fromFile {
					delete(runtime.Credentials, key)
				}
				continue
			}
			ours.Credentials[key] = value
			if fromFile {
				runtime.Credentials[key] = value
			}
		}
	}
	m.base = saved
	return nil
}

// UpdateIntegrationShared implements mcp.SharedIntegrationConfigUpdater.
func (m *manager) UpdateIntegrationShared(ctx context.Context, name string, update func(*mcp.IntegrationConfig) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.withFileLock(ctx, func() error {
		if err := m.adoptSavedCredentials(); err != nil {
			return err
		}
		ic := cloneIntegrationConfig(m.cfg.Integrations[name])
		if ic == nil {
			ic = &mcp.IntegrationConfig{Credentials: mcp.Credentials{}}
		}
		if ic.Credentials == nil {
			ic.Credentials = mcp.Credentials{}
		}
		if err := update(ic); err != nil {
			return err
		}
		return m.setIntegrationLocked(name, ic)
	})
}

// credentialKeys returns every key present in either set.
func credentialKeys(a, b mcp.Credentials) map[string]struct{} {
	keys := make(map[string]struct{}, len(a)+len(b))
	for key := range a {
		keys[key] = struct{}{}
	}
	for key := range b {
		keys[key] = struct{}{}
	}
	return keys
}
