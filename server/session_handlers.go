package server

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultHistoryN = 20
	maxHistoryN     = MaxBreadcrumbs
)

func (s *Server) handleSession(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		Action  string         `json:"action"`
		Context map[string]any `json:"context"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}

	sess := sessionFromCtx(ctx)
	if sess == nil {
		sess = s.sessionStore.GetOrCreate("default")
	}

	switch args.Action {
	case "set":
		if len(args.Context) == 0 {
			return errorResult("\"context\" is required for \"set\" action"), nil
		}
		sess.SetContext(args.Context)
		_ = s.sessionStore.Save(sess)
	case "get":
		// no-op — we return current context below
	case "clear":
		sess.ClearContext()
		_ = s.sessionStore.Save(sess)
	default:
		return errorResult("unknown action: " + args.Action + ". Valid actions: set, get, clear"), nil
	}

	result, err := mcp.JSONResult(map[string]any{
		"session_id": sess.ID,
		"context":    sess.GetContext(),
	})
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: result.Data},
		},
	}, nil
}

func (s *Server) handleHistory(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		LastN int    `json:"last_n"`
		Tool  string `json:"tool"`
	}
	if req.Params.Arguments != nil {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error()), nil
		}
	}
	if args.LastN <= 0 {
		args.LastN = defaultHistoryN
	}
	if args.LastN > maxHistoryN {
		args.LastN = maxHistoryN
	}

	sess := sessionFromCtx(ctx)
	if sess == nil {
		sess = s.sessionStore.GetOrCreate("default")
	}

	bcs := sess.RecentBreadcrumbs(args.LastN, mcp.ToolName(args.Tool))

	result, err := mcp.JSONResult(map[string]any{
		"session_id":       sess.ID,
		"breadcrumbs":      bcs,
		"total_in_session": len(sess.Breadcrumbs),
	})
	if err != nil {
		return errorResult(err.Error()), nil
	}
	data := columnarizeResult(result.Data)
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: data},
		},
	}, nil
}
