package readarr

import (
	"context"
	"fmt"
	"strconv"

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

func createTag(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	label := a.Str("label")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if label == "" {
		return mcp.ErrResult(fmt.Errorf("label is required"))
	}
	data, err := r.post(ctx, "/api/v1/tag", map[string]string{"label": label})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTag(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/tag/%d", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBookFiles(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Str("author_id")
	bookID := a.Str("book_id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if authorID != "" {
		params["authorId"] = authorID
	}
	if bookID != "" {
		params["bookId"] = bookID
	}
	data, err := r.get(ctx, "/api/v1/bookfile%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteBookFile(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/bookfile/%d", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBlocklist(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	page := a.OptInt("page", 1)
	pageSize := a.OptInt("page_size", 20)
	sortKey := a.Str("sort_key")
	sortDir := a.Str("sort_direction")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"page":     strconv.Itoa(page),
		"pageSize": strconv.Itoa(pageSize),
	}
	if sortKey != "" {
		params["sortKey"] = sortKey
	}
	if sortDir != "" {
		params["sortDirection"] = sortDir
	}
	data, err := r.get(ctx, "/api/v1/blocklist%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteBlocklistItem(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/blocklist/%d", id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRename(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Int("author_id")
	bookID := a.Int("book_id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if authorID == 0 {
		return mcp.ErrResult(fmt.Errorf("author_id is required"))
	}
	params := map[string]string{"authorId": strconv.Itoa(authorID)}
	if bookID != 0 {
		params["bookId"] = strconv.Itoa(bookID)
	}
	data, err := r.get(ctx, "/api/v1/rename%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRetag(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Int("author_id")
	bookID := a.Int("book_id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if authorID == 0 {
		return mcp.ErrResult(fmt.Errorf("author_id is required"))
	}
	params := map[string]string{"authorId": strconv.Itoa(authorID)}
	if bookID != 0 {
		params["bookId"] = strconv.Itoa(bookID)
	}
	data, err := r.get(ctx, "/api/v1/retag%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getManualImport(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	folder := a.Str("folder")
	filterExisting := a.Str("filter_existing_files")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if folder == "" {
		return mcp.ErrResult(fmt.Errorf("folder is required"))
	}
	params := map[string]string{"folder": folder}
	if filterExisting == "" {
		params["filterExistingFiles"] = "true"
	} else {
		params["filterExistingFiles"] = filterExisting
	}
	data, err := r.get(ctx, "/api/v1/manualimport%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func markFailed(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.post(ctx, fmt.Sprintf("/api/v1/history/failed/%d", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
