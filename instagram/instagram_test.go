package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "instagram", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"access_token": "IGQVtest123", "user_id": "17841400000"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAccessToken(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"access_token": "", "user_id": "17841400000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_token is required")
}

func TestConfigure_MissingUserID(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"access_token": "IGQVtest123", "user_id": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_id is required")
}

func TestConfigure_CustomAPIVersion(t *testing.T) {
	ig := &instagram{client: &http.Client{}, baseURL: "https://graph.instagram.com", apiVersion: "v21.0"}
	err := ig.Configure(mcp.Credentials{
		"access_token": "IGQVtest",
		"user_id":      "123",
		"api_version":  "v22.0",
	})
	assert.NoError(t, err)
	assert.Equal(t, "v22.0", ig.apiVersion)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveInstagramPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "instagram_", "tool %s missing instagram_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	ig := &instagram{accessToken: "test", userID: "123", client: &http.Client{}, baseURL: "http://localhost", apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("access_token"), "test-token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","username":"testuser"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "test-token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	data, err := ig.get(context.Background(), "/me?fields=id,username")
	require.NoError(t, err)
	assert.Contains(t, string(data), "testuser")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token"}}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "bad-token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	_, err := ig.get(context.Background(), "/me?fields=id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instagram API error (400)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	data, err := ig.doRequest(context.Background(), "DELETE", "/12345", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		msg := body["message"].(map[string]any)
		assert.Equal(t, "hello", msg["text"])
		_, _ = w.Write([]byte(`{"message_id":"m_123"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	data, err := ig.post(context.Background(), "/123/messages", map[string]any{
		"recipient": map[string]string{"id": "456"},
		"message":   map[string]any{"text": "hello"},
	})
	require.NoError(t, err)
	assert.Contains(t, string(data), "m_123")
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '&')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestUid(t *testing.T) {
	ig := &instagram{userID: "default-user"}

	assert.Equal(t, "default-user", ig.uid(map[string]any{}))
	assert.Equal(t, "custom-user", ig.uid(map[string]any{"user_id": "custom-user"}))
}

// --- handler integration tests ---

func TestGetProfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123")
		assert.Contains(t, r.URL.Query().Get("fields"), "id")
		_, _ = w.Write([]byte(`{"id":"123","username":"testuser","media_count":42}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_get_profile", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "testuser")
}

func TestDiscoverUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.RawQuery, "business_discovery")
		_, _ = w.Write([]byte(`{"business_discovery":{"username":"creator","followers_count":50000}}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_discover_user", map[string]any{
		"username": "creator",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "creator")
	assert.Contains(t, result.Data, "50000")
}

func TestListMedia(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123/media")
		_, _ = w.Write([]byte(`{"data":[{"id":"m1","caption":"Hello"}]}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_list_media", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestSendMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123/messages")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		recipient := body["recipient"].(map[string]any)
		assert.Equal(t, "456", recipient["id"])
		msg := body["message"].(map[string]any)
		assert.Equal(t, "Hi creator!", msg["text"])
		_, _ = w.Write([]byte(`{"recipient_id":"456","message_id":"m_abc"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_send_message", map[string]any{
		"recipient_id": "456",
		"message":      "Hi creator!",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "m_abc")
}

func TestListConversations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123/conversations")
		_, _ = w.Write([]byte(`{"data":[{"id":"conv1"}]}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_list_conversations", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "conv1")
}

func TestListComments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/m1/comments")
		_, _ = w.Write([]byte(`{"data":[{"id":"c1","text":"Great post!"}]}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_list_comments", map[string]any{
		"media_id": "m1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Great post!")
}

func TestGetMediaInsights(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/m1/insights")
		assert.Contains(t, r.URL.RawQuery, "metric=impressions")
		_, _ = w.Write([]byte(`{"data":[{"name":"impressions","values":[{"value":1500}]}]}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_get_media_insights", map[string]any{
		"media_id": "m1",
		"metric":   "impressions",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "1500")
}

func TestSearchHashtag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/ig_hashtag_search")
		assert.Contains(t, r.URL.RawQuery, "q=travel")
		_, _ = w.Write([]byte(`{"data":[{"id":"17843853986012965"}]}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_search_hashtag", map[string]any{
		"q": "travel",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "17843853986012965")
}

func TestDeleteComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_delete_comment", map[string]any{
		"comment_id": "c1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestCreateMediaContainer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123/media")
		assert.Contains(t, r.URL.RawQuery, "image_url=")
		_, _ = w.Write([]byte(`{"id":"container_123"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_create_media_container", map[string]any{
		"image_url": "https://example.com/photo.jpg",
		"caption":   "Beautiful sunset",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "container_123")
}

func TestPublishMedia(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v21.0/123/media_publish")
		assert.Contains(t, r.URL.RawQuery, "creation_id=container_123")
		_, _ = w.Write([]byte(`{"id":"media_456"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_publish_media", map[string]any{
		"container_id": "container_123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "media_456")
}

func TestUserIDOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/v21.0/999")
		_, _ = w.Write([]byte(`{"id":"999"}`))
	}))
	defer ts.Close()

	ig := &instagram{accessToken: "token", userID: "123", client: ts.Client(), baseURL: ts.URL, apiVersion: "v21.0"}
	result, err := ig.Execute(context.Background(), "instagram_get_profile", map[string]any{
		"user_id": "999",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
