package snowflake

import (
	"context"
	"fmt"
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
	SemanticView      string           `json:"semantic_view,omitempty"`
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
	semanticView := r.Str("semantic_view")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if question == "" {
		return mcp.ErrResult(fmt.Errorf("question is required"))
	}
	// Fall back to the configured default semantic view.
	if semanticView == "" && modelFile == "" && model == "" {
		semanticView = s.semanticView
	}
	if modelFile == "" && model == "" && semanticView == "" {
		return mcp.ErrResult(fmt.Errorf("semantic_view, semantic_model_file, or semantic_model is required"))
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
		SemanticView:      semanticView,
	}

	resp, err := s.doAnalystRequest(ctx, &req)
	if err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(resp)
}

func (s *snowflake) doAnalystRequest(ctx context.Context, req *analystRequest) (*analystResponse, error) {
	return doJSON[analystResponse](ctx, s, http.MethodPost, "/api/v2/cortex/analyst/message", req, nil)
}
