package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestProjectCatalog_ListChangedOnCreate(t *testing.T) {
	store := project.NewStore(t.TempDir())
	require.NoError(t, store.Load())
	bus := project.NewEventBus()
	store.SetEventBus(bus)
	cat := NewProjectCatalogServer(store, store, store, store, ProjectCatalogOptions{
		WritesEnabled: true,
	})
	cat.StartEventBridge(bus)

	httpSrv := httptest.NewServer(BuildHTTPMux(HTTPMuxConfig{ProjectCatalog: cat.Handler()}))
	t.Cleanup(httpSrv.Close)

	got := make(chan struct{}, 1)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "sub", Version: "0"}, &mcpsdk.ClientOptions{
		ResourceListChangedHandler: func(context.Context, *mcpsdk.ResourceListChangedRequest) {
			select {
			case got <- struct{}{}:
			default:
			}
		},
	})
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL + "/project-catalog/mcp",
		HTTPClient: http.DefaultClient,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	time.Sleep(100 * time.Millisecond)
	_, err = store.Create(context.Background(), project.CreateRequest{Definition: project.Definition{Version: "1", Name: "listen", Description: "x"}})
	require.NoError(t, err)

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("expected resources/list_changed")
	}
}
