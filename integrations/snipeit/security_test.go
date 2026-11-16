package snipeit

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestTagAuthorization(t *testing.T) {
	cases := []struct {
		name, body, exact string
		status            int
	}{
		{"denied company", assetFixture, "SECRET_OTHER_COMPANY", 403},
		{"different tag", assetFixture, strings.Replace(assetFixture, "IT & 7", "OTHER", 1), 200},
		{"malformed", `{}`, assetFixture, 200},
		{"incomplete", `{"total":2,"rows":[{"id":7}]}`, assetFixture, 200},
		{"excessive", `{"total":26,"rows":[` + strings.TrimSuffix(strings.Repeat(`{"id":7},`, 26), ",") + `]}`, assetFixture, 200},
		{"zero id", `{"total":1,"rows":[{"id":0}]}`, assetFixture, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/bytag/") {
					_, _ = io.WriteString(w, tc.body)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.exact)
			})
			result, err := adapter.Execute(t.Context(), "snipeit_get_asset_by_tag", map[string]any{"tag": "IT & 7"})
			require.NoError(t, err)
			require.True(t, result.IsError, result.Data)
			require.NotContains(t, result.Data, "SECRET")
		})
	}
}

func TestRedirectNeverReceivesCredential(t *testing.T) {
	var calls atomic.Int32
	destination := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(destination.Close)
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	})
	result, err := adapter.Execute(t.Context(), "snipeit_list_assets", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Zero(t, calls.Load())
}

func TestTLSRequiresTrustedCertificate(t *testing.T) {
	adapter, server := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"total":0,"rows":[]}`) })
	require.NoError(t, adapter.Configure(t.Context(), map[string]string{"base_url": server.URL, "token": "test-secret"}))
	require.False(t, adapter.Healthy(t.Context()))
}

func TestDecodedResponseLimit(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		_, _ = io.WriteString(writer, strings.Repeat(" ", 2*1024*1024+1))
		require.NoError(t, writer.Close())
	})
	result, err := adapter.Execute(t.Context(), "snipeit_list_assets", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "2 MiB")
}

func TestPageValidation(t *testing.T) {
	cases := []struct{ name, body string }{
		{"empty", `{"total":0,"rows":[]}`},
		{"null rows", `{"total":0,"rows":null}`},
		{"too many", `{"total":2,"rows":[` + assetFixture + `,` + assetFixture + `]}`},
		{"negative total", `{"total":-1,"rows":[]}`},
		{"oversized metadata", `{"total":1,"rows":[` + strings.Replace(assetFixture, "Incident laptop", strings.Repeat("x", 513), 1) + `]}`},
		{"invalid assignment", `{"total":1,"rows":[` + strings.Replace(assetFixture, `"type":"user"`, `"type":"SECRET"`, 1) + `]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tc.body) })
			result, err := adapter.Execute(t.Context(), "snipeit_list_assets", map[string]any{"limit": 1})
			require.NoError(t, err)
			require.Equal(t, tc.name != "empty", result.IsError, result.Data)
			if tc.name == "empty" {
				require.JSONEq(t, `{"total":0,"rows":[],"next_offset":null}`, result.Data)
			}
		})
	}
}

func TestDefaultPageAndDeadline(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "25", r.URL.Query().Get("limit"))
		require.Equal(t, "0", r.URL.Query().Get("offset"))
		_, _ = io.WriteString(w, `{"total":0,"rows":[]}`)
	})
	result, err := adapter.Execute(t.Context(), "snipeit_list_assets", nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err = adapter.Execute(ctx, "snipeit_list_assets", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestRateLimitGuidance(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "12")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	result, err := adapter.Execute(t.Context(), "snipeit_list_assets", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Data, "12 seconds")
}

func TestCredentialHints(t *testing.T) {
	adapter := New().(*snipeit)
	require.ElementsMatch(t, []string{"base_url", "ca_cert"}, adapter.PlainTextKeys())
	require.Equal(t, []string{"ca_cert"}, adapter.OptionalKeys())
	require.Len(t, adapter.Placeholders(), 3)
	require.Contains(t, adapter.Placeholders()["token"], "assets.view only")
}

func TestRawProjectionShape(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, assetFixture) })
	result, err := adapter.Execute(t.Context(), "snipeit_get_asset", map[string]any{"id": 7})
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &raw))
	require.Len(t, raw, 8)
	require.Equal(t, map[string]any{"id": float64(5), "type": "user"}, raw["assigned_to"])
	require.Equal(t, map[string]any{"id": float64(2), "name": "Laptop"}, raw["model"])
	fields, ok := adapter.CompactSpec("snipeit_get_asset")
	require.True(t, ok)
	require.Equal(t, raw, mcp.CompactAny(raw, fields))
}
