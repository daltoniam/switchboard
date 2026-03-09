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
	location := argStr(args, "location")
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
	return jsonResult(fns)
}

func functionsGet(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	fn, err := g.functionsClient.GetFunction(ctx, &functionspb.GetFunctionRequest{
		Name: argStr(args, "name"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(fn)
}

func functionsGetIAMPolicy(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	policy, err := g.functionsClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: argStr(args, "name"),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(policy)
}
