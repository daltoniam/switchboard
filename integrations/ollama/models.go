package ollama

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
)

func listModels(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/tags")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func showModel(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.post(ctx, "/api/show", map[string]any{"model": model})
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Inject model name — the API response doesn't include it as a top-level field,
	// but RenderMarkdown needs it for the heading.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		if nameJSON, err := json.Marshal(model); err == nil {
			obj["model"] = nameJSON
			if patched, err := json.Marshal(obj); err == nil {
				data = patched
			}
		}
	}
	return mcp.RawResult(data)
}

func pullModel(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.post(ctx, "/api/pull", map[string]any{"model": model, "stream": false})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteModel(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.del(ctx, "/api/delete", map[string]any{"model": model})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func copyModel(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	source := r.Str("source")
	destination := r.Str("destination")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.post(ctx, "/api/copy", copyRequest{
		Source:      ModelName(source),
		Destination: ModelName(destination),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createModel(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	req := map[string]any{"model": model, "stream": false}
	if v, _ := mcp.ArgStr(args, "from"); v != "" {
		req["from"] = v
	}
	if v, _ := mcp.ArgStr(args, "system"); v != "" {
		req["system"] = v
	}
	if v, _ := mcp.ArgStr(args, "template"); v != "" {
		req["template"] = v
	}
	if v, _ := mcp.ArgStr(args, "quantize"); v != "" {
		req["quantize"] = v
	}

	data, err := o.post(ctx, "/api/create", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRunning(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/ps")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVersion(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/api/version")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
