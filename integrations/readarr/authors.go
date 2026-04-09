package readarr

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listAuthors(ctx context.Context, r *readarr, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/api/v1/author")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.get(ctx, "/api/v1/author/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	foreignAuthorID := a.Str("foreign_author_id")
	rootFolderPath := a.Str("root_folder_path")
	qualityProfileID := a.OptInt("quality_profile_id", 1)
	metadataProfileID := a.OptInt("metadata_profile_id", 1)
	monitored := a.Bool("monitored")
	monitor := a.Str("monitor")
	searchForMissing := a.Bool("search_for_missing_books")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if foreignAuthorID == "" {
		return mcp.ErrResult(fmt.Errorf("foreign_author_id is required"))
	}
	if rootFolderPath == "" {
		return mcp.ErrResult(fmt.Errorf("root_folder_path is required"))
	}

	if _, ok := args["monitored"]; !ok {
		monitored = true
	}
	if monitor == "" {
		monitor = "all"
	}

	body := map[string]any{
		"foreignAuthorId":   foreignAuthorID,
		"qualityProfileId":  qualityProfileID,
		"metadataProfileId": metadataProfileID,
		"rootFolderPath":    rootFolderPath,
		"monitored":         monitored,
		"monitorNewItems":   "all",
		"addOptions": map[string]any{
			"monitor":               monitor,
			"searchForMissingBooks": searchForMissing,
		},
	}
	data, err := r.post(ctx, "/api/v1/author", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}

	existing, err := r.get(ctx, "/api/v1/author/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var author map[string]any
	if err := json.Unmarshal(existing, &author); err != nil {
		return mcp.ErrResult(fmt.Errorf("failed to parse author: %w", err))
	}

	if v, ok := args["monitored"]; ok {
		b, err := mcp.ArgBool(args, "monitored")
		if err != nil {
			return mcp.ErrResult(err)
		}
		_ = v
		author["monitored"] = b
	}
	if v, ok := args["quality_profile_id"]; ok {
		_ = v
		qp, err := mcp.ArgInt(args, "quality_profile_id")
		if err != nil {
			return mcp.ErrResult(err)
		}
		author["qualityProfileId"] = qp
	}
	if v, ok := args["metadata_profile_id"]; ok {
		_ = v
		mp, err := mcp.ArgInt(args, "metadata_profile_id")
		if err != nil {
			return mcp.ErrResult(err)
		}
		author["metadataProfileId"] = mp
	}
	if v, ok := args["path"]; ok {
		_ = v
		p, err := mcp.ArgStr(args, "path")
		if err != nil {
			return mcp.ErrResult(err)
		}
		author["path"] = p
	}

	moveFiles := a.Bool("move_files")
	params := map[string]string{}
	if moveFiles {
		params["moveFiles"] = "true"
	}

	data, err := r.put(ctx, fmt.Sprintf("/api/v1/author/%d%s", id, queryEncode(params)), author)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/author/%d%s", id, queryEncode(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
