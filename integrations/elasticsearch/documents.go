package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func getDocument(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/%s/_doc/%s", pathEscape(index), pathEscape(id)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func indexDocument(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	doc, err := mcp.ArgMap(args, "document")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if doc == nil {
		return mcp.ErrResult(fmt.Errorf("document is required"))
	}

	reader, err := jsonBody(doc)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var method, path string
	if id != "" {
		method = http.MethodPut
		path = fmt.Sprintf("/%s/_doc/%s", pathEscape(index), pathEscape(id))
	} else {
		method = http.MethodPost
		path = fmt.Sprintf("/%s/_doc", pathEscape(index))
	}

	data, err := e.doJSON(ctx, method, path, reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDocument(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}

	body := make(map[string]any)
	if doc, err := mcp.ArgMap(args, "doc"); err != nil {
		return mcp.ErrResult(err)
	} else if doc != nil {
		body["doc"] = doc
	}
	if scriptRaw, ok := args["script"]; ok && scriptRaw != nil {
		switch v := scriptRaw.(type) {
		case map[string]any:
			body["script"] = v
		case string:
			var script map[string]any
			if err := json.Unmarshal([]byte(v), &script); err != nil {
				return mcp.ErrResult(fmt.Errorf("invalid script JSON: %w", err))
			}
			body["script"] = script
		}
	}

	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("either doc or script is required"))
	}

	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_update/%s", pathEscape(index), pathEscape(id)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDocument(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}

	data, err := e.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/%s/_doc/%s", pathEscape(index), pathEscape(id)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func bulkOp(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	opsRaw, ok := args["operations"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("operations is required"))
	}
	ops, ok := opsRaw.([]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("operations must be a JSON array"))
	}

	var buf bytes.Buffer
	for _, opRaw := range ops {
		op, ok := opRaw.(map[string]any)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("each operation must be a JSON object"))
		}
		action, _ := op["action"].(string)
		idx, _ := op["index"].(string)
		id, _ := op["id"].(string)
		doc, _ := op["doc"].(map[string]any)

		meta := map[string]any{}
		if idx != "" {
			meta["_index"] = idx
		}
		if id != "" {
			meta["_id"] = id
		}

		switch action {
		case "index":
			line, _ := json.Marshal(map[string]any{"index": meta})
			buf.Write(line)
			buf.WriteByte('\n')
			docLine, _ := json.Marshal(doc)
			buf.Write(docLine)
			buf.WriteByte('\n')
		case "create":
			line, _ := json.Marshal(map[string]any{"create": meta})
			buf.Write(line)
			buf.WriteByte('\n')
			docLine, _ := json.Marshal(doc)
			buf.Write(docLine)
			buf.WriteByte('\n')
		case "update":
			line, _ := json.Marshal(map[string]any{"update": meta})
			buf.Write(line)
			buf.WriteByte('\n')
			updateBody, _ := json.Marshal(map[string]any{"doc": doc})
			buf.Write(updateBody)
			buf.WriteByte('\n')
		case "delete":
			line, _ := json.Marshal(map[string]any{"delete": meta})
			buf.Write(line)
			buf.WriteByte('\n')
		default:
			return mcp.ErrResult(fmt.Errorf("unknown action %q (valid: index, create, update, delete)", action))
		}
	}

	data, err := e.doNDJSON(ctx, "/_bulk", &buf)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func mget(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	docsRaw, ok := args["docs"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("docs is required"))
	}

	body := map[string]any{"docs": docsRaw}
	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_mget"
	if index != "" {
		path = fmt.Sprintf("/%s/_mget", pathEscape(index))
	}

	data, err := e.doJSON(ctx, http.MethodPost, path, reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func countDocs(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	body := make(map[string]any)
	if q, err := mcp.ArgMap(args, "query"); err != nil {
		return mcp.ErrResult(err)
	} else if q != nil {
		body["query"] = q
	}

	var reader io.Reader
	if len(body) > 0 {
		var err error
		reader, err = jsonBody(body)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}

	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_count", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteByQuery(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	query, err := mcp.ArgMap(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if query == nil {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}

	body := map[string]any{"query": query}
	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_delete_by_query", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateByQuery(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	query, err := mcp.ArgMap(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if query == nil {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}

	scriptRaw, ok := args["script"]
	if !ok || scriptRaw == nil {
		return mcp.ErrResult(fmt.Errorf("script is required"))
	}

	body := map[string]any{"query": query}
	switch v := scriptRaw.(type) {
	case map[string]any:
		body["script"] = v
	case string:
		var script map[string]any
		if err := json.Unmarshal([]byte(v), &script); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid script JSON: %w", err))
		}
		body["script"] = script
	}

	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_update_by_query", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
