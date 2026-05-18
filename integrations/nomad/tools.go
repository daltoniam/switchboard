package nomad

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Jobs ─────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_jobs"), Description: "List all jobs in the Nomad cluster. Start here for workload orchestration, container scheduling, and discovering running services.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}, {Name: mcp.ParamName("prefix"), Description: "Filter by job ID prefix"}, {Name: mcp.ParamName("filter"), Description: `Filter expression (e.g., Status == "running")`}},
	},
	{
		Name: mcp.ToolName("nomad_get_job"), Description: "Get full specification and status of a specific Nomad job, including task groups, constraints, and resource requirements. Use after list_jobs.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}, {Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},
	{
		Name: mcp.ToolName("nomad_get_job_versions"), Description: "Get version history for a Nomad job. Shows previous configurations and when changes were made.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}, {Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},
	{
		Name: mcp.ToolName("nomad_register_job"), Description: "Register (create or update) a Nomad job. Accepts a full job specification as JSON.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job"), Description: "Full job specification as JSON object", Required: true}, {Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},
	{
		Name: mcp.ToolName("nomad_stop_job"), Description: "Stop (deregister) a running Nomad job. All allocations will be stopped.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}, {Name: mcp.ParamName("purge"), Description: "Completely purge the job from the system (true/false, default: false)"}, {Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},
	{
		Name: mcp.ToolName("nomad_force_evaluate"), Description: "Force a new evaluation for a Nomad job, triggering rescheduling. Useful when allocations are unhealthy or stuck.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}, {Name: mcp.ParamName("namespace"), Description:

		// ── Allocations ──────────────────────────────────────────────────
		"Namespace (default: default)"}},
	},

	{
		Name: mcp.ToolName("nomad_list_allocations"), Description: "List all allocations across the Nomad cluster. Shows where tasks are placed and their health status.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}, {Name: mcp.ParamName("prefix"), Description: "Filter by allocation ID prefix"}, {Name: mcp.ParamName("filter"), Description: "Filter expression"}},
	},
	{
		Name: mcp.ToolName("nomad_get_allocation"), Description: "Get details of a specific Nomad allocation, including task states, events, restart history, and resource usage.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alloc_id"), Description: "Allocation ID", Required: true}},
	},
	{
		Name: mcp.ToolName("nomad_get_job_allocations"), Description: "List all allocations for a specific Nomad job. Shows placement, health, and status of each task instance.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}, {Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},
	{
		Name: mcp.ToolName("nomad_stop_allocation"), Description: "Stop a specific Nomad allocation. The scheduler may place a replacement depending on job configuration.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alloc_id"), Description: "Allocation ID", Required: true}},
	},
	{
		Name: mcp.ToolName("nomad_restart_allocation"), Description: "Restart a task within a Nomad allocation. Optionally specify which task to restart.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alloc_id"), Description: "Allocation ID", Required: true}, {Name: mcp.ParamName("task"), Description: "Task name (optional, restarts all tasks if omitted)"}},
	},
	{
		Name: mcp.ToolName("nomad_read_allocation_logs"), Description: "Read stdout or stderr logs from a task in a Nomad allocation. Use for debugging container and workload issues.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alloc_id"), Description: "Allocation ID", Required: true}, {Name: mcp.ParamName("task"), Description: "Task name", Required: true}, {Name: mcp.ParamName("log_type"), Description: "Log type: stdout or stderr (default: stdout)"}, {Name: mcp.ParamName("plain"), Description: "Return plain text instead of JSON (true/false, default: true)"},

		// ── Nodes ────────────────────────────────────────────────────────
		{Name: mcp.ParamName("origin"), Description: "Log origin: start or end (default: end)"}, {Name: mcp.ParamName("offset"), Description: "Byte offset to start reading from"}},
	},

	{
		Name: mcp.ToolName("nomad_list_nodes"), Description: "List all client nodes in the Nomad cluster. Shows node status, datacenter, drivers, and scheduling eligibility.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("prefix"), Description: "Filter by node ID prefix"}, {Name: mcp.ParamName("filter"), Description: `Filter expression (e.g., Status == "ready")`}},
	},
	{
		Name: mcp.ToolName("nomad_get_node"), Description: "Get full details of a specific Nomad node, including attributes, resources, drivers, host volumes, and metadata.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("node_id"), Description: "Node ID", Required: true}},
	},
	{
		Name: mcp.ToolName("nomad_get_node_allocations"), Description: "List all allocations placed on a specific Nomad node. Shows what workloads are running on the node.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("node_id"), Description: "Node ID", Required: true}},
	},
	{
		Name: mcp.ToolName("nomad_drain_node"), Description: "Enable or disable drain mode on a Nomad node. Draining migrates all allocations off the node for maintenance.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("node_id"), Description: "Node ID", Required: true}, {Name: mcp.ParamName("enable"), Description: "Enable drain (true) or disable (false)", Required: true}, {Name: mcp.ParamName("deadline"), Description: "Drain deadline duration (e.g., '1h', '30m'). Use -1 for no deadline"}, {Name: mcp.ParamName("ignore_system_jobs"), Description: "Skip draining system jobs (true/false, default: false)"}},
	},
	{
		Name: mcp.ToolName("nomad_node_eligibility"), Description: "Toggle scheduling eligibility for a Nomad node. Ineligible nodes won't receive new allocations but keep existing ones.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("node_id"), Description: "Node ID", Required: true}, {Name: mcp.ParamName("eligible"), Description: "Set eligible (true) or ineligible (false)",

		// ── Deployments ──────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("nomad_list_deployments"), Description: "List deployments across the Nomad cluster. Shows rolling update status, canary progress, and deployment health.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}, {Name: mcp.ParamName("prefix"), Description: "Filter by deployment ID prefix"}},
	},
	{
		Name: mcp.ToolName("nomad_get_deployment"), Description: "Get details of a specific Nomad deployment, including task group status, health, and canary information.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("nomad_promote_deployment"), Description: "Promote canary allocations in a Nomad deployment. Moves canaries to production after validation.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}, {Name: mcp.ParamName("all"), Description: "Promote all task groups (true/false, default: true)"}, {Name: mcp.ParamName("groups"), Description: "Comma-separated list of task groups to promote (alternative to all)"}},
	},
	{
		Name: mcp.ToolName("nomad_fail_deployment"), Description: "Mark a Nomad deployment as failed, triggering automatic rollback to the previous job version.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}},
	},

	// ── Evaluations ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_list_evaluations"), Description: "List evaluations in the Nomad scheduler queue. Shows scheduling decisions, blocked evaluations, and failures.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}, {Name: mcp.ParamName("prefix"), Description: "Filter by evaluation ID prefix"}, {Name: mcp.

		// ── Services ─────────────────────────────────────────────────────
		ParamName("filter"), Description: `Filter expression (e.g., Status == "blocked")`}},
	},

	{
		Name: mcp.ToolName("nomad_list_services"), Description: "List all registered services in the Nomad service discovery catalog.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("namespace"), Description: "Namespace (default: default)"}},
	},

	// ── Cluster ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("nomad_get_agent_self"), Description: "Get the current Nomad agent's configuration, stats, and node information. Useful for diagnosing agent state.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("nomad_get_cluster_status"), Description: "Get Nomad cluster status including Raft leader address and peer list. Use for cluster health checks.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("nomad_gc"), Description: "Trigger garbage collection on the Nomad cluster to clean up dead allocations, evaluations, and deployments.",
		Parameters: []mcp.Parameter{},
	},
}
