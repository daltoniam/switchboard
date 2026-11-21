package ntfy

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestMessageScopeAndBinaryContentAreRejected(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"binary encoding", strings.Replace(fixture, `"topic":"ops"`, `"topic":"ops","encoding":"base64"`, 1)},
		{"wrong topic", strings.Replace(fixture, `"topic":"ops"`, `"topic":"unrelated"`, 1)},
		{"missing topic", strings.Replace(fixture, `"topic":"ops",`, "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = io.WriteString(w, tc.body)
			})
			result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.True(t, result.IsError, result.Data)
			require.Equal(t, true, value["incomplete"])
			require.Empty(t, value["messages"])
			require.NotContains(t, result.Data, "Backup failed")
		})
	}
}

func TestFinalBudgetUsesJSONEscaping(t *testing.T) {
	body := ""
	for index := range 20 {
		body += fmt.Sprintf(`{"event":"message","topic":"ops","id":"%012d","time":%d,"title":"%s","message":"%s","tags":["%s"]}`+"\n", index, index+1, strings.Repeat("<", 128), strings.Repeat("<", 1024), strings.Repeat("<", 64))
	}
	require.Less(t, len(body), 64<<10)
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, body)
	})
	result, err := adapter.Execute(t.Context(), "ntfy_list_messages", map[string]any{"limit": 20})
	require.NoError(t, err)
	require.Less(t, len(result.Data), maxOutputBytes)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
	require.False(t, result.IsError, result.Data)
	require.Equal(t, true, value["truncations"].(map[string]any)["output_bytes"])
	require.Equal(t, false, value["truncations"].(map[string]any)["response_bytes"])
	require.Equal(t, true, value["incomplete"])
	require.NotEmpty(t, value["messages"])
}

func TestEachSanitizationFlagAndOptionalFields(t *testing.T) {
	body := `{"event":"message","topic":"ops","id":"AbCd1234EfGh","time":1,"title":"` + strings.Repeat("界", 129) + `","message":"` + strings.Repeat("界", 1025) + `","tags":["` + strings.Repeat("界", 65) + `","a","b","c","d","e","f","g","h"],"attachment":{"url":"SECRET"},"expires":2}`
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, body)
	})
	result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
	require.False(t, result.IsError, result.Data)
	flags := value["truncations"].(map[string]any)
	for _, field := range []string{"title", "message", "tags", "tags[0]"} {
		require.Equal(t, true, flags["AbCd1234EfGh."+field])
	}
	item := value["messages"].([]any)[0].(map[string]any)
	require.Len(t, item, 7)
	require.EqualValues(t, 2, item["expires"])
	require.Equal(t, 128, len([]rune(item["title"].(string))))
	require.Equal(t, 1024, len([]rune(item["message"].(string))))
	require.Len(t, item["tags"], 8)
	require.NotContains(t, result.Data, "SECRET")
}

func TestSharedNumericArgumentForms(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "application/x-ndjson") })
	result, _ := resultJSON(t, adapter, "ntfy_list_messages", map[string]any{"since": json.Number(fmt.Sprint(time.Now().Unix() - 60)), "priority": json.Number("3"), "limit": json.Number("2")})
	require.False(t, result.IsError, result.Data)
}

func TestHealthyExactBoundedPollAndCredentialInterfaces(t *testing.T) {
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ntfy/ops/json", r.URL.Path)
		require.Equal(t, url.Values{"poll": {"1"}, "since": {"latest"}}, r.URL.Query())
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, `{"event":"open"}`+"\n")
	})
	require.True(t, adapter.Healthy(t.Context()))
	require.ElementsMatch(t, []string{"base_url", "topic", "ca_cert"}, adapter.PlainTextKeys())
	require.Equal(t, []string{"ca_cert"}, adapter.OptionalKeys())
	require.Contains(t, adapter.Placeholders()["api_token"], "dedicated")
	require.Contains(t, adapter.Placeholders()["api_token"], "full account")
}

func TestJSONByteEstimatorMatchesEncoder(t *testing.T) {
	for _, text := range []string{"", "plain", "界😀", "\"\\\n\r\t\b\f", "\x00\x01<>&\u2028\u2029"} {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			data, err := json.Marshal(text)
			require.NoError(t, err)
			require.Equal(t, len(data), jsonStringBytes(text))
		})
	}
}

func TestEveryCompactionFieldExistsForEmptyAndErrorResponses(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			adapter, _ := testAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				w.WriteHeader(status)
			})
			for _, tool := range adapter.Tools() {
				args := map[string]any{}
				if tool.Name == "ntfy_get_message" {
					args = map[string]any{"id": "AbCd1234EfGh", "since": time.Now().Unix() - 60}
				}
				_, raw := resultJSON(t, adapter, tool.Name, args)
				fields, ok := adapter.CompactSpec(tool.Name)
				require.True(t, ok)
				for _, field := range fields {
					require.Contains(t, raw, field.Spec())
				}
			}
		})
	}
}

func TestReconfigureClosesOldConnectionsAndCanReopen(t *testing.T) {
	idleConnections := make(chan net.Conn, 1)
	closedConnections := make(chan net.Conn, 1)
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
	}))
	provider.Config.ConnState = func(connection net.Conn, state http.ConnState) {
		switch state {
		case http.StateIdle:
			select {
			case idleConnections <- connection:
			default:
			}
		case http.StateClosed:
			select {
			case closedConnections <- connection:
			default:
			}
		}
	}
	provider.StartTLS()
	t.Cleanup(provider.Close)
	adapter := New().(*ntfy)
	t.Cleanup(func() { require.NoError(t, adapter.Close()) })
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	creds := mcp.Credentials{"base_url": provider.URL + "/ntfy", "topic": "ops", "api_token": "test-secret", "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
	require.NoError(t, adapter.Configure(t.Context(), creds))
	require.Empty(t, adapter.snapshot().poll(t.Context(), pollOptions{since: "latest", limit: 1}).Error)
	var originalConnection net.Conn
	select {
	case originalConnection = <-idleConnections:
	case <-time.After(time.Second):
		t.Fatal("original connection did not become idle")
	}
	select {
	case <-closedConnections:
		t.Fatal("original connection closed before reconfiguration")
	default:
	}
	require.NoError(t, adapter.Configure(t.Context(), creds))
	select {
	case connection := <-closedConnections:
		require.Same(t, originalConnection, connection)
	case <-time.After(time.Second):
		t.Fatal("reconfiguration did not close the original idle connection")
	}
	require.True(t, adapter.Healthy(t.Context()))
	require.NoError(t, adapter.Close())
	require.NoError(t, adapter.Configure(t.Context(), creds))
	require.True(t, adapter.Healthy(t.Context()))
}

func TestCloseWithInflightRequests(t *testing.T) {
	const workers = 6
	entered := make(chan struct{}, workers)
	release := make(chan struct{})
	releasePolls := sync.OnceFunc(func() { close(release) })
	defer releasePolls()
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
			t.Error("unexpected additional in-flight request")
		}
		select {
		case <-release:
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = io.WriteString(w, fixture+"\n")
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	type execution struct {
		result *mcp.ToolResult
		err    error
	}
	results := make(chan execution, workers)
	for range workers {
		go func() {
			result, err := adapter.Execute(ctx, "ntfy_list_messages", nil)
			results <- execution{result, err}
		}()
	}
	for range workers {
		select {
		case <-entered:
		case value := <-results:
			t.Fatalf("poll returned before all requests were in flight: result=%+v, err=%v", value.result, value.err)
		case <-ctx.Done():
			t.Fatal("not all requests reached the HTTP handler")
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- adapter.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Close waited for active requests")
	}
	require.Nil(t, adapter.snapshot(), "Close must retire the active configuration")
	require.False(t, adapter.Healthy(ctx))
	afterClose, err := adapter.Execute(ctx, "ntfy_list_messages", nil)
	require.NoError(t, err)
	require.True(t, afterClose.IsError, afterClose.Data)
	require.Equal(t, mcp.ErrNotConfigured.Error(), afterClose.Data)
	releasePolls()
	for range workers {
		select {
		case value := <-results:
			require.NoError(t, value.err)
			require.NotNil(t, value.result)
			require.False(t, value.result.IsError, value.result.Data)
			var raw map[string]any
			require.NoError(t, json.Unmarshal([]byte(value.result.Data), &raw))
			require.Equal(t, "ops", raw["scope"].(map[string]any)["topic"])
			require.Len(t, raw["messages"], 1)
			require.Equal(t, "AbCd1234EfGh", raw["messages"].([]any)[0].(map[string]any)["id"])
		case <-ctx.Done():
			t.Fatal("active request did not finish after releasing the handler")
		}
	}
}

func TestExecuteAndHealthyReuseConnections(t *testing.T) {
	connections := make(chan string, 4)
	adapter, _ := testAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case connections <- r.RemoteAddr:
		default:
			t.Error("unexpected additional connection-reuse request")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, fixture+"\n")
	})
	var originalConnection string
	for _, operation := range []string{"execute", "healthy", "execute", "healthy"} {
		if operation == "healthy" {
			require.True(t, adapter.Healthy(t.Context()))
		} else {
			result, value := resultJSON(t, adapter, "ntfy_list_messages", nil)
			require.False(t, result.IsError, result.Data)
			require.Len(t, value["messages"], 1)
		}
		select {
		case connection := <-connections:
			if originalConnection == "" {
				originalConnection = connection
			}
			require.Equal(t, originalConnection, connection, "%s must reuse the established TLS connection", operation)
		case <-time.After(time.Second):
			t.Fatal("operation did not reach the HTTP handler")
		}
	}
}
