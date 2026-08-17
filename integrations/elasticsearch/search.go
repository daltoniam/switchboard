package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func search(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	size := r.Int("size")
	from := r.Int("from")
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
	if size > 0 {
		body["size"] = size
	}
	if from > 0 {
		body["from"] = from
	}
	if sortRaw, ok := args["sort"]; ok && sortRaw != nil {
		body["sort"] = sortRaw
	}
	if aggsRaw, ok := args["aggs"]; ok && aggsRaw != nil {
		body["aggs"] = aggsRaw
	}
	if srcRaw, ok := args["_source"]; ok && srcRaw != nil {
		body["_source"] = srcRaw
	}
	if hlRaw, ok := args["highlight"]; ok && hlRaw != nil {
		body["highlight"] = hlRaw
	}

	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_search", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func msearch(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	searchesRaw, ok := args["searches"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("searches is required"))
	}

	searches, ok := searchesRaw.([]any)
	if !ok {
		return mcp.ErrResult(fmt.Errorf("searches must be a JSON array"))
	}

	var ndjson strings.Builder
	for _, s := range searches {
		item, ok := s.(map[string]any)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("each search must be a JSON object"))
		}

		header := make(map[string]any)
		if idx, ok := item["index"]; ok {
			header["index"] = idx
		}
		headerBytes, _ := json.Marshal(header)
		ndjson.Write(headerBytes)
		ndjson.WriteByte('\n')

		bodyMap, _ := item["body"].(map[string]any)
		if bodyMap == nil {
			bodyMap = make(map[string]any)
		}
		bodyBytes, _ := json.Marshal(bodyMap)
		ndjson.Write(bodyBytes)
		ndjson.WriteByte('\n')
	}

	data, err := e.doNDJSON(ctx, "/_msearch", strings.NewReader(ndjson.String()))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

var validSQLFormats = map[string]bool{
	"json": true, "csv": true, "tsv": true, "txt": true, "yaml": true,
}

func sqlQuery(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	format := r.Str("format")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	if format == "" {
		format = "json"
	}
	if !validSQLFormats[format] {
		return mcp.ErrResult(fmt.Errorf("invalid format %q: must be one of json, csv, tsv, txt, yaml", format))
	}

	body := map[string]any{"query": query}
	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/_sql?format=%s", format), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
