package gcp

import (
	"context"
	"fmt"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func runListServices(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	location := r.Str("location")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", g.projectID, location)

	req := &runpb.ListServicesRequest{
		Parent: parent,
	}

	var services []*runpb.Service
	it := g.runServicesClient.ListServices(ctx, req)
	for i := 0; i < 500; i++ {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		services = append(services, s)
	}
	return mcp.JSONResult(services)
}

func runGetService(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	svc, err := g.runServicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(svc)
}

func runListRevisions(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	serviceName := r.Str("service_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &runpb.ListRevisionsRequest{
		Parent: serviceName,
	}

	var revisions []*runpb.Revision
	it := g.runRevisionsClient.ListRevisions(ctx, req)
	for i := 0; i < 500; i++ {
		r, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		revisions = append(revisions, r)
	}
	return mcp.JSONResult(revisions)
}

func runGetRevision(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	rev, err := g.runRevisionsClient.GetRevision(ctx, &runpb.GetRevisionRequest{
		Name: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(rev)
}
