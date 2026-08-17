package slack

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshViaCookie_EmptyCookie(t *testing.T) {
	result, err := refreshViaCookie("")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestRefreshViaCookie_CapturesRotatedCookie(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "xoxd-rotated-new"})
		fmt.Fprintf(w, `<script>var boot_data = {"token": "xoxc-refreshed-token-123"};</script>`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-old-cookie")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "xoxc-refreshed-token-123", result.token)
	assert.Equal(t, "xoxd-rotated-new", result.cookie)
}

func TestRefreshViaCookie_KeepsOriginalCookieWhenNoRotation(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<script>var boot_data = {"token": "xoxc-refreshed-token-456"};</script>`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-original-cookie")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "xoxc-refreshed-token-456", result.token)
	assert.Equal(t, "xoxd-original-cookie", result.cookie)
}

func TestRefreshViaCookie_ApiTokenFallback(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "xoxd-rotated-api"})
		fmt.Fprintf(w, `{"api_token":"xoxc-api-fallback-789","other":"data"}`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-old")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "xoxc-api-fallback-789", result.token)
	assert.Equal(t, "xoxd-rotated-api", result.cookie)
}

func TestRefreshViaCookie_NoTokenInResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "xoxd-new"})
		fmt.Fprintf(w, `<html>login page, no token here</html>`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-expired")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestRefreshViaCookie_IgnoresNonXoxdCookie(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "not-a-xoxd"})
		fmt.Fprintf(w, `{"token": "xoxc-test-token-000"}`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-original")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "xoxd-original", result.cookie)
}

func TestRefreshViaCookie_UsesLastSetCookie(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "xoxd-first"})
		http.SetCookie(w, &http.Cookie{Name: "d", Value: "xoxd-second"})
		fmt.Fprintf(w, `{"token": "xoxc-test-token-111"}`)
	}))
	defer srv.Close()

	result, err := refreshViaCookieWithClient(srv.Client(), srv.URL, "xoxd-original")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "xoxd-second", result.cookie)
}

func TestXoxcPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard", `"token":"xoxc-abc-123"`, "xoxc-abc-123"},
		{"with spaces", `"token" : "xoxc-def-456"`, "xoxc-def-456"},
		{"no match", `"token":"xoxp-not-browser"`, ""},
		{"empty", ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := xoxcPattern.FindStringSubmatch(tt.input)
			if tt.want == "" {
				assert.Empty(t, m)
			} else {
				require.Len(t, m, 2)
				assert.Equal(t, tt.want, m[1])
			}
		})
	}
}
