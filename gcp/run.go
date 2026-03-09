package gcp

import (
	"context"
	"fmt"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func runListServices(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	location := argStr(args, "location")
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
	return jsonResult(services)
}

func runGetService(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	svc, err := g.runServicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: argStr(args, "name"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(svc)
}

func runListRevisions(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &runpb.ListRevisionsRequest{
		Parent: argStr(args, "service_name"),
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
	return jsonResult(revisions)
}

func runGetRevision(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	rev, err := g.runRevisionsClient.GetRevision(ctx, &runpb.GetRevisionRequest{
		Name: argStr(args, "name"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(rev)
}
