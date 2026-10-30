package netpolicy

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL: func(raw string) error {
			u, err := http.NewRequest(http.MethodGet, raw, nil)
			if err != nil {
				return err
			}
			if u.URL.Host == "" {
				return assert.AnError
			}
			return nil
		},
		AuthHeader: "X-API-Key",
	})
	req, err := http.NewRequest(http.MethodGet, public.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", "secret")
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

	client := NewUserHTTPClient(UserHTTPClientConfig{
		AllowPrivate: true,
		ValidateURL:  func(string) error { return nil },
		AuthHeader:   "X-API-Key",
	})
	req, err := http.NewRequest(http.MethodGet, start.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", "secret")
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Empty(t, finalAuth)
}

func TestDialContext_BlocksLoopback(t *testing.T) {
	_, err := dialContext(context.Background(), "tcp", "127.0.0.1:9", false)
	assert.Error(t, err)
}
