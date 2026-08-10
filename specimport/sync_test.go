package specimport_test

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/specimport"
)

const openAPISync = `{
  "openapi": "3.0.0",
  "info": {"title": "Demo API"},
  "servers": [{"url": "https://api.example.com/v1"}],
  "paths": {"/users": {"get": {"operationId": "listUsers"}}}
}`

func cfg(name, spec string) mcp.SpecImportConfig {
	return mcp.SpecImportConfig{Name: name, Kind: "openapi", Spec: spec, Enabled: true}
}

func TestSyncer_AddsEnabled(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	res := s.Sync([]mcp.SpecImportConfig{cfg("Demo API", openAPISync)})
	if len(res.Added) != 1 || res.Added[0] != "demo_api" {
		t.Fatalf("Added = %v", res.Added)
	}
	if !res.Changed() {
		t.Error("Changed() = false, want true")
	}
	if _, ok := reg.Get("demo_api"); !ok {
		t.Error("demo_api not registered")
	}
}

func TestSyncer_SkipsDisabled(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	c := cfg("Demo API", openAPISync)
	c.Enabled = false
	res := s.Sync([]mcp.SpecImportConfig{c})
	if res.Changed() {
		t.Errorf("Changed() = true, want false: %+v", res)
	}
	if _, ok := reg.Get("demo_api"); ok {
		t.Error("disabled entry should not register")
	}
}

func TestSyncer_NoOpOnUnchanged(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)
	entries := []mcp.SpecImportConfig{cfg("Demo API", openAPISync)}

	s.Sync(entries)
	res := s.Sync(entries)
	if res.Changed() {
		t.Errorf("second sync changed: %+v", res)
	}
}

func TestSyncer_RemovesDropped(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	s.Sync([]mcp.SpecImportConfig{cfg("Demo API", openAPISync)})
	res := s.Sync(nil)
	if len(res.Removed) != 1 || res.Removed[0] != "demo_api" {
		t.Fatalf("Removed = %v", res.Removed)
	}
	if _, ok := reg.Get("demo_api"); ok {
		t.Error("demo_api should be unregistered")
	}
	if s.Managed("demo_api") {
		t.Error("demo_api should no longer be managed")
	}
}

func TestSyncer_UpdatesChanged(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	s.Sync([]mcp.SpecImportConfig{cfg("Demo API", openAPISync)})

	changed := cfg("Demo API", openAPISync)
	changed.Credentials = mcp.Credentials{"api_key": "secret"}
	res := s.Sync([]mcp.SpecImportConfig{changed})
	if len(res.Updated) != 1 || res.Updated[0] != "demo_api" {
		t.Fatalf("Updated = %v", res.Updated)
	}
	if _, ok := reg.Get("demo_api"); !ok {
		t.Error("demo_api should remain registered after update")
	}
}

func TestSyncer_BadEntryCollectsError(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	res := s.Sync([]mcp.SpecImportConfig{cfg("Bad", "not json")})
	if len(res.Errors) != 1 {
		t.Fatalf("Errors = %v", res.Errors)
	}
	if res.Changed() {
		t.Error("bad entry should not report change")
	}
}

func TestSyncer_LeavesUnmanagedAlone(t *testing.T) {
	reg := registry.New()
	s := specimport.NewSyncer(reg)

	// Register an unmanaged integration directly on the registry.
	unmanaged, err := specimport.Load(cfg("Other API", openAPISync))
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(unmanaged); err != nil {
		t.Fatal(err)
	}

	// Sync a managed entry, then clear it. The unmanaged one must survive.
	s.Sync([]mcp.SpecImportConfig{cfg("Demo API", openAPISync)})
	s.Sync(nil)

	if _, ok := reg.Get("other_api"); !ok {
		t.Error("unmanaged integration should not be removed by Sync")
	}
	if _, ok := reg.Get("demo_api"); ok {
		t.Error("managed integration should be removed")
	}
}
