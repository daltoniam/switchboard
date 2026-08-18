package main

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/projectinterop"
	"github.com/daltoniam/switchboard/project"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedCatalog_InteropMutationVisibleToRouterStore(t *testing.T) {
	root := t.TempDir()
	store := project.NewStore(root)
	require.NoError(t, store.Load())

	interop := projectinterop.NewWithCatalog(store)
	require.NoError(t, interop.Configure(context.Background(), mcp.Credentials{"config_root": t.TempDir()}))

	reg := registry.New()
	require.NoError(t, reg.Register(interop))
	services := &mcp.Services{
		Config:   stubConfig{integrations: map[string]*mcp.IntegrationConfig{"projectinterop": {Enabled: true}}},
		Registry: reg,
	}
	srv := server.New(services)
	router := server.NewProjectRouter(services, store, "", srv.SearchIndex())
	require.NotNil(t, router)

	_, err := interop.Execute(context.Background(), "projectinterop_create_project", map[string]any{
		"name": "shared", "repo": "/tmp/shared", "branch": "from-interop",
	})
	require.NoError(t, err)

	def, ok := store.Definition("shared")
	require.True(t, ok)
	assert.Equal(t, "from-interop", def.PrimaryBranch())
}

type stubConfig struct {
	integrations map[string]*mcp.IntegrationConfig
}

func (s stubConfig) Load() error { return nil }
func (s stubConfig) Save() error { return nil }
func (s stubConfig) Get() *mcp.Config {
	return &mcp.Config{Integrations: s.integrations}
}
func (s stubConfig) Update(*mcp.Config) error { return nil }
func (s stubConfig) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := s.integrations[name]
	return ic, ok
}
func (s stubConfig) SetIntegration(string, *mcp.IntegrationConfig) error { return nil }
func (s stubConfig) EnabledIntegrations() []string {
	var names []string
	for name, ic := range s.integrations {
		if ic != nil && ic.Enabled {
			names = append(names, name)
		}
	}
	return names
}
func (s stubConfig) SetWasmModules([]mcp.WasmModuleConfig) error { return nil }
func (s stubConfig) DefaultCredentialKeys(string) []string       { return nil }
