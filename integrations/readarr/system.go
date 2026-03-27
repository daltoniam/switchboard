package readarr

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listCommands(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/command")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func runCommand(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	name := a.Str("name")
	authorID := a.Int("author_id")
	bookID := a.Int("book_id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	body := map[string]any{"name": name}
	if authorID != 0 {
		body["authorId"] = authorID
	}
	if bookID != 0 {
		body["bookId"] = bookID
	}
	data, err := r.post(ctx, "/api/v1/command", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCommand(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.get(ctx, "/api/v1/command/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSystemStatus(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/system/status")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRootFolders(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/rootfolder")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listQualityProfiles(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/qualityprofile")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMetadataProfiles(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/metadataprofile")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTags(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/tag")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
