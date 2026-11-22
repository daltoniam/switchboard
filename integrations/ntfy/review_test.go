package ntfy

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
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
	for _, tc := range []struct {
		name      string
		replayIDs []string
		wantIDs   []string
	}{
		{"ascending IDs", []string{"AAAAAAAAAAAA", "ZZZZZZZZZZZZ"}, []string{"ZZZZZZZZZZZZ"}},
		{"descending IDs", []string{"ZZZZZZZZZZZZ", "AAAAAAAAAAAA"}, []string{"AAAAAAAAAAAA"}},
		{"mixed IDs", []string{"AAAAAAAAAAAA", "ZZZZZZZZZZZZ", "MMMMMMMMMMMM"}, []string{"MMMMMMMMMMMM", "ZZZZZZZZZZZZ"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body strings.Builder
			for _, id := range tc.replayIDs {
				body.WriteString(strings.Replace(fixture, "AbCd1234EfGh", id, 1))
				body.WriteByte('\n')
			}
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = io.WriteString(w, body.String())
			})
			result, value := resultJSON(t, adapter, "ntfy_list_messages", map[string]any{"limit": len(tc.wantIDs)})
			require.False(t, result.IsError, result.Data)
			messages := value["messages"].([]any)
			require.Len(t, messages, len(tc.wantIDs))
			var gotIDs []string
			for _, message := range messages {
				gotIDs = append(gotIDs, message.(map[string]any)["id"].(string))
			}
			require.Equal(t, tc.wantIDs, gotIDs, "same-second messages must use reverse replay order")
			require.Equal(t, true, value["truncations"].(map[string]any)["message_limit"])
		})
	}
}

func TestReconfigureAndCancelledCallsDoNotWaitForInflightPoll(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	markEntered := sync.OnceFunc(func() { close(entered) })
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ntfy/ops/json", r.URL.Path)
		assert.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		markEntered()
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
	replacementCalls := make(chan struct{}, 2)
	_, replacement := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/newtopic/json", r.URL.Path)
		assert.Equal(t, "Bearer new-secret", r.Header.Get("Authorization"))
		select {
		case replacementCalls <- struct{}{}:
		default:
			t.Error("unexpected additional request to the replacement server")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, strings.Replace(fixture, `"topic":"ops"`, `"topic":"newtopic"`, 1)+"\n")
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
		t.Fatal("reconfiguration waited for an in-flight network poll")
	}
	replacementContext, stopReplacement := context.WithTimeout(t.Context(), time.Second)
	defer stopReplacement()
	replacementResult, err := adapter.Execute(replacementContext, "ntfy_list_messages", nil)
	require.NoError(t, err)
	require.False(t, replacementResult.IsError, replacementResult.Data)
	var replacementValue map[string]any
	require.NoError(t, json.Unmarshal([]byte(replacementResult.Data), &replacementValue))
	require.Equal(t, "newtopic", replacementValue["scope"].(map[string]any)["topic"])
	require.Len(t, replacementValue["messages"], 1)
	require.Equal(t, "AbCd1234EfGh", replacementValue["messages"].([]any)[0].(map[string]any)["id"])
	require.True(t, adapter.Healthy(replacementContext))
	require.Len(t, replacementCalls, 2, "Execute and Healthy must both reach the replacement server")
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
	require.Len(t, replacementCalls, 2, "cancelled calls must not reach the replacement server")
	cancel()
	select {
	case value := <-first:
		require.NoError(t, value.err)
		require.NotNil(t, value.result)
		require.True(t, value.result.IsError, value.result.Data)
	case <-time.After(time.Second):
		t.Fatal("first poll did not respect cancellation")
	}
}

func TestInflightPollRetainsConfiguredSnapshot(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	releasePoll := sync.OnceFunc(func() { close(release) })
	defer releasePoll()
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, fixture+"\n")
	})
	type execution struct {
		result *mcp.ToolResult
		err    error
	}
	result := make(chan execution, 1)
	go func() {
		value, err := adapter.Execute(t.Context(), "ntfy_list_messages", nil)
		result <- execution{value, err}
	}()
	select {
	case <-entered:
	case value := <-result:
		t.Fatalf("poll returned before reaching the HTTP handler: result=%+v, err=%v", value.result, value.err)
	case <-time.After(time.Second):
		t.Fatal("poll did not reach the HTTP handler")
	}
	closed := make(chan error, 1)
	go func() { closed <- adapter.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Error("Close waited for an in-flight network poll")
	}
	releasePoll()
	select {
	case value := <-result:
		require.NoError(t, value.err)
		require.NotNil(t, value.result)
		require.False(t, value.result.IsError, value.result.Data)
		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(value.result.Data), &raw))
		require.Equal(t, "ops", raw["scope"].(map[string]any)["topic"])
		require.Len(t, raw["messages"], 1)
	case <-time.After(time.Second):
		t.Fatal("poll did not finish after releasing the HTTP handler")
	}
}
