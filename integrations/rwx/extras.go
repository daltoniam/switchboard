package rwx

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getArtifacts(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	ra := mcp.NewArgs(args)
	runIDRaw := ra.Str("run_id")
	download := ra.Bool("download")
	artifactKey := ra.Str("artifact_key")
	taskKey := ra.Str("task_key")
	taskIDRaw := ra.Str("task_id")
	if err := ra.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	id := extractRunID(runIDRaw)
	taskID := extractRunID(taskIDRaw)
	if id == "" && taskID == "" {
		return mcp.ErrResult(fmt.Errorf("either run_id or task_id is required"))
	}
	runURL := fmt.Sprintf("%s/mint/%s/runs/%s", r.baseURL, r.org, id)

	query := url.Values{}
	if taskID != "" {
		query.Set("task_id", taskID)
	} else {
		query.Set("run_id", id)
		if taskKey != "" {
			query.Set("task_key", taskKey)
		}
	}

	var artifacts []rwxArtifactDownload
	if artifactKey != "" {
		query.Set("key", artifactKey)
		var artifact rwxArtifactDownload
		if err := r.apiGetJSON(ctx, "/mint/api/artifact_download", query, &artifact); err != nil {
			return mcp.ErrResult(err)
		}
		artifacts = append(artifacts, artifact)
	} else {
		if err := r.apiGetJSON(ctx, "/mint/api/artifact_downloads", query, &artifacts); err != nil {
			return mcp.ErrResult(err)
		}
	}

	action := "listed"
	if download {
		action = "downloaded"
	}

	items := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		item := artifact.toMap(!download)
		if download {
			content, err := r.downloadArtifact(ctx, artifact)
			if err != nil {
				return mcp.ErrResult(err)
			}
			item["content"] = content
		}
		items = append(items, item)
	}

	resp := map[string]any{
		"run_id":    id,
		"url":       runURL,
		"action":    action,
		"artifacts": items,
		"count":     len(items),
	}
	if taskID != "" {
		resp["task_id"] = taskID
	}
	if taskKey != "" {
		resp["task_key"] = taskKey
	}
	if !download {
		resp["hint"] = "Set download=true to fetch artifact content. Tokens are intentionally omitted from listed results."
	}
	return mcp.JSONResult(resp)
}

func validateWorkflow(_ context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	filePath, err := mcp.ArgStr(args, "file_path")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		filePath = ".rwx/ci.yml"
	}

	output, err := r.runRWXCommand([]string{"lint", filePath, "--output", "json"}, 30000)
	if err != nil {
		return mcp.JSONResult(map[string]any{
			"isValid": false,
			"errors": []map[string]string{
				{"severity": "error", "message": err.Error()},
			},
			"warnings": []any{},
		})
	}
	if output != "" {
		return &mcp.ToolResult{Data: output}, nil
	}
	return mcp.JSONResult(map[string]any{
		"isValid":  true,
		"errors":   []any{},
		"warnings": []any{},
	})
}

func verifyCLI(_ context.Context, r *rwx, _ map[string]any) (*mcp.ToolResult, error) {
	check := r.getRWXCLIVersion()

	if !check.installed {
		return mcp.JSONResult(map[string]any{
			"status":               "not_installed",
			"message":              fmt.Sprintf("rwx CLI is not installed. Please install version >= %s.", minRWXVersion),
			"install_instructions": "Visit https://github.com/rwx-research/rwx-cli/releases or use: brew install rwx-research/tap/rwx",
		})
	}

	if !check.meetsMinimum {
		return mcp.JSONResult(map[string]any{
			"status":               "outdated",
			"current_version":      check.version,
			"required_version":     minRWXVersion,
			"message":              fmt.Sprintf("rwx CLI version %s is below minimum required version %s. Please upgrade.", check.version, minRWXVersion),
			"install_instructions": "Visit https://github.com/rwx-research/rwx-cli/releases or use: brew upgrade rwx-research/tap/rwx",
		})
	}

	return mcp.JSONResult(map[string]any{
		"status":  "ready",
		"version": check.version,
		"message": fmt.Sprintf("rwx CLI version %s is installed and ready.", check.version),
	})
}

// --- CLI version check ---

type rwxVersionCheck struct {
	installed    bool
	version      string
	meetsMinimum bool
}

func (r *rwx) getRWXCLIVersion() rwxVersionCheck {
	output, err := exec.Command(r.cliPath, "--version").CombinedOutput() // #nosec G204 -- resolved binary path
	if err != nil {
		return rwxVersionCheck{installed: false}
	}

	versionStr := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(versionStr)
	if match == nil {
		return rwxVersionCheck{installed: true}
	}

	version := match[1]
	return rwxVersionCheck{
		installed:    true,
		version:      version,
		meetsMinimum: isVersionGTE(version, minRWXVersion),
	}
}

func isVersionGTE(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var va, vb int
		if i < len(partsA) {
			_, _ = fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			_, _ = fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va > vb {
			return true
		}
		if va < vb {
			return false
		}
	}
	return true
}
