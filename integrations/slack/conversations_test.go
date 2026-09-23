package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConversationsClient(server *httptest.Server) *slackIntegration {
	return &slackIntegration{
		clients: map[string]*slack.Client{"T1": slack.New("xoxb-test", slack.OptionAPIURL(server.URL+"/"))},
		store:   &tokenStore{workspaces: map[string]*workspace{"T1": {TeamID: "T1"}}, defaultTeamID: "T1"},
	}
}

func TestListConversations_Pagination(t *testing.T) {
	for _, tc := range []struct {
		name, cursor, expectedCursor, nextCursor string
	}{
		{"first page", "", "", "page-two"},
		{"second page", "page-two", "page-two", "page-three"},
		{"last page", "page-three", "page-three", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/conversations.list", r.URL.Path)
				require.NoError(t, r.ParseForm())
				assert.Equal(t, tc.expectedCursor, r.Form.Get("cursor"))
				assert.Equal(t, "public_channel,private_channel", r.Form.Get("types"))
				_, _ = fmt.Fprintf(w, `{"ok":true,"channels":[{"id":"C1","name":"general"}],"response_metadata":{"next_cursor":%q}}`, tc.nextCursor)
			}))
			defer server.Close()
			result, err := listConversations(t.Context(), testConversationsClient(server), map[string]any{"cursor": tc.cursor})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(result.Data), &body))
			assert.Equal(t, tc.nextCursor, body["next_cursor"])
			assert.EqualValues(t, 1, body["count"])
		})
	}
}

func TestListConversations_RepeatedCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"channels":[{"id":"C1"}],"response_metadata":{"next_cursor":"same"}}`))
	}))
	defer server.Close()
	result, err := listConversations(t.Context(), testConversationsClient(server), map[string]any{"cursor": "same"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &body))
	assert.Equal(t, "", body["next_cursor"])
	assert.EqualValues(t, 1, body["count"])
	assert.Equal(t, "C1", body["conversations"].([]any)[0].(map[string]any)["id"])
	assert.Contains(t, body["warning"], "slack_token_status")
	assert.Contains(t, body["warning"], "public_channel")
}

func TestProbeGrantedScopes(t *testing.T) {
	for _, tc := range []struct {
		name, response, header string
		available              bool
	}{
		{"granted", `{"ok":true}`, "channels:read, groups:history", true},
		{"header absent", `{"ok":true}`, "", false},
		{"auth rejected", `{"ok":false,"error":"invalid_auth"}`, "channels:read", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/auth.test", r.URL.Path)
				require.NoError(t, r.ParseForm())
				assert.Equal(t, "xoxb-test", r.Form.Get("token"))
				assert.Equal(t, "d=xoxd-test", r.Header.Get("Cookie"))
				w.Header().Set("X-Oauth-Scopes", tc.header)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			client := &http.Client{Transport: &cookieTransport{cookie: "xoxd-test", inner: http.DefaultTransport}}
			scopes, available := probeGrantedScopes(t.Context(), &workspace{Token: "xoxb-test"}, client, server.URL+"/auth.test")
			assert.Equal(t, tc.available, available)
			if tc.available {
				assert.Equal(t, []string{"channels:read", "groups:history"}, scopes)
			} else {
				assert.Empty(t, scopes)
			}
		})
	}
}

func TestTokenStatus_ProbeBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	store := &tokenStore{workspaces: map[string]*workspace{
		"T1": {TeamID: "T1", Token: "xoxb-one"},
		"T2": {TeamID: "T2", Token: "xoxb-two"},
		"T3": {TeamID: "T3", Token: "xoxb-three"},
	}}
	start := time.Now()
	result, err := tokenStatusWithEndpoint(t.Context(), &slackIntegration{store: store}, server.URL+"/auth.test")
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Less(t, time.Since(start), 4*time.Second)
	var body struct {
		Workspaces []struct {
			ScopesAvailable bool `json:"scopes_available"`
		} `json:"workspaces"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Data), &body))
	require.Len(t, body.Workspaces, 3)
	for _, ws := range body.Workspaces {
		assert.False(t, ws.ScopesAvailable)
	}
}

func TestConversationReads_ChannelNotFound(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		call       func(*testing.T, *slackIntegration) (string, error)
	}{
		{"history", "/conversations.history", func(t *testing.T, s *slackIntegration) (string, error) {
			result, err := conversationsHistory(t.Context(), s, map[string]any{"channel_id": "C1"})
			if err != nil {
				return "", err
			}
			return result.Data, nil
		}},
		{"replies", "/conversations.replies", func(t *testing.T, s *slackIntegration) (string, error) {
			result, err := getThread(t.Context(), s, map[string]any{"channel_id": "C1", "thread_ts": "123.456"})
			if err != nil {
				return "", err
			}
			return result.Data, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.path, r.URL.Path)
				_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
			}))
			defer server.Close()
			message, err := tc.call(t, testConversationsClient(server))
			require.NoError(t, err)
			assert.Contains(t, message, "channel_not_found")
			assert.Contains(t, message, "history")
			assert.Contains(t, message, "cookie")
			assert.Contains(t, message, "slack_token_status")
		})
	}
}
