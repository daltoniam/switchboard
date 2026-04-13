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

func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":[{"name":"gemma4:e2b","model":"gemma4:e2b","size":8140152828,"details":{"family":"gemma4","parameter_size":"5.1B","quantization_level":"Q8_0"}}]}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := listModels(context.Background(), o, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp tagsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	require.Len(t, resp.Models, 1)
	assert.Equal(t, ModelName("gemma4:e2b"), resp.Models[0].Name)
	assert.Equal(t, ModelFamily("gemma4"), resp.Models[0].Details.Family)
}

func TestShowModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/show", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "gemma4:e2b", body["model"])
		w.Write([]byte(`{"details":{"family":"gemma4","parameter_size":"5.1B","quantization_level":"Q8_0","format":"gguf"},"capabilities":["completion","vision"],"parameters":"temperature 1\ntop_k 64","template":"{{ .Prompt }}","license":"Apache License 2.0","modified_at":"2026-04-11"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := showModel(context.Background(), o, map[string]any{"model": "gemma4:e2b"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp showResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Equal(t, ModelFamily("gemma4"), resp.Details.Family)
	assert.Equal(t, []Capability{"completion", "vision"}, resp.Capabilities)
}

func TestShowModel_MissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"model is required"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := showModel(context.Background(), o, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "model")
}

func TestPullModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/pull", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "llama3.2", body["model"])
		assert.Equal(t, false, body["stream"])
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := pullModel(context.Background(), o, map[string]any{"model": "llama3.2"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestDeleteModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/delete", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := deleteModel(context.Background(), o, map[string]any{"model": "old-model"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestCopyModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/copy", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body copyRequest
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, ModelName("gemma4:e2b"), body.Source)
		assert.Equal(t, ModelName("gemma4-backup"), body.Destination)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := copyModel(context.Background(), o, map[string]any{"source": "gemma4:e2b", "destination": "gemma4-backup"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/create", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "my-model", body["model"])
		assert.Equal(t, false, body["stream"])
		assert.Equal(t, "gemma3", body["from"])
		assert.Equal(t, "You are a helpful assistant.", body["system"])
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := createModel(context.Background(), o, map[string]any{
		"model":  "my-model",
		"from":   "gemma3",
		"system": "You are a helpful assistant.",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestListRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ps", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"models":[{"name":"gemma4:e2b","size":8140152828,"size_vram":8140152828,"context_length":131072,"expires_at":"2026-04-12T07:00:00Z","details":{"family":"gemma4","parameter_size":"5.1B"}}]}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := listRunning(context.Background(), o, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp psResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	require.Len(t, resp.Models, 1)
	assert.Equal(t, ModelName("gemma4:e2b"), resp.Models[0].Name)
	assert.Equal(t, 131072, resp.Models[0].ContextLength)
}

func TestGetVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/version", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"version":"0.20.5"}`))
	}))
	defer srv.Close()

	o := New().(*ollama)
	_ = o.Configure(context.Background(), mcp.Credentials{"base_url": srv.URL})
	result, err := getVersion(context.Background(), o, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp versionResponse
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Equal(t, "0.20.5", resp.Version)
}
