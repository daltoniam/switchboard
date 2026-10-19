package dbt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/itchyny/gojq"
)

// Run status codes from the Admin API v2. Run steps use the same codes.
const (
	statusSuccess = 10
	statusError   = 20
)

var runStatusCodes = map[string]int{
	"queued":    1,
	"starting":  2,
	"running":   3,
	"success":   statusSuccess,
	"error":     statusError,
	"cancelled": 30,
}

// includeRelated renders dbt's include_related list form: ['a','b'].
func includeRelated(items ...string) string {
	return "['" + strings.Join(items, "','") + "']"
}

type runStep struct {
	ID              int64  `json:"id"`
	Index           int    `json:"index"`
	Name            string `json:"name"`
	Status          int    `json:"status"`
	StatusHumanized string `json:"status_humanized"`
	Logs            string `json:"logs"`
}

type run struct {
	ID              int64     `json:"id"`
	JobDefinitionID int64     `json:"job_definition_id"`
	Status          int       `json:"status"`
	StatusHumanized string    `json:"status_humanized"`
	GitSHA          string    `json:"git_sha"`
	GitBranch       string    `json:"git_branch"`
	CreatedAt       string    `json:"created_at"`
	FinishedAt      string    `json:"finished_at"`
	Href            string    `json:"href"`
	ArtifactsSaved  bool      `json:"artifacts_saved"`
	RunSteps        []runStep `json:"run_steps"`
}

// runRef is the compact run reference returned by computed tools.
type runRef struct {
	ID         int64  `json:"id"`
	Status     int    `json:"status"`
	GitSHA     string `json:"git_sha"`
	FinishedAt string `json:"finished_at,omitempty"`
}

func refOf(r run) *runRef {
	return &runRef{ID: r.ID, Status: r.Status, GitSHA: r.GitSHA, FinishedAt: r.FinishedAt}
}

func listResult(ctx context.Context, d *dbt, apiPath string, q url.Values, key string) (*mcp.ToolResult, error) {
	env, err := d.getEnvelope(ctx, apiPath, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var items []any
	if err := json.Unmarshal(env.Data, &items); err != nil {
		return mcp.ErrResult(fmt.Errorf("dbt: decode %s: %w", key, err))
	}
	return mcp.JSONResult(map[string]any{
		key:           items,
		"total_count": env.Extra.Pagination.TotalCount,
	})
}

func parseStatus(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if code, ok := runStatusCodes[s]; ok {
		return code, nil
	}
	if code, err := strconv.Atoi(s); err == nil {
		for _, known := range runStatusCodes {
			if code == known {
				return code, nil
			}
		}
	}
	return 0, fmt.Errorf("unknown status %q: use queued, starting, running, success, error, or cancelled", s)
}

func listJobsRuns(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	jobID := r.Str("job_id")
	status := r.Str("status")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.OptInt("offset", 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{
		"limit":           {strconv.Itoa(clampLimit(limit))},
		"offset":          {strconv.Itoa(offset)},
		"order_by":        {"-id"},
		"include_related": {includeRelated("trigger")},
	}
	if jobID != "" {
		q.Set("job_definition_id", jobID)
	}
	if status != "" {
		code, err := parseStatus(status)
		if err != nil {
			return mcp.ErrResult(err)
		}
		q.Set("status", strconv.Itoa(code))
	}
	return listResult(ctx, d, "/runs/", q, "runs")
}

func runIDArg(args map[string]any) (int64, error) {
	r := mcp.NewArgs(args)
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return 0, err
	}
	if runID <= 0 {
		return 0, fmt.Errorf("run_id is required")
	}
	return runID, nil
}

func (d *dbt) getRun(ctx context.Context, runID int64, related ...string) (run, error) {
	var out run
	var q url.Values
	if len(related) > 0 {
		q = url.Values{"include_related": {includeRelated(related...)}}
	}
	err := d.getInto(ctx, fmt.Sprintf("/runs/%d/", runID), q, &out)
	return out, err
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n-- truncated --"
}

type runResults struct {
	Results []struct {
		UniqueID      string  `json:"unique_id"`
		Status        string  `json:"status"`
		Message       string  `json:"message"`
		Failures      *int    `json:"failures"`
		RelationName  string  `json:"relation_name"`
		CompiledCode  string  `json:"compiled_code"`
		ExecutionTime float64 `json:"execution_time"`
	} `json:"results"`
}

// freshnessResults is the subset of sources.json that `dbt source freshness`
// writes; its failures never appear in run_results.json.
type freshnessResults struct {
	Results []freshnessResult `json:"results"`
}

type freshnessResult struct {
	UniqueID       string  `json:"unique_id"`
	Status         string  `json:"status"`
	MaxLoadedAt    string  `json:"max_loaded_at"`
	SecondsAgo     float64 `json:"max_loaded_at_time_ago_in_s"`
	Error          string  `json:"error"`
	FreshnessRules struct {
		ErrorAfter freshnessThreshold `json:"error_after"`
	} `json:"criteria"`
}

type freshnessThreshold struct {
	Count  *int   `json:"count"`
	Period string `json:"period"`
}

func freshnessMessage(r freshnessResult) string {
	if r.Error != "" {
		return "freshness check failed: " + r.Error
	}
	msg := fmt.Sprintf("source is stale: last loaded %s (%.1fh ago)", r.MaxLoadedAt, r.SecondsAgo/3600)
	if t := r.FreshnessRules.ErrorAfter; t.Count != nil {
		msg += fmt.Sprintf(", error_after %d %s", *t.Count, t.Period)
	}
	return msg
}

// decodeArtifact streams one run-step artifact into v.
func (d *dbt) decodeArtifact(ctx context.Context, runID int64, name string, step int, v any) error {
	body, err := d.open(ctx, artifactPath(runID, name), stepQuery(step))
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	if err := json.NewDecoder(io.LimitReader(body, maxArtifactSize)).Decode(v); err != nil {
		return fmt.Errorf("dbt: decode %s: %w", name, err)
	}
	return nil
}

// failingStatuses are run_results and sources.json statuses that broke the run.
var failingStatuses = map[string]bool{"error": true, "fail": true, "runtime error": true}

type failedNode struct {
	UniqueID         string `json:"unique_id"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	Failures         *int   `json:"failures,omitempty"`
	OriginalFilePath string `json:"original_file_path,omitempty"`
	ResourceType     string `json:"resource_type,omitempty"`
	Materialized     string `json:"materialized,omitempty"`
	RelationName     string `json:"relation_name,omitempty"`
	CompiledCode     string `json:"compiled_code,omitempty"`
	MaxLoadedAt      string `json:"max_loaded_at,omitempty"`
	Step             int    `json:"step"`
}

type failedStep struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	LogTail string `json:"log_tail"`
}

func getJobRunError(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	runID, err := runIDArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	rn, err := d.getRun(ctx, runID, "run_steps")
	if err != nil {
		return mcp.ErrResult(err)
	}

	out := map[string]any{
		"run_id":                rn.ID,
		"job_id":                rn.JobDefinitionID,
		"status":                rn.StatusHumanized,
		"git_sha":               rn.GitSHA,
		"git_branch":            rn.GitBranch,
		"finished_at":           rn.FinishedAt,
		"href":                  rn.Href,
		"run_results_available": false,
	}
	if rn.Status != statusError {
		out["note"] = "run is not in error status"
	}

	steps := []failedStep{}
	nodes := []failedNode{}
	var warnings []string
	pending := false
	for _, st := range rn.RunSteps {
		if st.Status != statusError {
			continue
		}
		steps = append(steps, failedStep{Index: st.Index, Name: st.Name, Status: st.StatusHumanized, LogTail: tailLines(st.Logs, logTailLines)})

		found, err := d.stepFailures(ctx, rn.ID, st.Index)
		var nf *errNotFound
		switch {
		case errors.As(err, &nf):
			// Artifacts land after the job.run.errored webhook fires; until
			// then a 404 means "not yet", otherwise the step produced none
			// (parse or compile failures before any node ran).
			if !rn.ArtifactsSaved {
				pending = true
			}
		case err != nil:
			return mcp.ErrResult(err)
		default:
			out["run_results_available"] = true
			nodes = append(nodes, found.nodes...)
			if found.warning != "" {
				warnings = append(warnings, found.warning)
			}
		}
	}
	out["failed_steps"] = steps
	out["failed_nodes"] = nodes
	if pending {
		out["artifacts_pending"] = true
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	return mcp.JSONResult(out)
}

type stepFailureResult struct {
	nodes   []failedNode
	warning string
}

// stepFailures reads the step's run_results.json and joins each failing node
// to its source file via the same step's manifest.json.
func (d *dbt) stepFailures(ctx context.Context, runID int64, step int) (stepFailureResult, error) {
	var rr runResults
	rrErr := d.decodeArtifact(ctx, runID, "run_results.json", step, &rr)
	var fr freshnessResults
	frErr := d.decodeArtifact(ctx, runID, "sources.json", step, &fr)
	var nf *errNotFound
	switch {
	case rrErr != nil && !errors.As(rrErr, &nf):
		return stepFailureResult{}, rrErr
	case frErr != nil && !errors.As(frErr, &nf):
		return stepFailureResult{}, frErr
	case rrErr != nil && frErr != nil:
		return stepFailureResult{}, rrErr // neither artifact exists for this step
	}

	var res stepFailureResult
	for _, r := range fr.Results {
		if !failingStatuses[r.Status] {
			continue
		}
		res.nodes = append(res.nodes, failedNode{
			UniqueID:    r.UniqueID,
			Status:      r.Status,
			Message:     freshnessMessage(r),
			MaxLoadedAt: r.MaxLoadedAt,
			Step:        step,
		})
	}
	for _, r := range rr.Results {
		if !failingStatuses[r.Status] {
			continue
		}
		res.nodes = append(res.nodes, failedNode{
			UniqueID:     r.UniqueID,
			Status:       r.Status,
			Message:      r.Message,
			Failures:     r.Failures,
			RelationName: r.RelationName,
			CompiledCode: truncate(r.CompiledCode, compiledCodeMaxLen),
			Step:         step,
		})
	}
	if len(res.nodes) == 0 {
		return res, nil
	}

	m, err := d.loadManifest(ctx, runID, step)
	if err != nil {
		res.warning = fmt.Sprintf("could not load manifest.json for step %d to resolve file paths: %v", step, err)
		return res, nil
	}
	for i := range res.nodes {
		if n, ok := m.lookup(res.nodes[i].UniqueID); ok {
			res.nodes[i].OriginalFilePath = n.OriginalFilePath
			res.nodes[i].ResourceType = n.ResourceType
			res.nodes[i].Materialized = n.Config.Materialized
		}
	}
	return res, nil
}

// regressionScanLimit bounds how far back the regression search looks for
// the last successful run of the job.
const regressionScanLimit = 100

func getJobRunRegressionRange(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	runID, err := runIDArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	failing, err := d.getRun(ctx, runID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var history []run
	q := url.Values{
		"job_definition_id": {strconv.FormatInt(failing.JobDefinitionID, 10)},
		"order_by":          {"-id"},
		"limit":             {strconv.Itoa(regressionScanLimit)},
	}
	if err := d.getInto(ctx, "/runs/", q, &history); err != nil {
		return mcp.ErrResult(err)
	}

	// Walk back from the failing run: every errored run until the most
	// recent success belongs to the same regression window.
	var lastSuccess, firstFailing *runRef
	failingCount := 0
	for _, h := range history {
		if h.ID > failing.ID {
			continue
		}
		if h.Status == statusSuccess {
			lastSuccess = refOf(h)
			break
		}
		if h.Status == statusError {
			failingCount++
			firstFailing = refOf(h)
		}
	}
	if firstFailing == nil {
		firstFailing = refOf(failing)
	}

	out := map[string]any{
		"run_id":                     failing.ID,
		"job_id":                     failing.JobDefinitionID,
		"failing_run":                refOf(failing),
		"first_failing_run":          firstFailing,
		"last_success_run":           lastSuccess,
		"failing_runs_since_success": failingCount,
		"compare_head":               firstFailing.GitSHA,
	}
	if lastSuccess != nil {
		out["compare_base"] = lastSuccess.GitSHA
		if lastSuccess.GitSHA == firstFailing.GitSHA {
			out["note"] = "the last successful run used the same commit, so the failure is not caused by a code change (look at data, sources, or infrastructure)"
		}
	} else {
		out["note"] = fmt.Sprintf("no successful run found in the last %d runs of this job", regressionScanLimit)
	}
	return mcp.JSONResult(out)
}

func validArtifactPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return false
	}
	return path.Clean(p) == p && !strings.HasPrefix(p, "..")
}

func getJobRunArtifact(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	runID, err := runIDArg(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	artifact := r.Str("path")
	step := r.Int("step")
	filter := r.Str("jq")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if !validArtifactPath(artifact) {
		return mcp.ErrResult(fmt.Errorf("invalid artifact path %q: use a relative path such as run_results.json", artifact))
	}
	var query *gojq.Code
	if filter != "" {
		parsed, err := gojq.Parse(filter)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid jq filter: %w", err))
		}
		if query, err = gojq.Compile(parsed); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid jq filter: %w", err))
		}
	}

	body, err := d.open(ctx, artifactPath(runID, artifact), stepQuery(step))
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer func() { _ = body.Close() }()

	if query == nil {
		data, err := io.ReadAll(io.LimitReader(body, int64(d.inlineArtifactMax)+1))
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(data) > d.inlineArtifactMax {
			return mcp.ErrResult(fmt.Errorf("%s is larger than %d bytes; pass jq to select what you need (e.g. '.results[] | select(.status != \"success\")' or '.nodes[\"model.<project>.<name>\"]'), or use dbt_get_manifest_nodes / dbt_get_model_lineage for manifest lineage", artifact, d.inlineArtifactMax))
		}
		return mcp.RawResult(data)
	}

	var doc any
	dec := json.NewDecoder(io.LimitReader(body, maxArtifactSize))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("dbt: %s is not JSON, jq cannot apply: %w", artifact, err))
	}
	results := []any{}
	iter := query.RunWithContext(ctx, normalizeNumbers(doc))
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return mcp.ErrResult(fmt.Errorf("jq: %w", err))
		}
		results = append(results, v)
	}
	if len(results) == 1 {
		return mcp.JSONResult(results[0])
	}
	return mcp.JSONResult(results)
}

// normalizeNumbers converts json.Number into types gojq accepts (int or
// float64), keeping integer IDs exact.
func normalizeNumbers(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			t[k] = normalizeNumbers(child)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = normalizeNumbers(child)
		}
		return t
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		f, _ := t.Float64()
		return f
	default:
		return v
	}
}

// resolveManifestRun picks the run whose manifest.json backs the lineage
// tools: an explicit run_id, else the latest successful run of job_id or the
// configured prod_job_id.
func (d *dbt) resolveManifestRun(ctx context.Context, r *mcp.Args) (run, error) {
	runID := r.Int64("run_id")
	jobID := r.Str("job_id")
	if err := r.Err(); err != nil {
		return run{}, err
	}
	if runID > 0 {
		return d.getRun(ctx, runID)
	}
	if jobID == "" {
		jobID = d.prodJobID
	}
	if jobID == "" {
		return run{}, fmt.Errorf("pass job_id or run_id, or configure prod_job_id for the dbt integration")
	}
	var runs []run
	q := url.Values{
		"job_definition_id": {jobID},
		"status":            {strconv.Itoa(statusSuccess)},
		"order_by":          {"-id"},
		"limit":             {"1"},
	}
	if err := d.getInto(ctx, "/runs/", q, &runs); err != nil {
		return run{}, err
	}
	if len(runs) == 0 {
		return run{}, fmt.Errorf("job %s has no successful runs to read manifest.json from", jobID)
	}
	return runs[0], nil
}

func getManifestNodes(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	paths := r.StrSlice("file_paths")
	names := r.StrSlice("names")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if len(paths) == 0 && len(names) == 0 {
		return mcp.ErrResult(fmt.Errorf("pass file_paths or names"))
	}
	rn, err := d.resolveManifestRun(ctx, r)
	if err != nil {
		return mcp.ErrResult(err)
	}
	m, err := d.loadManifest(ctx, rn.ID, 0)
	if err != nil {
		return mcp.ErrResult(err)
	}
	res := m.matchNodes(paths, names)
	return mcp.JSONResult(map[string]any{
		"manifest_run_id":  rn.ID,
		"manifest_git_sha": rn.GitSHA,
		"project_name":     m.Metadata.ProjectName,
		"nodes":            res.Nodes,
		"unmatched":        res.Unmatched,
	})
}

func getModelLineage(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	uniqueIDs := r.StrSlice("unique_ids")
	names := r.StrSlice("names")
	direction := r.Str("direction")
	depth := 2
	if _, ok := args["depth"]; ok {
		depth = r.Int("depth")
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if len(uniqueIDs) == 0 && len(names) == 0 {
		return mcp.ErrResult(fmt.Errorf("pass unique_ids or names"))
	}
	switch direction {
	case "", "down", "downstream":
		direction = "down"
	case "up", "upstream":
		direction = "up"
	default:
		return mcp.ErrResult(fmt.Errorf("direction must be down or up, got %q", direction))
	}
	rn, err := d.resolveManifestRun(ctx, r)
	if err != nil {
		return mcp.ErrResult(err)
	}
	m, err := d.loadManifest(ctx, rn.ID, 0)
	if err != nil {
		return mcp.ErrResult(err)
	}
	ids, unmatched := m.resolveIDs(uniqueIDs, names)
	res := m.lineage(ids, direction, depth, defaultLineageMax)
	return mcp.JSONResult(map[string]any{
		"manifest_run_id":  rn.ID,
		"manifest_git_sha": rn.GitSHA,
		"direction":        res.Direction,
		"depth":            res.Depth,
		"start":            res.Start,
		"unmatched":        unmatched,
		"nodes":            res.Nodes,
		"tests":            res.Tests,
		"exposures":        res.Exposures,
		"truncated":        res.Truncated,
	})
}
