package netpolicy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
