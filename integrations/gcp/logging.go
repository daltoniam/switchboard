package gcp

import (
	"context"
	"fmt"

	loggingpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func loggingListEntries(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageSize := r.Int32("page_size")
	if pageSize <= 0 {
		pageSize = 50
	}

	req := &loggingpb.ListLogEntriesRequest{
		ResourceNames: []string{g.projectName()},
		PageSize:      pageSize,
	}
	if v := r.Str("filter"); v != "" {
		req.Filter = v
	}
	if v := r.Str("order_by"); v != "" {
		req.OrderBy = v
	} else {
		req.OrderBy = "timestamp desc"
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var entries []any
	it := g.loggingClient.ListLogEntries(ctx, req)
	for i := 0; i < int(pageSize); i++ {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		entries = append(entries, entry)
	}
	return mcp.JSONResult(entries)
}

func loggingListLogNames(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	req := &loggingpb.ListLogsRequest{
		Parent: g.projectName(),
	}

	var names []string
	it := g.loggingClient.ListLogs(ctx, req)
	for i := 0; i < 500; i++ {
		name, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		names = append(names, name)
	}
	return mcp.JSONResult(names)
}

func loggingListSinks(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	req := &loggingpb.ListSinksRequest{
		Parent: g.projectName(),
	}

	var sinks []any
	it := g.loggingConfigClient.ListSinks(ctx, req)
	for i := 0; i < 500; i++ {
		sink, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		sinks = append(sinks, sink)
	}
	return mcp.JSONResult(sinks)
}

func loggingGetSink(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sinkName := r.Str("sink_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	name := fmt.Sprintf("%s/sinks/%s", g.projectName(), sinkName)
	sink, err := g.loggingConfigClient.GetSink(ctx, &loggingpb.GetSinkRequest{
		SinkName: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(sink)
}
