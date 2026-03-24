package elasticsearch

import (
	"context"
	"fmt"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func clusterHealth(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_cluster/health", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func clusterStats(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_cluster/stats", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func nodeStats(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nodeID := r.Str("node_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_nodes/stats"
	if nodeID != "" {
		path = fmt.Sprintf("/_nodes/%s/stats", nodeID)
	}

	data, err := e.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func catNodes(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_cat/nodes?format=json&h=name,ip,heap.percent,ram.percent,cpu,load_1m,load_5m,load_15m,node.role,master,disk.used_percent,version", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func pendingTasks(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_cluster/pending_tasks", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
