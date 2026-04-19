package wasm

import (
	"context"
	"fmt"
	"log"
	"os"
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
	if hasNonEmpty {
		if err := mod.Configure(ctx, mergedCreds); err != nil {
			mod.Close(ctx) //nolint:errcheck
			return fmt.Errorf("configure WASM module %q: %w", path, err)
		}
	}

	l.mu.Lock()
	if old, ok := l.modules[mod.Name()]; ok {
		l.reg.Unregister(mod.Name())
		old.Close(ctx) //nolint:errcheck
		delete(l.modules, mod.Name())
	}
	l.mu.Unlock()

	if err := l.reg.Register(mod); err != nil {
		mod.Close(ctx) //nolint:errcheck
		return fmt.Errorf("register WASM module %q: %w", path, err)
	}

	ic := &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mergedCreds,
	}
	if hasExisting {
		ic.ToolGlobs = existing.ToolGlobs
	}
	_ = l.cfgMgr.SetIntegration(mod.Name(), ic)

	l.mu.Lock()
	l.modules[mod.Name()] = mod
	l.mu.Unlock()

	log.Printf("Live-loaded WASM integration %q from %s", mod.Name(), path)
	return nil
}

// UnloadPlugin removes a WASM module from the registry and closes it.
func (l *Loader) UnloadPlugin(ctx context.Context, name string) error {
	l.mu.Lock()
	mod, ok := l.modules[name]
	if ok {
		delete(l.modules, name)
	}
	l.mu.Unlock()

	if !ok {
		l.reg.Unregister(name)
		return nil
	}

	l.reg.Unregister(name)
	return mod.Close(ctx)
}

// Runtime returns the underlying wazero Runtime.
func (l *Loader) Runtime() *Runtime {
	return l.rt
}
