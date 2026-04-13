package ollama

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func chat(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	rawMsgs, ok := args["messages"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("missing required argument: messages"))
	}
	msgsJSON, err := json.Marshal(rawMsgs)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid messages: %w", err))
	}
	var messages []chatMessage
	if err := json.Unmarshal(msgsJSON, &messages); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid messages: %w", err))
	}

	req := chatRequest{
		Model:    ModelName(model),
		Messages: messages,
		Stream:   false,
	}
	if v, ok := args["tools"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.Tools = b
		}
	}
	if v, ok := args["format"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.Format = b
		}
	}
	if v, ok := args["options"]; ok {
		if m, ok := v.(map[string]any); ok {
			req.Options = m
		}
	}
	if v, ok := args["think"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.Think = b
		}
	}
	if v, ok := args["keep_alive"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.KeepAlive = b
		}
	}

	data, err := o.post(ctx, "/api/chat", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generate(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	req := generateRequest{
		Model:  ModelName(model),
		Stream: false,
	}
	if v, _ := mcp.ArgStr(args, "prompt"); v != "" {
		req.Prompt = v
	}
	if v, _ := mcp.ArgStr(args, "suffix"); v != "" {
		req.Suffix = v
	}
	if v, _ := mcp.ArgStr(args, "system"); v != "" {
		req.System = v
	}
	if v, ok := args["images"]; ok {
		if b, err := json.Marshal(v); err == nil {
			var imgs []string
			if err := json.Unmarshal(b, &imgs); err == nil {
				req.Images = imgs
			}
		}
	}
	if v, ok := args["format"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.Format = b
		}
	}
	if v, ok := args["options"]; ok {
		if m, ok := v.(map[string]any); ok {
			req.Options = m
		}
	}
	if v, ok := args["think"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.Think = b
		}
	}
	if v, ok := args["keep_alive"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.KeepAlive = b
		}
	}
	if v, ok := args["raw"]; ok {
		if b, ok := v.(bool); ok {
			req.Raw = b
		}
	}

	data, err := o.post(ctx, "/api/generate", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func embed(ctx context.Context, o *ollama, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	model := r.Str("model")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input, ok := args["input"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("missing required argument: input"))
	}

	req := embedRequest{
		Model:  ModelName(model),
		Input:  input,
		Stream: false,
	}
	if v, ok := args["truncate"]; ok {
		if b, ok := v.(bool); ok {
			req.Truncate = &b
		}
	}
	if v, ok := args["dimensions"]; ok {
		if f, ok := v.(float64); ok {
			d := int(f)
			req.Dimensions = &d
		}
	}
	if v, ok := args["options"]; ok {
		if m, ok := v.(map[string]any); ok {
			req.Options = m
		}
	}
	if v, ok := args["keep_alive"]; ok {
		if b, err := json.Marshal(v); err == nil {
			req.KeepAlive = b
		}
	}

	data, err := o.post(ctx, "/api/embed", req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
