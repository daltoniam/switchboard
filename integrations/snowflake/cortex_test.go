package snowflake

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCortexAnalyst_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/cortex/analyst/message", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req analystRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "What is the total revenue?", req.Messages[0].Content[0].Text)
		assert.Equal(t, "@DB.SCH.STG/model.yaml", req.SemanticModelFile)

		resp := analystResponse{
			RequestID: "req-123",
			Message: analystMessage{
				Role: "analyst",
				Content: []analystContent{
					{Type: "text", Text: "Here is the total revenue query."},
					{Type: "sql", Statement: "SELECT SUM(revenue) FROM sales"},
					{Type: "suggestions", Suggestions: []string{"By region?", "By quarter?"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "test-token"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":            "What is the total revenue?",
		"semantic_model_file": "@DB.SCH.STG/model.yaml",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "SELECT SUM(revenue) FROM sales")
	assert.Contains(t, result.Data, "Here is the total revenue query.")
	assert.Contains(t, result.Data, "By region?")
}

func TestCortexAnalyst_InlineModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req analystRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Empty(t, req.SemanticModelFile)
		assert.Contains(t, req.SemanticModel, "tables:")

		resp := analystResponse{
			RequestID: "req-456",
			Message: analystMessage{
				Role:    "analyst",
				Content: []analystContent{{Type: "text", Text: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "test-token"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":       "How many users?",
		"semantic_model": "tables:\n  - name: users",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCortexAnalyst_MissingQuestion(t *testing.T) {
	s := &snowflake{client: &http.Client{}}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"semantic_model_file": "@DB.SCH.STG/model.yaml",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "question is required")
}

func TestCortexAnalyst_MissingModel(t *testing.T) {
	s := &snowflake{client: &http.Client{}}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question": "How many users?",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "semantic_view, semantic_model_file, or semantic_model is required")
}

func TestCortexAnalyst_SemanticView(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req analystRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "MY_DB.MY_SCHEMA.MY_VIEW", req.SemanticView)
		assert.Empty(t, req.SemanticModelFile)
		assert.Empty(t, req.SemanticModel)

		resp := analystResponse{
			RequestID: "req-789",
			Message: analystMessage{
				Role:    "analyst",
				Content: []analystContent{{Type: "text", Text: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "test-token"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":      "How many users?",
		"semantic_view": "MY_DB.MY_SCHEMA.MY_VIEW",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCortexAnalyst_DefaultSemanticView(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req analystRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "DB.SCH.DEFAULT_VIEW", req.SemanticView)

		resp := analystResponse{
			RequestID: "req-default",
			Message: analystMessage{
				Role:    "analyst",
				Content: []analystContent{{Type: "text", Text: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "test-token", semanticView: "DB.SCH.DEFAULT_VIEW"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question": "How many users?",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCortexAnalyst_ExplicitOverridesDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req analystRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "DB.SCH.EXPLICIT_VIEW", req.SemanticView, "explicit should override default")

		resp := analystResponse{
			RequestID: "req-override",
			Message: analystMessage{
				Role:    "analyst",
				Content: []analystContent{{Type: "text", Text: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "test-token", semanticView: "DB.SCH.DEFAULT_VIEW"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":      "How many users?",
		"semantic_view": "DB.SCH.EXPLICIT_VIEW",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCortexAnalyst_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "bad-token"}
	result, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":            "test",
		"semantic_model_file": "@DB.SCH.STG/model.yaml",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unauthorized")
}

func TestCortexAnalyst_RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := &snowflake{client: ts.Client(), baseURL: ts.URL, token: "tok"}
	_, err := cortexAnalyst(context.Background(), s, map[string]any{
		"question":            "test",
		"semantic_model_file": "@DB.SCH.STG/model.yaml",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}
