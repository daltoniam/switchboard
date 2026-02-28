package gcp

import (
	"context"
	"fmt"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	resourcemanagerpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func getProject(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	project, err := g.projectsClient.GetProject(ctx, &resourcemanagerpb.GetProjectRequest{
		Name: g.projectName(),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(project)
}

func listProjects(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &resourcemanagerpb.SearchProjectsRequest{}
	if v := argStr(args, "query"); v != "" {
		req.Query = v
	}

	var projects []*resourcemanagerpb.Project
	it := g.projectsClient.SearchProjects(ctx, req)
	for i := 0; i < 100; i++ {
		p, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		projects = append(projects, p)
	}
	return jsonResult(projects)
}

func listFolders(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	req := &resourcemanagerpb.ListFoldersRequest{
		Parent: argStr(args, "parent"),
	}

	var folders []*resourcemanagerpb.Folder
	it := g.foldersClient.ListFolders(ctx, req)
	for i := 0; i < 100; i++ {
		f, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		folders = append(folders, f)
	}
	return jsonResult(folders)
}

func getFolder(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	folder, err := g.foldersClient.GetFolder(ctx, &resourcemanagerpb.GetFolderRequest{
		Name: fmt.Sprintf("folders/%s", argStr(args, "folder_id")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(folder)
}

func getIAMPolicy(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	policy, err := g.projectsClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: g.projectName(),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(policy)
}
