package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiNtfyMessageID = "NtfyBackup12"

func uiNtfyAssertSanitized(t *testing.T, text string) {
	t.Helper()
	require.Less(t, len(text), 40*1024)
	for _, forbidden := range []string{"SECRET_SENTINEL", "ui-ntfy-token", "live-seed-password", "TITLE_OVERFLOW", "MESSAGE_OVERFLOW", "TAG_OVERFLOW"} {
		require.NotContains(t, text, forbidden)
	}
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &value))
	for _, key := range []string{"scope", "window", "returned_count", "upstream_truncated", "adapter_truncated", "incomplete", "truncations", "status", "caveat"} {
		require.Contains(t, value, key)
	}
	messages, ok := value["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)
	require.LessOrEqual(t, len(messages), 20)
	require.EqualValues(t, len(messages), value["returned_count"])
	require.Equal(t, map[string]any{"topic": "ops_notifications"}, value["scope"])
	for _, item := range messages {
		message, ok := item.(map[string]any)
		require.True(t, ok)
		for key := range message {
			require.Contains(t, []string{"id", "time", "priority", "tags", "title", "message", "expires"}, key)
		}
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				require.NotContains(t, []string{"click", "icon", "actions", "attachment", "sender", "client", "encoding", "content_type", "api_token", "authorization", "unknown_metadata"}, strings.ToLower(key))
				walk(nested)
			}
			if _, message := typed["id"]; message {
				for key, limit := range map[string]int{"title": 128, "message": 1024} {
					field, ok := typed[key].(string)
					require.True(t, ok, "%s must be a string", key)
					require.LessOrEqual(t, len([]rune(field)), limit)
				}
				tags, ok := typed["tags"].([]any)
				require.True(t, ok)
				require.LessOrEqual(t, len(tags), 8)
				for _, tag := range tags {
					text, ok := tag.(string)
					require.True(t, ok)
					require.LessOrEqual(t, len([]rune(text)), 64)
				}
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
}

func uiNtfyMCP(t *testing.T, app *uiApp, pollCutoffs func() []string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ntfy-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	catalog, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"limit": 0}})
	require.NoError(t, err)
	require.False(t, catalog.IsError)
	catalogText, ok := catalog.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	var catalogResponse struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(catalogText.Text), &catalogResponse))
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "ntfy private notification inbox recent backup deployment alerts", "integration": "ntfy", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		require.Zero(t, catalogResponse.Total)
		return nil
	}
	require.Equal(t, 2, catalogResponse.Total)
	require.ElementsMatch(t, []string{"ntfy_list_messages", "ntfy_get_message"}, names)
	since := time.Now().Add(-30 * time.Minute).Unix()
	var discoveredID string
	for _, c := range []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"latest", "ntfy_list_messages", map[string]any{}},
		{"filtered", "ntfy_list_messages", map[string]any{"since": since, "priority": 4, "tags": "backup", "limit": 1}},
		{"exact", "ntfy_get_message", map[string]any{"since": since}},
	} {
		if c.tool == "ntfy_get_message" {
			require.NotEmpty(t, discoveredID, "discover the exact message ID through the list tool before lookup")
			c.args["id"] = discoveredID
		}
		for _, raw := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/raw=%t", c.tool, c.name, raw), func(t *testing.T) {
				arguments := map[string]any{"tool_name": c.tool, "arguments": c.args}
				if raw {
					args, err := json.Marshal(c.args)
					require.NoError(t, err)
					arguments = map[string]any{"script": fmt.Sprintf("JSON.stringify(api.call(%q, %s));", c.tool, args)}
				}
				before := len(pollCutoffs())
				result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: arguments})
				require.NoError(t, err)
				require.False(t, result.IsError)
				if len(c.args) > 0 {
					require.Equal(t, []string{strconv.FormatInt(since, 10)}, pollCutoffs()[before:], "each replay poll must forward the requested cutoff unchanged")
				}
				text, ok := result.Content[0].(*mcpsdk.TextContent)
				require.True(t, ok)
				data := text.Text
				if raw {
					encoded, err := json.Marshal(liveDecodeScript(t, c.tool, data))
					require.NoError(t, err)
					data = string(encoded)
				}
				require.Contains(t, data, uiNtfyMessageID)
				require.Contains(t, data, "Backup completed")
				require.Contains(t, data, "Snapshot saved")
				require.Contains(t, data, "ops_notifications")
				require.Contains(t, data, "truncat")
				uiNtfyAssertSanitized(t, data)
				var value map[string]any
				require.NoError(t, json.Unmarshal([]byte(data), &value))
				messages := value["messages"].([]any)
				require.Len(t, messages, 1)
				require.EqualValues(t, 1, value["returned_count"])
				message := messages[0].(map[string]any)
				require.Equal(t, uiNtfyMessageID, message["id"])
				require.EqualValues(t, 4, message["priority"])
				require.Contains(t, message["tags"], "backup")
				if c.tool == "ntfy_list_messages" {
					discoveredID = message["id"].(string)
				}
				require.Equal(t, "incomplete", value["status"])
				require.Equal(t, true, value["adapter_truncated"])
				require.Equal(t, false, value["upstream_truncated"])
				require.Equal(t, true, value["incomplete"])
				window := value["window"].(map[string]any)
				if len(c.args) == 0 {
					require.Equal(t, "latest", window["since"])
				} else {
					require.Equal(t, strconv.FormatInt(since, 10), window["since"])
				}
				truncations := value["truncations"].(map[string]any)
				require.Equal(t, c.name == "filtered", truncations["message_limit"])
				for _, field := range []string{"title", "message", "tags", "tags[1]"} {
					require.Equal(t, true, truncations[uiNtfyMessageID+"."+field])
				}
			})
		}
	}
	return names
}

func TestUINtfy(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	var mu sync.Mutex
	var cutoffs []string
	pollCutoffs := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), cutoffs...)
	}
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ui-ntfy-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/ops_notifications/json" || query.Get("poll") != "1" || query.Get("since") == "" {
			t.Errorf("ntfy must use finite polling on its single configured topic: %s %s", r.Method, r.URL.Path)
			http.Error(w, "invalid poll", http.StatusBadRequest)
			return
		}
		for key := range query {
			if key != "poll" && key != "since" && key != "id" && key != "priority" && key != "tags" {
				t.Errorf("unexpected ntfy query key: %s", key)
			}
		}
		if query.Get("since") != "latest" {
			mu.Lock()
			cutoffs = append(cutoffs, query.Get("since"))
			mu.Unlock()
			since, err := strconv.ParseInt(query.Get("since"), 10, 64)
			if err != nil || since < time.Now().Add(-time.Hour).Unix() || since > time.Now().Unix() {
				t.Errorf("ntfy since must be latest or within the last hour")
				http.Error(w, "invalid since", http.StatusBadRequest)
				return
			}
		}
		filtered := query.Get("since") != "latest" && query.Get("id") == ""
		if filtered && (query.Get("priority") != "4" || query.Get("tags") != "backup") {
			t.Errorf("filtered ntfy poll must forward exact priority=4 and tags=backup")
			http.Error(w, "missing or invalid filters", http.StatusBadRequest)
			return
		}
		if !filtered && (query.Has("priority") || query.Has("tags")) {
			t.Errorf("unfiltered ntfy poll must not send priority or tags")
			http.Error(w, "unexpected filters", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		if query.Get("id") != "" && query.Get("id") != uiNtfyMessageID {
			return
		}
		message := map[string]any{
			"id": uiNtfyMessageID, "time": time.Now().Add(-time.Minute).Unix(), "expires": time.Now().Add(time.Hour).Unix(),
			"event": "message", "topic": "ops_notifications", "priority": 4,
			"title":   "Backup completed " + strings.Repeat("T", 128) + "TITLE_OVERFLOW",
			"message": "Snapshot saved " + strings.Repeat("m", 1024) + "MESSAGE_OVERFLOW",
			"tags":    []string{"backup", strings.Repeat("t", 64) + "TAG_OVERFLOW", "three", "four", "five", "six", "seven", "eight", "TAG_OVERFLOW"},
			"click":   "https://SECRET_SENTINEL.invalid/click", "icon": "https://SECRET_SENTINEL.invalid/icon",
			"actions":    []map[string]any{{"action": "http", "url": "https://SECRET_SENTINEL.invalid", "headers": map[string]string{"Authorization": "SECRET_SENTINEL"}}},
			"attachment": map[string]string{"url": "https://SECRET_SENTINEL.invalid/attachment"},
			"sender":     "SECRET_SENTINEL", "client": "SECRET_SENTINEL", "unknown_metadata": "SECRET_SENTINEL",
		}
		encoder := json.NewEncoder(w)
		if filtered {
			message["id"] = "NtfyBackup11"
			message["time"] = time.Now().Add(-2 * time.Minute).Unix()
			if err := encoder.Encode(message); err != nil {
				t.Errorf("encode older ntfy fixture: %v", err)
			}
			message["id"] = uiNtfyMessageID
			message["time"] = time.Now().Add(-time.Minute).Unix()
		}
		if err := encoder.Encode(message); err != nil {
			t.Errorf("encode ntfy fixture: %v", err)
		}
	}))
	t.Cleanup(provider.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/ntfy"]`)
	count, err := card.Count()
	require.NoError(t, err)
	require.Equal(t, 1, count, "fresh installs must register the ntfy integration")
	uiText(t, card, "Disabled")
	require.Empty(t, uiNtfyMCP(t, app, pollCutoffs))
	require.NoError(t, card.Click())
	require.Equal(t, app.URL+"/integrations/ntfy", page.URL())
	form := page.Locator("#integration-configuration")
	fieldCount, err := form.Locator(`input[id^="cred_"]`).Count()
	require.NoError(t, err)
	require.Equal(t, 4, fieldCount)
	for _, field := range []struct{ key, inputType, hint string }{
		{"base_url", "text", "https"}, {"topic", "text", "topic"},
		{"api_token", "password", "dedicated"}, {"ca_cert", "text", "pem"},
	} {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, strings.ToLower(placeholder), field.hint)
	}
	uiText(t, form.Locator(`label[for="cred_ca_cert"]`), "optional")
	uiText(t, page.Locator("body"), "Available Tools (2)")
	credentials := map[string]string{"base_url": provider.URL, "topic": "ops_notifications", "api_token": "invalid-token", "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
	for key, value := range credentials {
		require.NoError(t, form.Locator("#cred_"+key).Fill(value))
	}
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill("ui-ntfy-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiNtfyMCP(t, app, pollCutoffs), 2)
	uiCapture(t, page, "ntfy-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "ntfy-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/ntfy")
	uiText(t, page.Locator(".badge").First(), "Connected")
	for key, expected := range map[string]string{"base_url": provider.URL, "topic": "ops_notifications", "api_token": "ui-ntfy-token", "ca_cert": credentials["ca_cert"]} {
		value, err := form.Locator("#cred_" + key).InputValue()
		require.NoError(t, err)
		require.Equal(t, expected, value)
	}
	require.Len(t, uiNtfyMCP(t, app, pollCutoffs), 2)
	require.NoError(t, form.Locator("#cred_api_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill("ui-ntfy-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiNtfyMCP(t, app, pollCutoffs))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
