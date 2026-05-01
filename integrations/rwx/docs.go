package rwx

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func docsSearch(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	query := ra.Str("query")
	limit := ra.Int("limit")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = 5
	}

	cmdArgs := []string{"docs", "search", query, "--output", "json", "--limit", fmt.Sprintf("%d", limit)}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var parsed struct {
		Query     string `json:"Query"`
		TotalHits int    `json:"TotalHits"`
		Results   []struct {
			URL   string `json:"url"`
			Path  string `json:"path"`
			Title string `json:"title"`
			Body  string `json:"body"`
		} `json:"Results"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return mcp.ErrResult(fmt.Errorf("parse docs search: %w", err))
	}

	var results []map[string]any
	for _, doc := range parsed.Results {
		entry := map[string]any{
			"title": doc.Title,
			"url":   doc.URL,
			"path":  doc.Path,
		}
		if len(doc.Body) > 500 {
			entry["snippet"] = doc.Body[:500] + "..."
		} else {
			entry["snippet"] = doc.Body
		}
		results = append(results, entry)
	}

	return mcp.JSONResult(map[string]any{
		"query":      parsed.Query,
		"total_hits": parsed.TotalHits,
		"count":      len(results),
		"results":    results,
	})
}

func docsPull(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	urlOrPath, err := mcp.ArgStr(args, "url_or_path")
	if err != nil {
		return mcp.ErrResult(err)
	}

	cmdArgs := []string{"docs", "pull", urlOrPath, "--output", "json"}
	output, err := r.runRWXCommand(cmdArgs, 30000)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var parsed struct {
		URL  string `json:"URL"`
		Body string `json:"Body"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return mcp.ErrResult(fmt.Errorf("parse docs pull: %w", err))
	}

	return mcp.JSONResult(map[string]any{
		"url":     parsed.URL,
		"content": parsed.Body,
	})
}
