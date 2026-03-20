package digitalocean

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func listKubernetesClusters(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	opt := listOpts(args)
	clusters, _, err := d.client.Kubernetes.List(ctx, opt)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(clusters)
}

func getKubernetesCluster(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "cluster_id")
	cluster, _, err := d.client.Kubernetes.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(cluster)
}

func listKubernetesNodePools(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "cluster_id")
	pools, _, err := d.client.Kubernetes.ListNodePools(ctx, id, listOpts(args))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(pools)
}
