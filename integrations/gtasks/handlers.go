package gtasks

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// ── Tasklist resource ───────────────────────────────────────────────

func listTasklists(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	maxResults := r.OptInt("max_results", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	params := url.Values{}
	if maxResults > 0 {
		params.Set("maxResults", strconv.Itoa(maxResults))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/users/@me/lists"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTasklist(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if title == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_create_tasklist: title is required"))
	}

	data, err := g.post(ctx, "/users/@me/lists", map[string]any{"title": title})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTasklist(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_delete_tasklist: tasklist_id is required"))
	}

	data, err := g.delete(ctx, "/users/@me/lists/"+url.PathEscape(tasklistID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Task resource ───────────────────────────────────────────────────

func listTasks(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	maxResults := r.OptInt("max_results", 0)
	pageToken := r.Str("page_token")
	showCompleted := r.Bool("show_completed")
	showHidden := r.Bool("show_hidden")
	showDeleted := r.Bool("show_deleted")
	dueMin := r.Str("due_min")
	dueMax := r.Str("due_max")
	completedMin := r.Str("completed_min")
	completedMax := r.Str("completed_max")
	updatedMin := r.Str("updated_min")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_list_tasks: tasklist_id is required"))
	}

	params := url.Values{}
	if maxResults > 0 {
		params.Set("maxResults", strconv.Itoa(maxResults))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if showCompleted {
		params.Set("showCompleted", "true")
	}
	if showHidden {
		params.Set("showHidden", "true")
	}
	if showDeleted {
		params.Set("showDeleted", "true")
	}
	if dueMin != "" {
		params.Set("dueMin", dueMin)
	}
	if dueMax != "" {
		params.Set("dueMax", dueMax)
	}
	if completedMin != "" {
		params.Set("completedMin", completedMin)
	}
	if completedMax != "" {
		params.Set("completedMax", completedMax)
	}
	if updatedMin != "" {
		params.Set("updatedMin", updatedMin)
	}

	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTask(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_get_task: tasklist_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_get_task: task_id is required"))
	}

	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks/" + url.PathEscape(taskID)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTask(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	title := r.Str("title")
	notes := r.Str("notes")
	due := r.Str("due")
	parentID := r.Str("parent_id")
	previousID := r.Str("previous_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_create_task: tasklist_id is required"))
	}
	if title == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_create_task: title is required"))
	}

	body := map[string]any{"title": title}
	if notes != "" {
		body["notes"] = notes
	}
	if due != "" {
		body["due"] = due
	}

	params := url.Values{}
	if parentID != "" {
		params.Set("parent", parentID)
	}
	if previousID != "" {
		params.Set("previous", previousID)
	}
	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTask(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_update_task: tasklist_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_update_task: task_id is required"))
	}

	// Build PATCH body from optional fields. Distinguish "field present but
	// empty" (caller wants to clear) from "field absent" (leave unchanged)
	// using the raw args map — r.Str collapses both cases to "".
	body := map[string]any{}
	if v, ok := args["title"]; ok {
		body["title"] = v
	}
	if v, ok := args["notes"]; ok {
		body["notes"] = v
	}
	if v, ok := args["due"]; ok {
		body["due"] = v
	}
	if v, ok := args["status"]; ok {
		body["status"] = v
	}
	if v, ok := args["completed"]; ok {
		body["completed"] = v
	}
	if v, ok := args["deleted"]; ok {
		body["deleted"] = v
	}
	if v, ok := args["hidden"]; ok {
		body["hidden"] = v
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("gtasks_update_task: no fields to update"))
	}

	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks/" + url.PathEscape(taskID)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTask(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_delete_task: tasklist_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_delete_task: task_id is required"))
	}

	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks/" + url.PathEscape(taskID)
	data, err := g.delete(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func moveTask(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	taskID := r.Str("task_id")
	parentID := r.Str("parent_id")
	previousID := r.Str("previous_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_move_task: tasklist_id is required"))
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_move_task: task_id is required"))
	}

	params := url.Values{}
	if parentID != "" {
		params.Set("parent", parentID)
	}
	if previousID != "" {
		params.Set("previous", previousID)
	}
	path := "/lists/" + url.PathEscape(tasklistID) + "/tasks/" + url.PathEscape(taskID) + "/move"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func clearCompleted(ctx context.Context, g *gtasks, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tasklistID := r.Str("tasklist_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tasklistID == "" {
		return mcp.ErrResult(fmt.Errorf("gtasks_clear_completed: tasklist_id is required"))
	}

	path := "/lists/" + url.PathEscape(tasklistID) + "/clear"
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
