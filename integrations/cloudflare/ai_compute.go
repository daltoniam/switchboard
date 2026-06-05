package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Workers AI ---

func listAIModels(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	search, _ := mcp.ArgStr(args, "search")
	task, _ := mcp.ArgStr(args, "task")
	source, _ := mcp.ArgStr(args, "source")
	q := queryEncode(map[string]string{
		"search":   search,
		"task":     task,
		"source":   source,
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/accounts/%s/ai/models/search%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func runAIModel(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	modelName := r.Str("model_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Body is free-form so callers can pass `prompt`, `messages`, `input`, etc.
	body, _ := mcp.ArgMap(args, "body")
	if body == nil {
		// Convenience: if caller passed a top-level `prompt`, wrap it.
		if prompt, _ := mcp.ArgStr(args, "prompt"); prompt != "" {
			body = map[string]any{"prompt": prompt}
		}
	}
	if body == nil {
		return mcp.ErrResult(fmt.Errorf("either `body` (map) or `prompt` (string) is required"))
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/ai/run/%s", acct, modelName), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Vectorize v2 ---

func listVectorizeIndexes(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/vectorize/v2/indexes", acct)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVectorizeIndex(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	indexName := r.Str("index_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/vectorize/v2/indexes/%s", acct, indexName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createVectorizeIndex(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	metric := r.Str("metric")
	dimensions := r.Int("dimensions")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	description, _ := mcp.ArgStr(args, "description")
	body := map[string]any{
		"name": name,
		"config": map[string]any{
			"metric":     metric,
			"dimensions": dimensions,
		},
	}
	if description != "" {
		body["description"] = description
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/vectorize/v2/indexes", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteVectorizeIndex(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	indexName := r.Str("index_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/vectorize/v2/indexes/%s", acct, indexName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryVectorizeIndex(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	indexName := r.Str("index_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	vec, ok := args["vector"]
	if !ok || vec == nil {
		return mcp.ErrResult(fmt.Errorf("vector (array of floats) is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"vector": vec}
	if topK := mcp.OptInt(args, "topK", 0); topK > 0 {
		body["topK"] = topK
	}
	if returnValues, err := mcp.ArgBool(args, "returnValues"); err == nil && hasKey(args, "returnValues") {
		body["returnValues"] = returnValues
	}
	if returnMetadata, _ := mcp.ArgStr(args, "returnMetadata"); returnMetadata != "" {
		body["returnMetadata"] = returnMetadata
	}
	if filter, _ := mcp.ArgMap(args, "filter"); filter != nil {
		body["filter"] = filter
	}
	if ns, _ := mcp.ArgStr(args, "namespace"); ns != "" {
		body["namespace"] = ns
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/vectorize/v2/indexes/%s/query", acct, indexName), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// hasKey reports whether a key is present (and non-nil) in args.
func hasKey(args map[string]any, key string) bool {
	v, ok := args[key]
	return ok && v != nil
}
