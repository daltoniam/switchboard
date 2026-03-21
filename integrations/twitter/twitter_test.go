package twitter

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
	assert.Equal(t, "twitter", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"id":"12345","name":"Test","username":"test"}}`))
	}))
	defer ts.Close()

	tw := &twitter{client: ts.Client(), baseURL: ts.URL}
	err := tw.Configure(context.Background(), mcp.Credentials{"bearer_token": "test-token"})
	assert.NoError(t, err)
	assert.Equal(t, "12345", tw.userID)
}

func TestConfigure_MissingBearerToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"bearer_token": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bearer_token is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"id":"12345"}}`))
	}))
	defer ts.Close()

	tw := &twitter{client: ts.Client(), baseURL: "https://api.x.com/2"}
	err := tw.Configure(context.Background(), mcp.Credentials{
		"bearer_token": "test",
		"base_url":     ts.URL + "/",
	})
	assert.NoError(t, err)
	assert.Equal(t, ts.URL, tw.baseURL)
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

func TestTools_AllHaveTwitterPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "twitter_", "tool %s missing twitter_ prefix", tool.Name)
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
	tw := &twitter{bearerToken: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := tw.Execute(context.Background(), "twitter_nonexistent", nil)
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
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"12345"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := tw.get(context.Background(), "/users/me")
	require.NoError(t, err)
	assert.Contains(t, string(data), "12345")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"title":"Unauthorized","detail":"Invalid token"}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := tw.get(context.Background(), "/users/me")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "twitter API error (401)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := tw.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"title":"Too Many Requests"}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	_, err := tw.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body)
		_, _ = w.Write([]byte(`{"data":{"id":"tweet-1"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := tw.post(context.Background(), "/tweets", map[string]string{"text": "hello"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "tweet-1")
}

func TestPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"data":{"hidden":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := tw.put(context.Background(), "/test", map[string]any{"hidden": true})
	require.NoError(t, err)
	assert.Contains(t, string(data), "hidden")
}

// --- Result helper tests ---

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

// --- Argument helper tests ---

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
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestMe(t *testing.T) {
	tw := &twitter{userID: "12345"}
	assert.Equal(t, "12345", tw.me())
}

// --- Handler integration tests ---

func TestGetTweet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/tweets/123456")
		_, _ = w.Write([]byte(`{"data":{"id":"123456","text":"Hello world"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_tweet", map[string]any{"id": "123456"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello world")
}

func TestCreateTweet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/tweets", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Hello from Switchboard!", body["text"])
		_, _ = w.Write([]byte(`{"data":{"id":"new-tweet-1","text":"Hello from Switchboard!"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_create_tweet", map[string]any{
		"text": "Hello from Switchboard!",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-tweet-1")
}

func TestCreateTweet_WithReply(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		reply := body["reply"].(map[string]any)
		assert.Equal(t, "orig-tweet", reply["in_reply_to_tweet_id"])
		_, _ = w.Write([]byte(`{"data":{"id":"reply-1"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_create_tweet", map[string]any{
		"text":     "Great point!",
		"reply_to": "orig-tweet",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteTweet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/tweets/123")
		_, _ = w.Write([]byte(`{"data":{"deleted":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_delete_tweet", map[string]any{"id": "123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")
}

func TestSearchRecent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/tweets/search/recent")
		assert.Equal(t, "golang", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"data":[{"id":"1","text":"Go is great"}],"meta":{"result_count":1}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_search_recent", map[string]any{
		"query": "golang",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Go is great")
}

func TestGetUserByUsername(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/by/username/elonmusk")
		_, _ = w.Write([]byte(`{"data":{"id":"44196397","name":"Elon Musk","username":"elonmusk"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_user_by_username", map[string]any{
		"username": "elonmusk",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "44196397")
}

func TestGetFollowing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/12345/following")
		_, _ = w.Write([]byte(`{"data":[{"id":"1","username":"friend"}],"meta":{"result_count":1}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_following", map[string]any{
		"user_id": "12345",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "friend")
}

func TestFollowUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/following")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "target-user", body["target_user_id"])
		_, _ = w.Write([]byte(`{"data":{"following":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_follow_user", map[string]any{
		"target_user_id": "target-user",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "following")
}

func TestLikeTweet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/likes")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "tweet-999", body["tweet_id"])
		_, _ = w.Write([]byte(`{"data":{"liked":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_like_tweet", map[string]any{
		"tweet_id": "tweet-999",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "liked")
}

func TestGetBookmarks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/bookmarks")
		_, _ = w.Write([]byte(`{"data":[{"id":"bm-1","text":"Bookmarked tweet"}]}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_get_bookmarks", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Bookmarked tweet")
}

func TestGetList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/lists/list-1")
		_, _ = w.Write([]byte(`{"data":{"id":"list-1","name":"Tech News"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_list", map[string]any{"id": "list-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Tech News")
}

func TestCreateList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/lists", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "My List", body["name"])
		_, _ = w.Write([]byte(`{"data":{"id":"new-list","name":"My List"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_create_list", map[string]any{
		"name": "My List",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-list")
}

func TestSendDM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/dm_conversations/with/user-456/messages")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Hello!", body["text"])
		_, _ = w.Write([]byte(`{"data":{"dm_event_id":"dm-1"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_send_dm", map[string]any{
		"participant_id": "user-456",
		"text":           "Hello!",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "dm-1")
}

func TestGetSpace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/spaces/space-1")
		_, _ = w.Write([]byte(`{"data":{"id":"space-1","title":"Tech Talk","state":"live"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_space", map[string]any{"id": "space-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Tech Talk")
}

func TestGetUsage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/usage/tweets")
		_, _ = w.Write([]byte(`{"data":[{"date":"2024-01-01","usage":[{"tweets":100}]}]}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_usage", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "2024-01-01")
}

func TestGetHomeTimeline(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/timelines/reverse_chronological")
		_, _ = w.Write([]byte(`{"data":[{"id":"1","text":"Timeline tweet"}]}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_get_home_timeline", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Timeline tweet")
}

func TestBlockUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/blocking")
		_, _ = w.Write([]byte(`{"data":{"blocking":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_block_user", map[string]any{
		"target_user_id": "bad-user",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "blocking")
}

func TestRetweet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me123/retweets")
		_, _ = w.Write([]byte(`{"data":{"retweeted":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL, userID: "me123"}
	result, err := tw.Execute(context.Background(), "twitter_retweet", map[string]any{
		"tweet_id": "tweet-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "retweeted")
}

func TestHideReply(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/tweets/reply-1/hidden")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["hidden"])
		_, _ = w.Write([]byte(`{"data":{"hidden":true}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_hide_reply", map[string]any{"id": "reply-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/users/me")
		_, _ = w.Write([]byte(`{"data":{"id":"me123","username":"myuser"}}`))
	}))
	defer ts.Close()

	tw := &twitter{bearerToken: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := tw.Execute(context.Background(), "twitter_get_me", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "myuser")
}
