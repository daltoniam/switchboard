package netpolicy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserHTTPClient_RejectsRedirectToLoopback(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(redirectTarget.Close)

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	t.Cleanup(public.Close)

	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL: func(raw string) error {
			if raw == redirectTarget.URL {
				return assert.AnError
			}
			return nil
		},
	})
	req, err := http.NewRequest(http.MethodGet, public.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	assert.Error(t, err)
}

func TestNewUserHTTPClient_ValidatesInitialURL(t *testing.T) {
	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL: func(string) error {
			return assert.AnError
		},
	})
	req, err := http.NewRequest(http.MethodGet, "https://example.test/", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	assert.Error(t, err)
}

func TestNewUserHTTPClient_StripsAuthOnCrossOriginRedirect(t *testing.T) {
	var seenAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(other.Close)

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	t.Cleanup(public.Close)

	origin, err := url.Parse(public.URL)
	require.NoError(t, err)
	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL:  func(string) error { return nil },
		AuthHeader:   "X-API-Key",
		AuthValue:    "secret",
		AuthOrigin:   origin,
	})
	req, err := http.NewRequest(http.MethodGet, public.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Empty(t, seenAuth)
}

func TestNewUserHTTPClient_StripsAuthThroughRedirectChain(t *testing.T) {
	var finalAuth string
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finalAuth = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(final.Close)

	mid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	t.Cleanup(mid.Close)

	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mid.URL, http.StatusFound)
	}))
	t.Cleanup(start.Close)

	origin, err := url.Parse(start.URL)
	require.NoError(t, err)
	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL:  func(string) error { return nil },
		AuthHeader:   "X-API-Key",
		AuthValue:    "secret",
		AuthOrigin:   origin,
	})
	req, err := http.NewRequest(http.MethodGet, start.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Empty(t, finalAuth)
}

func TestNewUserHTTPClient_OmitsAuthWithoutOrigin(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL:  func(string) error { return nil },
		AuthHeader:   "X-API-Key",
		AuthValue:    "secret",
	})
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Empty(t, seenAuth)
}

func TestDialContext_BlocksLoopback(t *testing.T) {
	_, err := dialContext(context.Background(), "tcp", "127.0.0.1:9", false)
	assert.Error(t, err)
}

func TestDialContext_RejectsDisallowedResolution(t *testing.T) {
	orig := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = orig })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	_, err := dialContext(context.Background(), "tcp", "rebind.example.test:443", false)
	assert.Error(t, err)
}

func TestDialContext_FallsBackPastAnUnreachableAddressInTime(t *testing.T) {
	// The first address never answers (a blackholed IPv6 route, say); the
	// second accepts at once. The dial must reach the second well within the
	// client timeout instead of spending it all on the first.
	origLookup, origDial := lookupIPAddr, dialAddr
	t.Cleanup(func() { lookupIPAddr, dialAddr = origLookup, origDial })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("2001:db8::1")}, {IP: net.ParseIP("93.184.216.34")}}, nil
	}
	dialAddr = func(ctx context.Context, _ *net.Dialer, _, target string) (net.Conn, error) {
		if strings.HasPrefix(target, "[2001:db8::1]") {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		client, server := net.Pipe()
		t.Cleanup(func() { _ = server.Close() })
		return client, nil
	}

	start := time.Now()
	conn, err := dialContext(context.Background(), "tcp", "dualstack.example.test:443", false)
	require.NoError(t, err)
	_ = conn.Close()
	assert.Less(t, time.Since(start), 2*time.Second, "an unreachable first address must not hold up the next one")
}

func TestDialContext_StopsAtTheOverallDeadline(t *testing.T) {
	origLookup, origDial := lookupIPAddr, dialAddr
	t.Cleanup(func() { lookupIPAddr, dialAddr = origLookup, origDial })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("2001:db8::1")}, {IP: net.ParseIP("2001:db8::2")}}, nil
	}
	dialAddr = func(ctx context.Context, _ *net.Dialer, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := dialContext(ctx, "tcp", "blackhole.example.test:443", false)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second, "dialing must respect the caller's deadline")
}
