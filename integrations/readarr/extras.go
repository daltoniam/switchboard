package readarr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getCalendar(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	start := a.Str("start")
	end := a.Str("end")
	unmonitored := a.Str("unmonitored")
	includeAuthor := a.Str("include_author")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if start != "" {
		params["start"] = start
	}
	if end != "" {
		params["end"] = end
	}
	if unmonitored != "" {
		params["unmonitored"] = unmonitored
	}
	if includeAuthor != "" {
		params["includeAuthor"] = includeAuthor
	}
	data, err := r.get(ctx, "/api/v1/calendar%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMissing(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := r.get(ctx, "/api/v1/wanted/missing%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCutoff(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := r.get(ctx, "/api/v1/wanted/cutoff%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getQueue(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	page := a.OptInt("page", 1)
	pageSize := a.OptInt("page_size", 20)
	sortKey := a.Str("sort_key")
	sortDir := a.Str("sort_direction")
	includeAuthor := a.Str("include_author")
	includeBook := a.Str("include_book")
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
	if includeAuthor != "" {
		params["includeAuthor"] = includeAuthor
	}
	if includeBook != "" {
		params["includeBook"] = includeBook
	}
	data, err := r.get(ctx, "/api/v1/queue%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteQueueItem(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	removeFromClient := a.Str("remove_from_client")
	blocklist := a.Str("blocklist")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	params := map[string]string{
		"removeFromClient": "true",
	}
	if removeFromClient != "" {
		params["removeFromClient"] = removeFromClient
	}
	if blocklist != "" {
		params["blocklist"] = blocklist
	}
	data, err := r.delWithQuery(ctx, fmt.Sprintf("/api/v1/queue/%d%s", id, queryEncode(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteQueueBulk(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	idsStr := a.Str("ids")
	removeFromClient := a.Str("remove_from_client")
	blocklist := a.Str("blocklist")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if idsStr == "" {
		return mcp.ErrResult(fmt.Errorf("ids is required"))
	}

	var ids []int
	for _, s := range strings.Split(idsStr, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid queue ID %q: %w", s, err))
		}
		ids = append(ids, id)
	}

	params := map[string]string{
		"removeFromClient": "true",
	}
	if removeFromClient != "" {
		params["removeFromClient"] = removeFromClient
	}
	if blocklist != "" {
		params["blocklist"] = blocklist
	}

	body := map[string]any{"ids": ids}
	data, err := r.doRequest(ctx, "DELETE", "/api/v1/queue/bulk"+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func grabQueueItem(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	id := a.Int("id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("id is required"))
	}
	data, err := r.post(ctx, fmt.Sprintf("/api/v1/queue/grab/%d", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getHistory(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	page := a.OptInt("page", 1)
	pageSize := a.OptInt("page_size", 20)
	sortKey := a.Str("sort_key")
	sortDir := a.Str("sort_direction")
	eventType := a.Str("event_type")
	bookID := a.Str("book_id")
	includeAuthor := a.Str("include_author")
	includeBook := a.Str("include_book")
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
	if eventType != "" {
		params["eventType"] = eventType
	}
	if bookID != "" {
		params["bookId"] = bookID
	}
	if includeAuthor != "" {
		params["includeAuthor"] = includeAuthor
	}
	if includeBook != "" {
		params["includeBook"] = includeBook
	}
	data, err := r.get(ctx, "/api/v1/history%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getHistoryAuthor(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	authorID := a.Int("author_id")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if authorID == 0 {
		return mcp.ErrResult(fmt.Errorf("author_id is required"))
	}
	qs := queryEncode(map[string]string{"authorId": strconv.Itoa(authorID)})
	data, err := r.get(ctx, "/api/v1/history/author%s", qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getHistorySince(ctx context.Context, r *readarr, args map[string]any) (*mcp.ToolResult, error) {
	a := mcp.NewArgs(args)
	date := a.Str("date")
	eventType := a.Str("event_type")
	if err := a.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if date == "" {
		return mcp.ErrResult(fmt.Errorf("date is required"))
	}
	params := map[string]string{"date": date}
	if eventType != "" {
		params["eventType"] = eventType
	}
	data, err := r.get(ctx, "/api/v1/history/since%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
