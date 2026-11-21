package ntfy

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestPublicServiceTrailingDotIsRejected(t *testing.T) {
	for _, host := range []string{"ntfy.sh.", "foo.ntfy.sh.", "NTFY.SH.", "ntfy.sh", "foo.ntfy.sh"} {
		t.Run(host, func(t *testing.T) {
			err := New().Configure(t.Context(), mcp.Credentials{"base_url": "https://" + host, "topic": "ops", "api_token": "test-secret"})
			require.Error(t, err)
		})
	}
}

func TestSameSecondMessagesUseReverseReplayOrder(t *testing.T) {
	body := strings.Replace(fixture, "AbCd1234EfGh", "AAAAAAAAAAAA", 1) + "\n" + strings.Replace(fixture, "AbCd1234EfGh", "ZZZZZZZZZZZZ", 1) + "\n"
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, body)
	})
	result, value := resultJSON(t, adapter, "ntfy_list_messages", map[string]any{"limit": 1})
	require.False(t, result.IsError, result.Data)
	require.Equal(t, "ZZZZZZZZZZZZ", value["messages"].([]any)[0].(map[string]any)["id"])
	require.Equal(t, true, value["truncations"].(map[string]any)["message_limit"])
}

func TestReconfigureAndCancelledCallsDoNotWaitForInflightPoll(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ntfy/ops/json", r.URL.Path)
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		close(entered)
		select {
		case <-release:
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, fixture+"\n")
		case <-r.Context().Done():
		}
	})
	defer close(release)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type execution struct {
		result *mcp.ToolResult
		err    error
	}
	first := make(chan execution, 1)
	go func() {
		result, err := adapter.Execute(ctx, "ntfy_list_messages", nil)
		first <- execution{result, err}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first poll did not start")
	}
	_, replacement := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/newtopic/json", r.URL.Path)
		require.Equal(t, "Bearer new-secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/x-ndjson")
	})
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: replacement.Certificate().Raw})
	reconfigured := make(chan error, 1)
	go func() {
		reconfigured <- adapter.Configure(ctx, mcp.Credentials{"base_url": replacement.URL, "topic": "newtopic", "api_token": "new-secret", "ca_cert": string(cert)})
	}()
	select {
	case err := <-reconfigured:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Error("reconfiguration waited for an in-flight network poll")
	}
	cancelled, stop := context.WithCancel(t.Context())
	stop()
	cancelledExecute := make(chan execution, 1)
	go func() {
		result, err := adapter.Execute(cancelled, "ntfy_list_messages", nil)
		cancelledExecute <- execution{result, err}
	}()
	cancelledHealth := make(chan bool, 1)
	go func() { cancelledHealth <- adapter.Healthy(cancelled) }()
	select {
	case result := <-cancelledExecute:
		require.NoError(t, result.err)
		require.True(t, result.result.IsError)
	case <-time.After(200 * time.Millisecond):
		t.Error("cancelled Execute waited behind reconfiguration and an in-flight poll")
	}
	select {
	case healthy := <-cancelledHealth:
		require.False(t, healthy)
	case <-time.After(200 * time.Millisecond):
		t.Error("cancelled Healthy waited behind reconfiguration and an in-flight poll")
	}
	cancel()
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("first poll did not respect cancellation")
	}
}

func TestInflightPollRetainsConfiguredSnapshot(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, fixture+"\n")
	})
	result := make(chan *mcp.ToolResult, 1)
	go func() { value, _ := adapter.Execute(t.Context(), "ntfy_list_messages", nil); result <- value }()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- adapter.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Error("Close waited for an in-flight network poll")
	}
	close(release)
	value := <-result
	require.False(t, value.IsError, value.Data)
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(value.Data), &raw))
	require.Equal(t, "ops", raw["scope"].(map[string]any)["topic"])
	require.Len(t, raw["messages"], 1)
}
