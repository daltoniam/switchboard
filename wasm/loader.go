package wasm

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

// Loader manages live-loading and unloading of WASM plugins. It holds the
// wazero Runtime and provides thread-safe LoadPlugin / UnloadPlugin methods
// that the web handlers call after marketplace install/update/uninstall.
type Loader struct {
	mu      sync.Mutex
	rt      *Runtime
	reg     mcp.Registry
	cfgMgr  mcp.ConfigService
	modules map[string]*Module // name -> loaded module
}

// NewLoader creates a Loader from an existing Runtime.
func NewLoader(rt *Runtime, reg mcp.Registry, cfgMgr mcp.ConfigService) *Loader {
	return &Loader{
		rt:      rt,
		reg:     reg,
		cfgMgr:  cfgMgr,
		modules: make(map[string]*Module),
	}
}

// TrackModule records a module that was loaded at startup so UnloadPlugin
// can close it later. Called by the startup path after loadWasmModule.
func (l *Loader) TrackModule(mod *Module) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.modules[mod.Name()] = mod
}

// LoadPlugin reads a WASM file from disk, instantiates it, configures it
// with merged credentials, and registers it in the integration registry.
// If a module with the same name is already loaded it is unloaded first.
func (l *Loader) LoadPlugin(ctx context.Context, path string, nameOverride string) error {
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read WASM module %q: %w", path, err)
	}

	mod, err := l.rt.LoadModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("load WASM module %q: %w", path, err)
	}
	if nameOverride != "" {
		mod.SetName(nameOverride)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	name := mod.Name()
	if old, ok := l.modules[name]; ok {
		if err := old.Close(ctx); err != nil {
			_ = mod.Close(ctx)
			return fmt.Errorf("close previous WASM module: %w", err)
		}
		l.reg.Unregister(name)
		delete(l.modules, name)
	}

	mod.SetConfigService(l.cfgMgr)

	mergedCreds := mcp.Credentials{}
	for _, key := range mod.CredentialKeys() {
		mergedCreds[key] = ""
	}
	existing, hasExisting := l.cfgMgr.GetIntegration(mod.Name())
	if hasExisting {
		for k, v := range existing.Credentials {
			mergedCreds[k] = v
		}
	}

	hasNonEmpty := false
	for _, v := range mergedCreds {
		if v != "" {
			hasNonEmpty = true
			break
		}
	}
	// Disabled plugins are loaded for discovery/configuration but must not be
	// validated until the user explicitly enables them. This allows partial
	// credentials to be staged without startup rewriting or load failure.
	if hasExisting && existing.Enabled && hasNonEmpty {
		if err := mod.Configure(ctx, mergedCreds); err != nil {
			mod.Close(ctx) //nolint:errcheck
			return fmt.Errorf("configure WASM module %q: %w", path, err)
		}
	}

	ic := &mcp.IntegrationConfig{
		Enabled:     false,
		Credentials: mergedCreds,
	}
	if hasExisting {
		ic.Enabled = existing.Enabled
		ic.ToolGlobs = append([]string(nil), existing.ToolGlobs...)
		if existing.Identities != nil {
			ic.Identities = make(map[string]mcp.IntegrationIdentity, len(existing.Identities))
		}
		for id, identity := range existing.Identities {
			identityCopy := mcp.IntegrationIdentity{}
			if identity.Credentials != nil {
				identityCopy.Credentials = mcp.Credentials{}
			}
			for key, value := range identity.Credentials {
				identityCopy.Credentials[key] = value
			}
			if identity.Metadata != nil {
				identityCopy.Metadata = map[string]string{}
			}
			for key, value := range identity.Metadata {
				identityCopy.Metadata[key] = value
			}
			ic.Identities[id] = identityCopy
		}
	}
	// Persist only when a new plugin needs a config entry or its declared
	// credential schema changed. Startup loading must never rewrite unrelated
	// config state or silently enable a plugin.
	if !hasExisting || !reflect.DeepEqual(existing, ic) {
		if err := l.mergeCredentialSchema(mod.Name(), mod.CredentialKeys(), ic); err != nil {
			mod.Close(ctx) //nolint:errcheck
			return fmt.Errorf("persist WASM module %q configuration: %w", path, err)
		}
	}

	if err := l.reg.Register(mod); err != nil {
		mod.Close(ctx) //nolint:errcheck
		return fmt.Errorf("register WASM module %q: %w", path, err)
	}

	l.modules[mod.Name()] = mod

	log.Printf("Live-loaded WASM integration %q from %s", mod.Name(), path)
	return nil
}

func (l *Loader) mergeCredentialSchema(name string, keys []string, fallback *mcp.IntegrationConfig) error {
	if updater, ok := l.cfgMgr.(mcp.IntegrationConfigUpdater); ok {
		return updater.UpdateIntegration(name, func(ic *mcp.IntegrationConfig) error {
			if ic.Credentials == nil {
				ic.Credentials = mcp.Credentials{}
			}
			for _, key := range keys {
				if _, exists := ic.Credentials[key]; !exists {
					ic.Credentials[key] = ""
				}
			}
			return nil
		})
	}
	return l.cfgMgr.SetIntegration(name, fallback)
}

// UnloadPlugin removes a WASM module from the registry and closes it.
func (l *Loader) UnloadPlugin(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if mod, ok := l.modules[name]; ok {
		if err := mod.Close(ctx); err != nil {
			return err
		}
		delete(l.modules, name)
	}
	l.reg.Unregister(name)
	return nil
}

// Runtime returns the underlying wazero Runtime.
func (l *Loader) Runtime() *Runtime {
	return l.rt
}
