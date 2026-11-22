package ntfy

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

type gatedTransport struct {
	transport *http.Transport
	entered   chan struct{}
	release   chan struct{}
}

func (transport *gatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	close(transport.entered)
	select {
	case <-transport.release:
		return transport.transport.RoundTrip(request)
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

func (transport *gatedTransport) CloseIdleConnections() {
	transport.transport.CloseIdleConnections()
}

func TestRetiredSnapshotClosesConnectionsStartedAfterRetirement(t *testing.T) {
	for _, retirement := range []string{"close", "configure"} {
		for _, operation := range []string{"execute", "healthy"} {
			for _, protocol := range []string{"http1", "http2"} {
				t.Run(retirement+"/"+operation+"/"+protocol, func(t *testing.T) {
					closedConnections := make(chan struct{}, 1)
					protocols := make(chan int, 1)
					provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
						protocols <- request.ProtoMajor
						w.Header().Set("Content-Type", "application/x-ndjson")
						_, _ = w.Write([]byte(fixture + "\n"))
					}))
					provider.EnableHTTP2 = protocol == "http2"
					provider.Config.ConnState = func(_ net.Conn, state http.ConnState) {
						if state == http.StateClosed {
							select {
							case closedConnections <- struct{}{}:
							default:
							}
						}
					}
					provider.StartTLS()
					t.Cleanup(provider.Close)
					adapter := New().(*ntfy)
					t.Cleanup(func() { require.NoError(t, adapter.Close()) })
					cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
					creds := mcp.Credentials{"base_url": provider.URL, "topic": "ops", "api_token": "test-secret", "ca_cert": string(cert)}
					require.NoError(t, adapter.Configure(t.Context(), creds))
					state := adapter.snapshot()
					transport := &gatedTransport{
						transport: state.client.Transport.(*http.Transport),
						entered:   make(chan struct{}),
						release:   make(chan struct{}),
					}
					state.client.Transport = transport
					t.Cleanup(transport.CloseIdleConnections)
					releaseRequest := sync.OnceFunc(func() { close(transport.release) })
					defer releaseRequest()
					ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
					defer cancel()
					type execution struct {
						result  *mcp.ToolResult
						err     error
						healthy bool
					}
					results := make(chan execution, 1)
					go func() {
						if operation == "healthy" {
							results <- execution{healthy: adapter.Healthy(ctx)}
							return
						}
						result, err := adapter.Execute(ctx, "ntfy_list_messages", nil)
						results <- execution{result: result, err: err}
					}()
					select {
					case <-transport.entered:
					case value := <-results:
						t.Fatalf("operation returned before acquiring a snapshot: %+v", value)
					case <-ctx.Done():
						t.Fatal("operation did not acquire a snapshot")
					}
					retired := make(chan error, 1)
					go func() {
						if retirement == "configure" {
							retired <- adapter.Configure(ctx, creds)
							return
						}
						retired <- adapter.Close()
					}()
					select {
					case err := <-retired:
						require.NoError(t, err)
					case <-time.After(time.Second):
						t.Fatal("snapshot retirement waited for an active operation")
					}
					require.NotSame(t, state, adapter.snapshot())
					releaseRequest()
					select {
					case value := <-results:
						require.NoError(t, value.err)
						if operation == "healthy" {
							require.True(t, value.healthy)
						} else {
							require.NotNil(t, value.result)
							require.False(t, value.result.IsError, value.result.Data)
							require.Contains(t, value.result.Data, "AbCd1234EfGh")
						}
					case <-ctx.Done():
						t.Fatal("retired snapshot did not finish its operation")
					}
					select {
					case major := <-protocols:
						if protocol == "http2" {
							require.Equal(t, 2, major)
						} else {
							require.Equal(t, 1, major)
						}
					case <-ctx.Done():
						t.Fatal("operation did not reach the provider")
					}
					select {
					case <-closedConnections:
					case <-time.After(time.Second):
						t.Fatal("retired snapshot retained its completed connection")
					}
				})
			}
		}
	}
}
