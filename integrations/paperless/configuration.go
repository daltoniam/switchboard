package paperless

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// listResource returns a paginated Paperless collection. Resource-specific
// filtering is deliberately passed through only as documented query values.
func listResource(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
		r := mcp.NewArgs(args)
		page := r.OptInt("page", 1)
		pageSize := r.OptInt("page_size", 25)
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
		data, err := p.get(ctx, path+"?"+query.Encode())
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func getResource(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
		id, err := resourceID(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		data, err := p.get(ctx, fmt.Sprintf("%s%d/", path, id))
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func createResource(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
		body, err := resourceData(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		data, err := p.doRequest(ctx, http.MethodPost, path, body)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func updateResource(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
		id, err := resourceID(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		body, err := resourceData(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		data, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("%s%d/", path, id), body)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func deleteResource(path string) handlerFunc {
	return func(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
		id, err := resourceID(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		data, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("%s%d/", path, id), nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
}

func resourceID(args map[string]any) (int, error) {
	r := mcp.NewArgs(args)
	id := r.Int("id")
	if err := r.Err(); err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("id is required")
	}
	return id, nil
}

func resourceData(args map[string]any) (map[string]any, error) {
	data, err := mcp.ArgMap(args, "data")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}
	return data, nil
}

func getTaskSummary(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	days := r.OptInt("days", 30)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/tasks/summary/?"+url.Values{"days": {strconv.Itoa(days)}}.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func runTask(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskType := r.Str("task_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if taskType == "" {
		return mcp.ErrResult(fmt.Errorf("task_type is required"))
	}
	data, err := p.doRequest(ctx, http.MethodPost, "/api/tasks/run/", map[string]any{"task_type": taskType})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func acknowledgeTasks(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	body, err := resourceData(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.doRequest(ctx, http.MethodPost, "/api/tasks/acknowledge/", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func bulkEditDocuments(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	body, err := resourceData(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.doRequest(ctx, http.MethodPost, "/api/documents/bulk_edit/", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func bulkEditObjects(ctx context.Context, p *paperless, args map[string]any) (*mcp.ToolResult, error) {
	body, err := resourceData(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.doRequest(ctx, http.MethodPost, "/api/bulk_edit_objects/", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
