package nomad

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Jobs ─────────────────────────────────────────────────────────
	mcp.ToolName("nomad_list_jobs"):        {"ID", "Name", "Type", "Status", "StatusDescription", "Priority", "Namespace", "Datacenters", "SubmitTime", "JobSummary.Summary"},
	mcp.ToolName("nomad_get_job"):          {"ID", "Name", "Type", "Status", "Priority", "Namespace", "Datacenters", "TaskGroups[].Name", "TaskGroups[].Count", "TaskGroups[].Tasks[].Name", "TaskGroups[].Tasks[].Driver", "TaskGroups[].Tasks[].Resources", "Constraints", "SubmitTime", "Version"},
	mcp.ToolName("nomad_get_job_versions"): {"Versions[].ID", "Versions[].Version", "Versions[].SubmitTime", "Versions[].Status", "Versions[].Stable"},

	// ── Allocations ──────────────────────────────────────────────────
	mcp.ToolName("nomad_list_allocations"):    {"ID", "JobID", "TaskGroup", "NodeID", "NodeName", "DesiredStatus", "ClientStatus", "ClientDescription", "CreateTime", "ModifyTime"},
	mcp.ToolName("nomad_get_allocation"):      {"ID", "JobID", "TaskGroup", "NodeID", "NodeName", "DesiredStatus", "ClientStatus", "ClientDescription", "TaskStates", "CreateTime", "ModifyTime", "Resources", "DeploymentID"},
	mcp.ToolName("nomad_get_job_allocations"): {"ID", "TaskGroup", "NodeID", "NodeName", "DesiredStatus", "ClientStatus", "ClientDescription", "CreateTime"},

	// ── Nodes ────────────────────────────────────────────────────────
	mcp.ToolName("nomad_list_nodes"):           {"ID", "Name", "Address", "Datacenter", "NodeClass", "NodePool", "Status", "StatusDescription", "Drain", "SchedulingEligibility", "Drivers", "Version"},
	mcp.ToolName("nomad_get_node"):             {"ID", "Name", "Address", "Datacenter", "NodeClass", "NodePool", "Status", "Drain", "SchedulingEligibility", "Drivers", "HostVolumes", "Attributes", "Resources", "Reserved", "Meta", "Version"},
	mcp.ToolName("nomad_get_node_allocations"): {"ID", "JobID", "TaskGroup", "DesiredStatus", "ClientStatus", "ClientDescription", "CreateTime"},

	// ── Deployments ──────────────────────────────────────────────────
	mcp.ToolName("nomad_list_deployments"): {"ID", "JobID", "Namespace", "Status", "StatusDescription", "TaskGroups", "IsMultiregion", "CreateIndex", "ModifyIndex"},
	mcp.ToolName("nomad_get_deployment"):   {"ID", "JobID", "Namespace", "Status", "StatusDescription", "TaskGroups", "IsMultiregion", "JobVersion", "JobCreateIndex"},

	// ── Evaluations ──────────────────────────────────────────────────
	mcp.ToolName("nomad_list_evaluations"): {"ID", "JobID", "Type", "Status", "StatusDescription", "Priority", "TriggeredBy", "CreateTime", "ModifyTime"},

	// ── Services ─────────────────────────────────────────────────────
	mcp.ToolName("nomad_list_services"): {"Namespace", "Services[].ServiceName", "Services[].Tags"},

	// ── Cluster ──────────────────────────────────────────────────────
	mcp.ToolName("nomad_get_agent_self"):     {"config.Datacenter", "config.Region", "config.Version", "config.Server.Enabled", "config.Client.Enabled", "member.Name", "member.Addr", "member.Status", "stats.nomad", "stats.raft"},
	mcp.ToolName("nomad_get_cluster_status"): {"leader", "peers"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("nomad: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
