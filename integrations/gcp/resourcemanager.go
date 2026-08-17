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
	return mcp.JSONResult(project)
}

func listProjects(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	req := &resourcemanagerpb.SearchProjectsRequest{}
	if v := r.Str("query"); v != "" {
		req.Query = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
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
	return mcp.JSONResult(projects)
}

func listFolders(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Str("parent")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	req := &resourcemanagerpb.ListFoldersRequest{
		Parent: parent,
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
	return mcp.JSONResult(folders)
}

func getFolder(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	folderID := r.Str("folder_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	folder, err := g.foldersClient.GetFolder(ctx, &resourcemanagerpb.GetFolderRequest{
		Name: fmt.Sprintf("folders/%s", folderID),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(folder)
}

func getIAMPolicy(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	policy, err := g.projectsClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: g.projectName(),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(policy)
}
