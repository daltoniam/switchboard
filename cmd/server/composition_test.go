package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/awm"
	"github.com/daltoniam/switchboard/awmgrpc"
	awmv1 "github.com/daltoniam/switchboard/gen/awm/v1"
	"github.com/daltoniam/switchboard/integrations/projectinterop"
	"github.com/daltoniam/switchboard/project"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
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
		"name": "shared", "description": "from-interop",
	})
	require.NoError(t, err)

	def, ok := store.Definition("shared")
	require.True(t, ok)
	assert.Equal(t, "from-interop", def.Description)
}

func TestProductionListen_DefaultsToLoopback(t *testing.T) {
	addr, err := awmgrpc.TCPListenAddr("", 3847)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3847", addr)

	addr, err = awmgrpc.TCPListenAddr("0.0.0.0", 3847)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3847", addr)
}

func TestProductionGRPC_UnixSocketDoesNotServeHTTP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not used on windows")
	}
	root := t.TempDir()
	store := project.NewStore(root)
	require.NoError(t, store.Load())
	work := awm.NewStore(root)
	work.SetCatalog(store)

	grpcServer := awmgrpc.NewServer(store, store, store, work, awmgrpc.Options{
		CatalogEnabled: true,
		WritesEnabled:  true,
	})
	path := filepath.Join(t.TempDir(), "awm.sock")
	ln, err := awmgrpc.ListenUnix(path)
	require.NoError(t, err)
	go func() { _ = grpcServer.Serve(ln) }()
	t.Cleanup(func() { grpcServer.Stop(); _ = ln.Close(); _ = os.Remove(path) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	conn, err := grpc.NewClient("unix://"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	health, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, health.Status)

	listed, err := awmv1.NewProjectCatalogServiceClient(conn).ListProjects(ctx, &awmv1.ListProjectsRequest{})
	require.NoError(t, err)
	assert.Empty(t, listed.Projects)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
		Timeout: time.Second,
	}
	_, err = httpClient.Get("http://unix/mcp")
	require.Error(t, err, "HTTP/MCP must not be reachable on the native gRPC unix socket")
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
