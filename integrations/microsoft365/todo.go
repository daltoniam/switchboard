package microsoft365

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listTodoLists(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/me/todo/lists" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listTodoTasks(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		listID := r.Str("list_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/me/todo/lists/" + url.PathEscape(listID) + "/tasks" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func todoTaskBody(r *mcp.Args) map[string]any {
	body := map[string]any{}
	if v := r.Str("title"); v != "" {
		body["title"] = v
	}
	if v := r.Str("body"); v != "" {
		body["body"] = map[string]any{"content": v, "contentType": "text"}
	}
	if v := r.Str("due"); v != "" {
		body["dueDateTime"] = map[string]any{"dateTime": v, "timeZone": "UTC"}
	}
	if v := r.Str("importance"); v != "" {
		body["importance"] = v
	}
	if v := r.Str("status"); v != "" {
		body["status"] = v
	}
	return body
}

func createTodoTask(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	listID := r.Str("list_id")
	body := todoTaskBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.post(ctx, "/me/todo/lists/"+url.PathEscape(listID)+"/tasks", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func updateTodoTask(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	listID := r.Str("list_id")
	taskID := r.Str("task_id")
	body := todoTaskBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := "/me/todo/lists/" + url.PathEscape(listID) + "/tasks/" + url.PathEscape(taskID)
	data, err := m.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func deleteTodoTask(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	listID := r.Str("list_id")
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.del(ctx, "/me/todo/lists/"+url.PathEscape(listID)+"/tasks/"+url.PathEscape(taskID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}
