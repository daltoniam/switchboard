package jenkins

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
)

type gatedTransport struct {
	transport *http.Transport
	entered   chan struct{}
	release   chan struct{}
}

func (g *gatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	close(g.entered)
	select {
	case <-g.release:
		return g.transport.RoundTrip(request)
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

func (g *gatedTransport) CloseIdleConnections() {
	g.transport.CloseIdleConnections()
}

func TestRetiredClientClosesConnectionsStartedAfterRetirement(t *testing.T) {
	for _, retirement := range []string{"close", "configure"} {
		for _, operation := range []string{"execute", "healthy"} {
			for _, protocol := range []string{"http1", "http2"} {
				t.Run(retirement+"/"+operation+"/"+protocol, func(t *testing.T) {
					closedConnections := make(chan struct{}, 1)
					protocols := make(chan int, 1)
					provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						protocols <- r.ProtoMajor
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(jobsFixture))
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
					j := New().(*jenkins)
					t.Cleanup(func() { _ = j.Close() })
					creds := credentials(provider.URL)
					creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
					if err := j.Configure(t.Context(), creds); err != nil {
						t.Fatal(err)
					}
					client := j.client
					transport := &gatedTransport{transport: client.Transport.(*http.Transport), entered: make(chan struct{}), release: make(chan struct{})}
					client.Transport = transport
					t.Cleanup(transport.CloseIdleConnections)
					release := sync.OnceFunc(func() { close(transport.release) })
					defer release()
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
							results <- execution{healthy: j.Healthy(ctx)}
							return
						}
						result, err := j.Execute(ctx, "jenkins_list_jobs", nil)
						results <- execution{result: result, err: err}
					}()
					select {
					case <-transport.entered:
					case <-ctx.Done():
						t.Fatal("operation did not acquire client")
					}
					retired := make(chan error, 1)
					go func() {
						if retirement == "configure" {
							retired <- j.Configure(ctx, creds)
							return
						}
						retired <- j.Close()
					}()
					select {
					case err := <-retired:
						if err != nil {
							t.Fatal(err)
						}
					case <-time.After(time.Second):
						t.Fatal("retirement blocked on active operation")
					}
					release()
					select {
					case value := <-results:
						if value.err != nil || (operation == "healthy" && !value.healthy) || (operation == "execute" && (value.result == nil || value.result.IsError)) {
							t.Fatalf("retired operation failed: %+v", value)
						}
					case <-ctx.Done():
						t.Fatal("retired operation did not finish")
					}
					select {
					case major := <-protocols:
						want := 1
						if protocol == "http2" {
							want = 2
						}
						if major != want {
							t.Fatalf("protocol=%d want %d", major, want)
						}
					case <-ctx.Done():
						t.Fatal("request did not reach provider")
					}
					select {
					case <-closedConnections:
					case <-time.After(time.Second):
						t.Fatal("retired client retained completed connection")
					}
				})
			}
		}
	}
}
