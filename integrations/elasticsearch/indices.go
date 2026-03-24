package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listIndices(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pattern := r.Str("pattern")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_cat/indices?format=json&h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size&s=index"
	if pattern != "" {
		path = fmt.Sprintf("/_cat/indices/%s?format=json&h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size&s=index", pathEscape(pattern))
	}

	data, err := e.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/%s", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	body := make(map[string]any)
	if v, err := mcp.ArgMap(args, "settings"); err != nil {
		return mcp.ErrResult(err)
	} else if v != nil {
		body["settings"] = v
	}
	if v, err := mcp.ArgMap(args, "mappings"); err != nil {
		return mcp.ErrResult(err)
	} else if v != nil {
		body["mappings"] = v
	}

	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPut, fmt.Sprintf("/%s", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/%s", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMapping(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/%s/_mapping", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func putMapping(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	props, err := mcp.ArgMap(args, "properties")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if props == nil {
		return mcp.ErrResult(fmt.Errorf("properties is required"))
	}

	body := map[string]any{"properties": props}
	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPut, fmt.Sprintf("/%s/_mapping", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSettings(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/%s/_settings", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func putSettings(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	settings, err := mcp.ArgMap(args, "settings")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if settings == nil {
		return mcp.ErrResult(fmt.Errorf("settings is required"))
	}

	reader, err := jsonBody(settings)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPut, fmt.Sprintf("/%s/_settings", pathEscape(index)), reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func indexStats(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/%s/_stats", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func openIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_open", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func closeIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_close", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func refreshIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/%s/_refresh", pathEscape(index)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func forcemergeIndex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	maxSegs := r.Int("max_segments")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if index == "" {
		return mcp.ErrResult(fmt.Errorf("index is required"))
	}

	path := fmt.Sprintf("/%s/_forcemerge", pathEscape(index))
	if maxSegs > 0 {
		path += fmt.Sprintf("?max_num_segments=%d", maxSegs)
	}

	data, err := e.doJSON(ctx, http.MethodPost, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAliases(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_cat/aliases?format=json&h=alias,index,filter,routing.index,routing.search,is_write_index&s=alias"
	if index != "" {
		path = fmt.Sprintf("/_cat/aliases/%s?format=json&h=alias,index,filter,routing.index,routing.search,is_write_index&s=alias", pathEscape(index))
	}

	data, err := e.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAliases(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	actionsRaw, ok := args["actions"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("actions is required"))
	}

	body := map[string]any{"actions": actionsRaw}
	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, "/_aliases", reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIndexTemplates(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_index_template", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIndexTemplate(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/_index_template/%s", pathEscape(name)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSnapshotRepos(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_snapshot", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSnapshots(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	repo := r.Str("repository")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if repo == "" {
		return mcp.ErrResult(fmt.Errorf("repository is required"))
	}

	data, err := e.doJSON(ctx, http.MethodGet, fmt.Sprintf("/_snapshot/%s/_all", pathEscape(repo)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTasks(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	actions := r.Str("actions")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_tasks?detailed=true&group_by=none"
	if actions != "" {
		path += "&actions=" + url.QueryEscape(actions)
	}

	data, err := e.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelTask(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if taskID == "" {
		return mcp.ErrResult(fmt.Errorf("task_id is required"))
	}

	data, err := e.doJSON(ctx, http.MethodPost, fmt.Sprintf("/_tasks/%s/_cancel", pathEscape(taskID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func catShards(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	index := r.Str("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := "/_cat/shards?format=json&h=index,shard,prirep,state,docs,store,ip,node&s=index"
	if index != "" {
		path = fmt.Sprintf("/_cat/shards/%s?format=json&h=index,shard,prirep,state,docs,store,ip,node&s=index", pathEscape(index))
	}

	data, err := e.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func catAllocation(ctx context.Context, e *esInt, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := e.doJSON(ctx, http.MethodGet, "/_cat/allocation?format=json&h=shards,disk.indices,disk.used,disk.avail,disk.total,disk.percent,host,ip,node&s=node", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func reindex(ctx context.Context, e *esInt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	srcIndex := r.Str("source_index")
	destIndex := r.Str("dest_index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if srcIndex == "" {
		return mcp.ErrResult(fmt.Errorf("source_index is required"))
	}
	if destIndex == "" {
		return mcp.ErrResult(fmt.Errorf("dest_index is required"))
	}

	source := map[string]any{"index": srcIndex}
	if q, err := mcp.ArgMap(args, "query"); err != nil {
		return mcp.ErrResult(err)
	} else if q != nil {
		source["query"] = q
	}

	body := map[string]any{
		"source": source,
		"dest":   map[string]any{"index": destIndex},
	}

	if scriptRaw, ok := args["script"]; ok && scriptRaw != nil {
		switch v := scriptRaw.(type) {
		case map[string]any:
			body["script"] = v
		case string:
			var script map[string]any
			if err := json.Unmarshal([]byte(v), &script); err != nil {
				return mcp.ErrResult(fmt.Errorf("invalid script JSON: %w", err))
			}
			body["script"] = script
		}
	}

	reader, err := jsonBody(body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := e.doJSON(ctx, http.MethodPost, "/_reindex?wait_for_completion=false", reader)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
