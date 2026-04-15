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
		sess = s.sessionStore.GetOrCreate(sessionIDFromReq(req.Session))
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
		sess = s.sessionStore.GetOrCreate(sessionIDFromReq(req.Session))
	}

	bcs := sess.RecentBreadcrumbs(args.LastN, mcp.ToolName(args.Tool))

	result, err := mcp.JSONResult(map[string]any{
		"session_id":       sess.ID,
		"breadcrumbs":      bcs,
		"total_in_session": sess.TotalBreadcrumbs(),
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

func (s *Server) handlePin(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	var args struct {
		Action string `json:"action"`
		Handle string `json:"handle"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}

	sess := sessionFromCtx(ctx)
	if sess == nil {
		sess = s.sessionStore.GetOrCreate(sessionIDFromReq(req.Session))
	}

	switch args.Action {
	case "list":
		pinned := sess.ListPinned()
		type pinSummary struct {
			Handle string       `json:"handle"`
			Tool   mcp.ToolName `json:"tool"`
			Size   int          `json:"size_bytes"`
		}
		summaries := make([]pinSummary, 0, len(pinned))
		for _, pr := range pinned {
			summaries = append(summaries, pinSummary{
				Handle: pr.Handle,
				Tool:   pr.Tool,
				Size:   pr.SizeBytes,
			})
		}
		result, err := mcp.JSONResult(map[string]any{
			"pinned":       summaries,
			"total_bytes":  sess.PinnedBytes(),
			"max_bytes":    MaxPinnedBytes,
			"pinned_count": len(summaries),
		})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
		}, nil

	case "get":
		if args.Handle == "" {
			return errorResult("\"handle\" is required for \"get\" action"), nil
		}
		ref := args.Handle
		if args.Path != "" {
			ref = args.Handle + "." + args.Path
		}
		val, err := sess.ResolveRef(ref)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		result, err := mcp.JSONResult(val)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
		}, nil

	case "unpin":
		if args.Handle == "" {
			return errorResult("\"handle\" is required for \"unpin\" action"), nil
		}
		ok := sess.Unpin(args.Handle)
		_ = s.sessionStore.Save(sess)
		result, err := mcp.JSONResult(map[string]any{
			"unpinned": ok,
			"handle":   args.Handle,
		})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result.Data}},
		}, nil

	default:
		return errorResult("unknown action: " + args.Action + ". Valid actions: list, get, unpin"), nil
	}
}
