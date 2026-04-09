package snowflake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

// --- Cortex Analyst types ---

type analystMessage struct {
	Role    string           `json:"role"`
	Content []analystContent `json:"content"`
}

type analystContent struct {
	Type        string   `json:"type"`
	Text        string   `json:"text,omitempty"`
	Statement   string   `json:"statement,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type analystRequest struct {
	Messages          []analystMessage `json:"messages"`
	SemanticModelFile string           `json:"semantic_model_file,omitempty"`
	SemanticModel     string           `json:"semantic_model,omitempty"`
}

type analystResponse struct {
	RequestID string         `json:"request_id"`
	Message   analystMessage `json:"message"`
}

// cortexAnalyst sends a question to the Cortex Analyst semantic layer and
// returns the generated SQL, explanation, and any follow-up suggestions.
func cortexAnalyst(ctx context.Context, s *snowflake, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	question := r.Str("question")
	modelFile := r.Str("semantic_model_file")
	model := r.Str("semantic_model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if question == "" {
		return mcp.ErrResult(fmt.Errorf("question is required"))
	}
	if modelFile == "" && model == "" {
		return mcp.ErrResult(fmt.Errorf("semantic_model_file or semantic_model is required"))
	}

	req := analystRequest{
		Messages: []analystMessage{
			{
				Role: "user",
				Content: []analystContent{
					{Type: "text", Text: question},
				},
			},
		},
		SemanticModelFile: modelFile,
		SemanticModel:     model,
	}

	resp, err := s.doAnalystRequest(ctx, &req)
	if err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(resp)
}

func (s *snowflake) doAnalystRequest(ctx context.Context, req *analystRequest) (*analystResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("snowflake: marshal analyst request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/v2/cortex/analyst/message", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("snowflake: create analyst request: %w", err)
	}

	token, err := s.getToken()
	if err != nil {
		return nil, fmt.Errorf("snowflake: get auth token: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Switchboard/1.0")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("snowflake: analyst request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("snowflake: read analyst response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		var result analystResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("snowflake: parse analyst response: %w", err)
		}
		return &result, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("snowflake: unauthorized (401) — check token")
	case resp.StatusCode == http.StatusTooManyRequests:
		retryAfter := mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("snowflake: analyst rate limited (429)"),
			RetryAfter: retryAfter,
		}
	case resp.StatusCode >= 500:
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("snowflake: analyst server error (%d): %s", resp.StatusCode, string(respBody)),
		}
	default:
		return nil, fmt.Errorf("snowflake: analyst error (%d): %s", resp.StatusCode, string(respBody))
	}
}
