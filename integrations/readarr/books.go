package readarr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Str("author_id")
	includeAll := a.Str("include_all_author_books")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if authorID != "" {
		params["authorId"] = authorID
	}
	if includeAll != "" {
		params["includeAllAuthorBooks"] = includeAll
	}
	data, err := r.get(ctx, "/api/v1/book%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBook(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.get(ctx, "/api/v1/book/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	term := a.Str("term")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if term == "" {
		return mcp.ErrResult(fmt.Errorf("term is required"))
	}
	qs := queryEncode(map[string]string{"term": term})
	data, err := r.get(ctx, "/api/v1/search%s", qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func monitorBooks(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	idsStr := a.Str("book_ids")
	monitored := a.Bool("monitored")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if idsStr == "" {
		return mcp.ErrResult(fmt.Errorf("book_ids is required"))
	}

	var bookIDs []int
	for _, s := range strings.Split(idsStr, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid book ID %q: %w", s, err))
		}
		bookIDs = append(bookIDs, id)
	}

	body := map[string]any{
		"bookIds":   bookIDs,
		"monitored": monitored,
	}
	data, err := r.put(ctx, "/api/v1/book/monitor", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addBook(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	foreignBookID := a.Str("foreign_book_id")
	foreignAuthorID := a.Str("foreign_author_id")
	rootFolderPath := a.Str("root_folder_path")
	qualityProfileID := a.OptInt("quality_profile_id", 1)
	metadataProfileID := a.OptInt("metadata_profile_id", 1)
	monitored := a.Bool("monitored")
	searchForBook := a.Bool("search_for_book")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if foreignBookID == "" {
		return mcp.ErrResult(fmt.Errorf("foreign_book_id is required"))
	}
	if foreignAuthorID == "" {
		return mcp.ErrResult(fmt.Errorf("foreign_author_id is required"))
	}
	if rootFolderPath == "" {
		return mcp.ErrResult(fmt.Errorf("root_folder_path is required"))
	}

	// Default monitored to true if not explicitly set
	if _, ok := args["monitored"]; !ok {
		monitored = true
	}

	body := map[string]any{
		"foreignBookId": foreignBookID,
		"monitored":     monitored,
		"editions": []map[string]any{{
			"foreignEditionId": "0",
			"monitored":        true,
		}},
		"author": map[string]any{
			"foreignAuthorId":   foreignAuthorID,
			"qualityProfileId":  qualityProfileID,
			"metadataProfileId": metadataProfileID,
			"rootFolderPath":    rootFolderPath,
		},
		"addOptions": map[string]any{
			"searchForNewBook": searchForBook,
		},
	}
	data, err := r.post(ctx, "/api/v1/book", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteBook(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	deleteFiles := a.Bool("delete_files")
	addExclusion := a.Bool("add_import_list_exclusion")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	params := map[string]string{}
	if deleteFiles {
		params["deleteFiles"] = "true"
	}
	if addExclusion {
		params["addImportListExclusion"] = "true"
	}
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/book/%d%s", id, queryEncode(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
