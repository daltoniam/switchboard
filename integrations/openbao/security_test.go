package openbao

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCancellation(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	started := time.Now()
	result, err := adapter.Execute(ctx, "openbao_get_health", nil)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Less(t, time.Since(started), time.Second)
}

func TestTLSVerificationRequired(t *testing.T) {
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"initialized":true,"sealed":false,"standby":false}`)
	}))
	defer provider.Close()
	adapter := New()
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": provider.URL, "token": "test", "mount": "secret", "path_prefix": "approved"}))
	result := execute(t, adapter, "openbao_get_health", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "CA certificate")
}

func TestInstallationPrefixPreserved(t *testing.T) {
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/proxy/bao/v1/secret/metadata/approved/key", request.URL.Path)
		_, _ = io.WriteString(writer, metadataFixture)
	}))
	defer provider.Close()
	adapter := New()
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{
		"base_url": provider.URL + "/proxy/bao/", "token": "test", "mount": "secret", "path_prefix": "approved",
		"ca_cert": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})),
	}))
	assert.False(t, execute(t, adapter, "openbao_get_secret_metadata", map[string]any{"path": "key"}).IsError)
}

func TestUntrustedListings(t *testing.T) {
	cases := []string{
		`{"data":{"keys":["../outside"]}}`,
		`{"data":{"keys":["nested/key"]}}`,
		`{"data":{"keys":["zeta","alpha"]}}`,
		`{"data":{"keys":["same","same"]}}`,
		`{"data":{"keys":["a","b","c"]}}`,
	}
	for _, fixture := range cases {
		t.Run(fixture, func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, fixture) })
			assert.True(t, execute(t, adapter, "openbao_list_secret_keys", map[string]any{"limit": 1}).IsError)
		})
	}
}

func TestMetadataMissingFieldsStayMissing(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"data":{"current_version":1,"created_time":null,"versions":{"1":{"destroyed":null}}}}`)
	})
	result := execute(t, adapter, "openbao_get_secret_metadata", map[string]any{"path": "key"})
	require.False(t, result.IsError, result.Data)
	assert.NotContains(t, result.Data, "destroyed")
	assert.NotContains(t, result.Data, "created_time")
}

func TestMalformedMetadataVersions(t *testing.T) {
	for _, key := range []string{"0", "-1", "not-a-version", "18446744073709551616"} {
		t.Run(key, func(t *testing.T) {
			adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, `{"data":{"current_version":1,"versions":{"`+key+`":{}}}}`)
			})
			assert.True(t, execute(t, adapter, "openbao_get_secret_metadata", map[string]any{"path": "key"}).IsError)
		})
	}
}

func TestEmptyListing(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"data":{"keys":[]}}`)
	})
	result := execute(t, adapter, "openbao_list_secret_keys", nil)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, `"keys":[]`)
	assert.Contains(t, result.Data, `"has_more":false`)
}

func TestDefaultPagination(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "21", request.URL.Query().Get("limit"))
		_, _ = io.WriteString(writer, `{"data":{"keys":["key"]}}`)
	})
	assert.False(t, execute(t, adapter, "openbao_list_secret_keys", nil).IsError)
}

func TestRetryableStatus(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "3")
		writer.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := adapter.Execute(t.Context(), "openbao_list_secret_keys", nil)
	var retry *mcp.RetryableError
	require.ErrorAs(t, err, &retry)
	assert.Equal(t, 3*time.Second, retry.RetryAfter)
	assert.NotContains(t, err.Error(), "test-token")
}

func TestCloseAndReconfigure(t *testing.T) {
	adapter := testAdapter(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"initialized":true,"sealed":false,"standby":false}`)
	})
	closer, ok := any(adapter).(io.Closer)
	require.True(t, ok, "HTTP transport connections must be released when hosts drop the adapter")
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	assert.False(t, adapter.Healthy(t.Context()))
	assert.True(t, execute(t, adapter, "openbao_get_health", nil).IsError)
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials{"base_url": "https://bao.example.com", "token": "replacement", "mount": "secret", "path_prefix": "approved"}))
	assert.NotNil(t, adapter.client)
	assert.Equal(t, "replacement", adapter.token)
}

func TestInputDoesNotEscapeInErrors(t *testing.T) {
	adapter := testAdapter(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	result := execute(t, adapter, "openbao_get_secret_metadata", map[string]any{"path": strings.Repeat("SECRET_SENTINEL", 100)})
	require.True(t, result.IsError)
	assert.NotContains(t, result.Data, "SECRET_SENTINEL")
}
