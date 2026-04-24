package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func listJobs(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	prefix := r.Str("prefix")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns, "prefix": prefix, "filter": filter})
	data, err := n.get(ctx, "/v1/jobs%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getJob(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.get(ctx, "/v1/job/%s%s", url.PathEscape(jobID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getJobVersions(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.get(ctx, "/v1/job/%s/versions%s", url.PathEscape(jobID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func registerJob(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	jobSpec, ok := args["job"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("job is required"))
	}

	body := map[string]any{"Job": jobSpec}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.post(ctx, "/v1/jobs"+q, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func stopJob(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	purge := r.Str("purge")
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns, "purge": purge})
	data, err := n.del(ctx, "/v1/job/%s%s", url.PathEscape(jobID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func forceEvaluate(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.post(ctx, fmt.Sprintf("/v1/job/%s/evaluate%s", url.PathEscape(jobID), q), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getJobAllocations(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.get(ctx, "/v1/job/%s/allocations%s", url.PathEscape(jobID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Allocations ──────────────────────────────────────────────────────

func listAllocations(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	prefix := r.Str("prefix")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns, "prefix": prefix, "filter": filter})
	data, err := n.get(ctx, "/v1/allocations%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAllocation(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	allocID := r.Str("alloc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.get(ctx, "/v1/allocation/%s", url.PathEscape(allocID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func stopAllocation(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	allocID := r.Str("alloc_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.post(ctx, fmt.Sprintf("/v1/allocation/%s/stop", url.PathEscape(allocID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func restartAllocation(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	allocID := r.Str("alloc_id")
	task := r.Str("task")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if task != "" {
		body["TaskName"] = task
	}
	data, err := n.put(ctx, fmt.Sprintf("/v1/client/allocation/%s/restart", url.PathEscape(allocID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func readAllocationLogs(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	allocID := r.Str("alloc_id")
	task := r.Str("task")
	logType := r.Str("log_type")
	plain := r.Str("plain")
	origin := r.Str("origin")
	offset := r.Str("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if logType == "" {
		logType = "stdout"
	}
	if plain == "" {
		plain = "true"
	}
	if origin == "" {
		origin = "end"
	}
	params := map[string]string{
		"task":   task,
		"type":   logType,
		"plain":  plain,
		"origin": origin,
		"offset": offset,
	}
	q := queryEncode(params)
	data, err := n.get(ctx, "/v1/client/fs/logs/%s%s", url.PathEscape(allocID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	// When plain=true, Nomad returns raw text, not JSON
	if strings.ToLower(plain) == "true" {
		return &mcp.ToolResult{Data: string(data)}, nil
	}
	return mcp.RawResult(data)
}

// ── Nodes ────────────────────────────────────────────────────────────

func listNodes(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	prefix := r.Str("prefix")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"prefix": prefix, "filter": filter})
	data, err := n.get(ctx, "/v1/nodes%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getNode(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nodeID := r.Str("node_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.get(ctx, "/v1/node/%s", url.PathEscape(nodeID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getNodeAllocations(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nodeID := r.Str("node_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.get(ctx, "/v1/node/%s/allocations", url.PathEscape(nodeID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func drainNode(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nodeID := r.Str("node_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	enable, err := mcp.ArgBool(args, "enable")
	if err != nil {
		return mcp.ErrResult(err)
	}

	deadline := r.Str("deadline")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ignoreSystemJobs, _ := mcp.ArgBool(args, "ignore_system_jobs")

	var body map[string]any
	if enable {
		drainSpec := map[string]any{}
		if deadline != "" {
			drainSpec["Deadline"] = parseDuration(deadline)
		}
		if ignoreSystemJobs {
			drainSpec["IgnoreSystemJobs"] = true
		}
		body = map[string]any{
			"DrainSpec": drainSpec,
		}
	} else {
		body = map[string]any{
			"DrainSpec": nil,
		}
	}

	data, err := n.post(ctx, fmt.Sprintf("/v1/node/%s/drain", url.PathEscape(nodeID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func nodeEligibility(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	nodeID := r.Str("node_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	eligible, err := mcp.ArgBool(args, "eligible")
	if err != nil {
		return mcp.ErrResult(err)
	}

	eligibility := "ineligible"
	if eligible {
		eligibility = "eligible"
	}
	body := map[string]any{
		"Eligibility": eligibility,
	}
	data, err := n.post(ctx, fmt.Sprintf("/v1/node/%s/eligibility", url.PathEscape(nodeID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Deployments ──────────────────────────────────────────────────────

func listDeployments(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	prefix := r.Str("prefix")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns, "prefix": prefix})
	data, err := n.get(ctx, "/v1/deployments%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDeployment(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.get(ctx, "/v1/deployment/%s", url.PathEscape(deploymentID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func promoteDeployment(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	all := r.Str("all")
	groups := r.Str("groups")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"DeploymentID": deploymentID,
		"All":          all != "false",
	}
	if groups != "" {
		body["Groups"] = strings.Split(groups, ",")
		body["All"] = false
	}
	data, err := n.post(ctx, fmt.Sprintf("/v1/deployment/promote/%s", url.PathEscape(deploymentID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func failDeployment(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deploymentID := r.Str("deployment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"DeploymentID": deploymentID,
	}
	data, err := n.post(ctx, fmt.Sprintf("/v1/deployment/fail/%s", url.PathEscape(deploymentID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Evaluations ──────────────────────────────────────────────────────

func listEvaluations(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	prefix := r.Str("prefix")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns, "prefix": prefix, "filter": filter})
	data, err := n.get(ctx, "/v1/evaluations%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Services ─────────────────────────────────────────────────────────

func listServices(ctx context.Context, n *nomad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ns := r.Str("namespace")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"namespace": ns})
	data, err := n.get(ctx, "/v1/services%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Cluster ──────────────────────────────────────────────────────────

func getAgentSelf(ctx context.Context, n *nomad, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := n.get(ctx, "/v1/agent/self")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getClusterStatus(ctx context.Context, n *nomad, _ map[string]any) (*mcp.ToolResult, error) {
	leader, err := n.get(ctx, "/v1/status/leader")
	if err != nil {
		return mcp.ErrResult(err)
	}
	peers, err := n.get(ctx, "/v1/status/peers")
	if err != nil {
		return mcp.ErrResult(err)
	}
	combined := map[string]json.RawMessage{
		"leader": leader,
		"peers":  peers,
	}
	return mcp.JSONResult(combined)
}

func gc(ctx context.Context, n *nomad, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := n.put(ctx, "/v1/system/gc", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// parseDuration converts a human-readable duration (e.g., "1h", "30m", "1h30m") to
// nanoseconds for the Nomad API. Returns 0 for empty or invalid input; "-1" maps to
// -1 (no deadline).
func parseDuration(s string) int64 {
	if s == "" {
		return 0
	}
	if s == "-1" {
		return -1
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d.Nanoseconds()
}
