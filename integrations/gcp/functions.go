package gcp

import (
	"context"
	"fmt"

	functionspb "cloud.google.com/go/functions/apiv2/functionspb"
	iampb "cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func functionsList(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	location := r.Str("location")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if location == "" {
		location = "-"
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", g.projectID, location)

	req := &functionspb.ListFunctionsRequest{
		Parent: parent,
	}

	var fns []*functionspb.Function
	it := g.functionsClient.ListFunctions(ctx, req)
	for i := 0; i < 500; i++ {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		fns = append(fns, f)
	}
	return mcp.JSONResult(fns)
}

func functionsGet(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	fn, err := g.functionsClient.GetFunction(ctx, &functionspb.GetFunctionRequest{
		Name: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(fn)
}

func functionsGetIAMPolicy(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	policy, err := g.functionsClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: name,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(policy)
}
