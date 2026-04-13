package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, ModelName("gemma4:e2b"), req.Model)
		assert.False(t, req.Stream)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, ChatRole("user"), req.Messages[0].Role)

		w.Write([]byte(`{"model":"gemma4:e2b","message":{"role":"assistant","content":"Hello!","thinking":"I should greet the user."},"done":true,"done_reason":"stop","total_duration":5000000,"eval_count":3}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := chat(context.Background(), o, map[string]any{
		"model": "gemma4:e2b",
		"messages": []any{
			map[string]any{"role": "user", "content": "Hi"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp chatResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Equal(t, "Hello!", resp.Message.Content)
	assert.Equal(t, "I should greet the user.", resp.Message.Thinking)
	assert.Equal(t, DoneReason("stop"), resp.DoneReason)
	assert.Equal(t, Nanoseconds(5000000), resp.TotalDuration)
}

func TestChat_MissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"model is required"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := chat(context.Background(), o, map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "model")
}

func TestChat_MissingMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server when messages is missing")
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := chat(context.Background(), o, map[string]any{
		"model": "gemma4:e2b",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "messages")
}

func TestGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		var req generateRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, ModelName("gemma4:e2b"), req.Model)
		assert.Equal(t, "Say hello", req.Prompt)
		assert.False(t, req.Stream)

		w.Write([]byte(`{"model":"gemma4:e2b","response":"Hello!","done":true,"done_reason":"stop","total_duration":300000000,"eval_count":3}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := generate(context.Background(), o, map[string]any{
		"model":  "gemma4:e2b",
		"prompt": "Say hello",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp generateResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Equal(t, "Hello!", resp.Response)
	assert.Equal(t, DoneReason("stop"), resp.DoneReason)
	assert.Equal(t, Nanoseconds(300000000), resp.TotalDuration)
}

func TestEmbed_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "nomic-embed-text", body["model"])
		assert.Equal(t, false, body["stream"])

		w.Write([]byte(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3]],"total_duration":1000000,"load_duration":500000,"prompt_eval_count":5}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := embed(context.Background(), o, map[string]any{
		"model": "nomic-embed-text",
		"input": "hello world",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp embedResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Equal(t, ModelName("nomic-embed-text"), resp.Model)
	require.Len(t, resp.Embeddings, 1)
	assert.Equal(t, Embedding{0.1, 0.2, 0.3}, resp.Embeddings[0])
	assert.Equal(t, Nanoseconds(1000000), resp.TotalDuration)
}

func TestEmbed_UnsupportedModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"this model does not support embeddings"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := embed(context.Background(), o, map[string]any{
		"model": "gemma4:e2b",
		"input": "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "does not support embeddings")
}

func TestEmbed_MissingInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server when input is missing")
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := embed(context.Background(), o, map[string]any{
		"model": "nomic-embed-text",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "input")
}
