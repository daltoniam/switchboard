package rwx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func launchCIRun(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	cmdArgs := []string{"run", ".rwx/ci.yml", "--output", "json"}

	wait := argBool(args, "wait")
	if wait {
		cmdArgs = append(cmdArgs, "--wait")
	}

	targets := argStrSlice(args, "targets")
	for _, t := range targets {
		cmdArgs = append(cmdArgs, "--target", t)
	}

	var timeoutMs int
	if wait {
		timeoutMs = 30 * 60 * 1000 // 30 min
	}

	output, err := r.runRWXCommand(cmdArgs, timeoutMs)
	if err != nil {
		return errResult(err)
	}

	var parsed struct {
		RunID     string `json:"run_id"`
		RunURL    string `json:"run_url"`
		Result    string `json:"result"`
		Execution string `json:"execution"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return errResult(fmt.Errorf("parse run output: %w", err))
	}

	runURL := parsed.RunURL
	if runURL == "" {
		runURL = fmt.Sprintf("%s/mint/%s/runs/%s", rwxAPIBase, rwxOrg, parsed.RunID)
	}

	if wait {
		status := normalizeStatus(parsed.Result)
		resp := map[string]any{
			"completed": true,
			"run_id":    parsed.RunID,
			"status":    status,
			"url":       runURL,
		}
		if status == "failure" {
			resp["next_step"] = "Use rwx_get_run_results to see task failures, or rwx_grep_logs to search for errors"
		} else {
			resp["next_step"] = "Run completed successfully"
		}
		result, _ := jsonResult(resp)
		if status == "failure" {
			result.IsError = true
		}
		return result, nil
	}

	return jsonResult(map[string]any{
		"completed": false,
		"run_id":    parsed.RunID,
		"status":    "launched",
		"url":       runURL,
		"next_step": "Use rwx_wait_for_ci_run to wait for completion, or launch with wait=true",
	})
}

func waitForCIRun(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "run_id"))
	runURL := fmt.Sprintf("%s/mint/%s/runs/%s", rwxAPIBase, rwxOrg, id)

	timeoutSec := argInt(args, "timeout_seconds")
	if timeoutSec <= 0 {
		timeoutSec = 1800
	}
	pollSec := argInt(args, "poll_interval_seconds")
	if pollSec <= 0 {
		pollSec = 30
	}

	start := time.Now()
	maxDuration := time.Duration(timeoutSec) * time.Second
	pollCount := 0
	consecutiveErrors := 0

	for time.Since(start) < maxDuration {
		pollCount++
		status, isComplete, err := fetchRunStatus(ctx, r, id)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= 5 {
				return errResult(fmt.Errorf("failed to fetch run status after 5 consecutive errors: %w", err))
			}
		} else {
			consecutiveErrors = 0
			if isComplete {
				return jsonResult(map[string]any{
					"completed":       true,
					"run_id":          id,
					"run_url":         runURL,
					"status":          status,
					"elapsed_seconds": int(time.Since(start).Seconds()),
					"polls":           pollCount,
				})
			}
		}

		select {
		case <-ctx.Done():
			return errResult(ctx.Err())
		case <-time.After(time.Duration(pollSec) * time.Second):
		}
	}

	return jsonResult(map[string]any{
		"completed":       false,
		"timeout":         true,
		"run_id":          id,
		"run_url":         runURL,
		"elapsed_seconds": int(time.Since(start).Seconds()),
		"polls":           pollCount,
		"message":         fmt.Sprintf("Run did not complete within %d seconds", timeoutSec),
	})
}

func getRecentRuns(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ref := argStr(args, "ref")
	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 5
	}

	fetchLimit := limit * 10
	if fetchLimit > 100 {
		fetchLimit = 100
	}

	apiURL := fmt.Sprintf("%s/mint/api/runs?limit=%d", rwxAPIBase, fetchLimit)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return errResult(err)
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return errResult(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("RWX API error (%d): %s", resp.StatusCode, string(body))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return errResult(re)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errResult(fmt.Errorf("RWX API error (%d): %s", resp.StatusCode, string(body)))
	}

	var data struct {
		Runs []struct {
			ID              string `json:"id"`
			Branch          string `json:"branch"`
			CommitSHA       string `json:"commit_sha"`
			ResultStatus    string `json:"result_status"`
			ExecutionStatus string `json:"execution_status"`
			Title           string `json:"title"`
			Trigger         string `json:"trigger"`
			DefinitionPath  string `json:"definition_path"`
		} `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return errResult(fmt.Errorf("parse runs response: %w", err))
	}

	var runs []map[string]any
	for _, run := range data.Runs {
		if run.Branch != ref || run.DefinitionPath != ".rwx/ci.yml" {
			continue
		}
		status := "running"
		if run.ExecutionStatus == "finished" {
			status = normalizeStatus(run.ResultStatus)
		}
		runs = append(runs, map[string]any{
			"run_id":     run.ID,
			"status":     status,
			"commit_sha": run.CommitSHA,
			"title":      run.Title,
			"url":        fmt.Sprintf("%s/mint/%s/runs/%s", rwxAPIBase, rwxOrg, run.ID),
		})
		if len(runs) >= limit {
			break
		}
	}

	return jsonResult(map[string]any{
		"ref":   ref,
		"count": len(runs),
		"runs":  runs,
	})
}

func getRunResults(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "run_id"))
	output, err := r.runRWXCommand([]string{"results", id, "--output", "json"}, 0)
	if err != nil {
		return errResult(err)
	}

	var parsed struct {
		RunID     string `json:"run_id"`
		Result    string `json:"result"`
		Execution string `json:"execution"`
		Duration  int    `json:"duration_seconds"`
		Tasks     []struct {
			Key      string `json:"key"`
			Status   string `json:"status"`
			Duration int    `json:"duration_seconds"`
			CacheHit bool   `json:"cache_hit"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return errResult(fmt.Errorf("parse results: %w", err))
	}

	runURL := fmt.Sprintf("%s/mint/%s/runs/%s", rwxAPIBase, rwxOrg, id)
	status := normalizeStatus(parsed.Result)

	var failedKeys []string
	succeeded, failed, skipped, cached := 0, 0, 0, 0
	for _, t := range parsed.Tasks {
		switch strings.ToLower(t.Status) {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
			failedKeys = append(failedKeys, t.Key)
		case "skipped":
			skipped++
		}
		if t.CacheHit {
			cached++
		}
	}

	resp := map[string]any{
		"run_id":           id,
		"url":              runURL,
		"status":           status,
		"execution":        parsed.Execution,
		"duration_seconds": parsed.Duration,
		"summary": map[string]int{
			"total":     len(parsed.Tasks),
			"succeeded": succeeded,
			"failed":    failed,
			"skipped":   skipped,
			"cached":    cached,
		},
		"failed_tasks": failedKeys,
		"tasks":        parsed.Tasks,
	}

	result, _ := jsonResult(resp)
	if status == "failure" {
		result.IsError = true
	}
	return result, nil
}

// --- helpers ---

func fetchRunStatus(ctx context.Context, r *rwx, runID string) (status string, isComplete bool, err error) {
	apiURL := fmt.Sprintf("%s/mint/api/runs/%s", rwxAPIBase, runID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("API request failed: %d %s", resp.StatusCode, string(body))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return "", false, re
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", false, fmt.Errorf("API request failed: %d %s", resp.StatusCode, string(body))
	}

	var data struct {
		CompletedAt *string `json:"completed_at"`
		RunStatus   struct {
			Execution string `json:"execution"`
			Result    string `json:"result"`
		} `json:"run_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", false, err
	}

	isComplete = data.CompletedAt != nil || data.RunStatus.Execution == "finished"
	status = normalizeStatus(data.RunStatus.Result)
	if !isComplete {
		status = "running"
	}
	return status, isComplete, nil
}

func normalizeStatus(result string) string {
	switch strings.ToLower(result) {
	case "succeeded":
		return "success"
	case "failed":
		return "failure"
	default:
		if result == "" {
			return "unknown"
		}
		return strings.ToLower(result)
	}
}

func extractRunID(runIDOrURL string) string {
	if strings.Contains(runIDOrURL, "/") {
		parts := strings.Split(runIDOrURL, "/")
		return parts[len(parts)-1]
	}
	return runIDOrURL
}
