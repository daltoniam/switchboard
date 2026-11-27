package notion

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

// idleTrackingServer counts connections the client has closed.
func idleTrackingServer(t *testing.T) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var opened, closed atomic.Int32
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"object":"user","id":"u1","results":[]}`)
	}))
	ts.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed.Add(1)
		}
	}
	ts.Start()
	t.Cleanup(ts.Close)
	return ts, &opened, &closed
}

// Close must release the adapter's pooled connections, so a host that
// builds one instance per request leaves nothing open behind it.
func TestClose_ReleasesV1Connections(t *testing.T) {
	ts, opened, closed := idleTrackingServer(t)
	n := New().(*notion)
	require.NoError(t, n.Configure(context.Background(), mcp.Credentials{"access_token": "tok", "base_url": ts.URL}))
	require.Positive(t, opened.Load())
	require.Zero(t, closed.Load(), "the token check leaves its connection idle in the pool")

	require.NoError(t, n.Close())
	require.Eventually(t, func() bool { return closed.Load() == opened.Load() }, 2*time.Second, 10*time.Millisecond,
		"Close must close the idle connections")
	require.NoError(t, n.Close(), "Close is safe to call twice")
}

// Reconfiguring replaces the v1 backend; the old one's connections go too.
func TestConfigure_ReleasesReplacedV1Connections(t *testing.T) {
	ts, opened, closed := idleTrackingServer(t)
	n := New().(*notion)
	t.Cleanup(func() { _ = n.Close() })
	require.NoError(t, n.Configure(context.Background(), mcp.Credentials{"access_token": "old", "base_url": ts.URL}))
	first := opened.Load()
	require.NoError(t, n.Configure(context.Background(), mcp.Credentials{"access_token": "new", "base_url": ts.URL}))
	require.Eventually(t, func() bool { return closed.Load() >= first }, 2*time.Second, 10*time.Millisecond,
		"the replaced backend's idle connections must be closed")
}
