package kubernetes

import mcp "github.com/daltoniam/switchboard"

const namespaceParamDesc = "Namespace (omit to use configured namespace; use all_namespaces=true for all namespaces where supported)."
const contextParamDesc = "Kubernetes context or configured cluster name. Omit to use the configured default context."

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("kubernetes_list_contexts"),
		Description: "List Kubernetes kubeconfig contexts, clusters, users, and namespaces. Start here for local Kubernetes context discovery before choosing a cluster.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_clusters"),
		Description: "List configured Kubernetes clusters and contexts available to this integration. Start here when choosing which cluster to inspect.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_namespaces"),
		Description: "List Kubernetes namespaces in the cluster. Start here for Kubernetes cluster discovery and finding available environments.",
		Parameters: map[string]string{
			"context": contextParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_pods"),
		Description: "List Kubernetes pods with phase, node, labels, restarts, and readiness. Start here for workload health, container debugging, and finding failing pods.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"namespace":      namespaceParamDesc,
			"all_namespaces": "Set true to list pods across all namespaces.",
			"label_selector": "Kubernetes label selector, for example app=api,tier=backend.",
			"field_selector": "Kubernetes field selector, for example status.phase=Running.",
			"limit":          "Maximum number of pods to return (default: 100, max: 500).",
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_get_pod"),
		Description: "Get details for a specific Kubernetes pod, including containers, conditions, owner references, and recent status. Use after list_pods.",
		Parameters: map[string]string{
			"context":   contextParamDesc,
			"namespace": namespaceParamDesc,
			"pod":       "Pod name.",
		},
		Required: []string{"pod"},
	},
	{
		Name:        mcp.ToolName("kubernetes_read_pod_logs"),
		Description: "Read logs from a Kubernetes pod container. Use after list_pods or get_pod for debugging crashes, errors, and workload failures.",
		Parameters: map[string]string{
			"context":    contextParamDesc,
			"namespace":  namespaceParamDesc,
			"pod":        "Pod name.",
			"container":  "Container name (optional for single-container pods).",
			"tail":       "Number of log lines from the end (default: 200, max: 2000).",
			"previous":   "Set true to read logs from the previous terminated container instance.",
			"timestamps": "Set true to include log timestamps.",
		},
		Required: []string{"pod"},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_events"),
		Description: "List Kubernetes events sorted by time. Use for debugging scheduling, image pulls, probes, restarts, and rollout failures.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"namespace":      namespaceParamDesc,
			"all_namespaces": "Set true to list events across all namespaces.",
			"field_selector": "Kubernetes field selector, for example involvedObject.name=api.",
			"limit":          "Maximum number of events to return (default: 100, max: 500).",
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_deployments"),
		Description: "List Kubernetes deployments with replicas, availability, labels, and images. Use for workload discovery and rollout health checks.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"namespace":      namespaceParamDesc,
			"all_namespaces": "Set true to list deployments across all namespaces.",
			"label_selector": "Kubernetes label selector.",
			"limit":          "Maximum number of deployments to return (default: 100, max: 500).",
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_get_deployment"),
		Description: "Get details for a specific Kubernetes deployment, including rollout conditions, replica counts, strategy, selectors, and images. Use after list_deployments.",
		Parameters: map[string]string{
			"context":    contextParamDesc,
			"namespace":  namespaceParamDesc,
			"deployment": "Deployment name.",
		},
		Required: []string{"deployment"},
	},
	{
		Name:        mcp.ToolName("kubernetes_rollout_status"),
		Description: "Check Kubernetes deployment rollout status and explain whether a rollout is complete, progressing, or stuck. Use after list_deployments or get_deployment.",
		Parameters: map[string]string{
			"context":    contextParamDesc,
			"namespace":  namespaceParamDesc,
			"deployment": "Deployment name.",
		},
		Required: []string{"deployment"},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_services"),
		Description: "List Kubernetes services with type, cluster IPs, external IPs, ports, and selectors. Use for service discovery and network debugging.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"namespace":      namespaceParamDesc,
			"all_namespaces": "Set true to list services across all namespaces.",
			"label_selector": "Kubernetes label selector.",
			"limit":          "Maximum number of services to return (default: 100, max: 500).",
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_ingresses"),
		Description: "List Kubernetes ingresses with hosts, paths, classes, TLS, and load balancer addresses. Use for HTTP routing and exposure debugging.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"namespace":      namespaceParamDesc,
			"all_namespaces": "Set true to list ingresses across all namespaces.",
			"label_selector": "Kubernetes label selector.",
			"limit":          "Maximum number of ingresses to return (default: 100, max: 500).",
		},
	},
	{
		Name:        mcp.ToolName("kubernetes_list_nodes"),
		Description: "List Kubernetes cluster nodes with readiness, versions, roles, taints, capacity, and allocatable resources. Use for cluster capacity and node health debugging.",
		Parameters: map[string]string{
			"context":        contextParamDesc,
			"label_selector": "Kubernetes label selector.",
			"limit":          "Maximum number of nodes to return (default: 100, max: 500).",
		},
	},
}
