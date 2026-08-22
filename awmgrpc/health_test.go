package awmgrpc

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/awm"
	awmv1 "github.com/daltoniam/switchboard/gen/awm/v1"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealth_ServingOnLoopbackAndUDS(t *testing.T) {
	root := t.TempDir()
	catalog := project.NewStore(root)
	require.NoError(t, catalog.Load())
	work := awm.NewStore(root)
	work.SetCatalog(catalog)

	t.Run("loopback h2c", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		srv := NewServer(catalog, catalog, catalog, work, Options{CatalogEnabled: true, WritesEnabled: true})
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(srv.Stop)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)
		conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		health := grpc_health_v1.NewHealthClient(conn)
		assertServing(t, ctx, health, "")
		assertServing(t, ctx, health, awmv1.ProjectCatalogService_ServiceDesc.ServiceName)
		assertServing(t, ctx, health, awmv1.ResourceService_ServiceDesc.ServiceName)
		assertServing(t, ctx, health, awmv1.WorkSessionService_ServiceDesc.ServiceName)
	})

	t.Run("unix domain socket", func(t *testing.T) {
		requireUnix(t)
		path := filepath.Join(t.TempDir(), "awm.sock")
		ln, err := ListenUnix(path)
		require.NoError(t, err)
		srv := NewServer(catalog, catalog, catalog, work, Options{CatalogEnabled: true, WritesEnabled: true})
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() { srv.Stop(); _ = ln.Close() })

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)
		conn, err := grpc.NewClient("unix://"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		health := grpc_health_v1.NewHealthClient(conn)
		assertServing(t, ctx, health, "")
		assertServing(t, ctx, health, awmv1.ProjectCatalogService_ServiceDesc.ServiceName)

		listed, err := awmv1.NewProjectCatalogServiceClient(conn).ListProjects(ctx, &awmv1.ListProjectsRequest{})
		require.NoError(t, err)
		assert.Empty(t, listed.Projects)
	})
}

func TestHealth_CatalogDisabledMarksCatalogNotServing(t *testing.T) {
	root := t.TempDir()
	catalog := project.NewStore(root)
	require.NoError(t, catalog.Load())
	work := awm.NewStore(root)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := NewServer(catalog, catalog, catalog, work, Options{CatalogEnabled: false, WritesEnabled: true})
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	health := grpc_health_v1.NewHealthClient(conn)
	assertServing(t, ctx, health, "")
	assertServing(t, ctx, health, awmv1.ResourceService_ServiceDesc.ServiceName)

	resp, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: awmv1.ProjectCatalogService_ServiceDesc.ServiceName,
	})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
}

func assertServing(t *testing.T, ctx context.Context, client grpc_health_v1.HealthClient, service string) {
	t.Helper()
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: service})
	require.NoError(t, err, "health check %q", service)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status, "service %q", service)
}
