package rwx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	maxLinesPerPage = 50
	logsCacheTTL    = 30 * time.Minute
	maxLogSize      = 100000 // 100KB cap on full logs returned
)

// --- Log cache ---

type logCacheEntry struct {
	logs      string
	expiresAt time.Time
}

type logCache struct {
	mu      sync.RWMutex
	entries map[string]*logCacheEntry
}

func newLogCache() *logCache {
	return &logCache{entries: make(map[string]*logCacheEntry)}
}

func (c *logCache) get(id string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[id]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.logs, true
}

func (c *logCache) set(id, logs string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = &logCacheEntry{
		logs:      logs,
		expiresAt: time.Now().Add(logsCacheTTL),
	}
}

// --- Log download ---

func downloadLogs(ctx context.Context, r *rwx, id string) (string, error) {
	if cached, ok := r.logCache.get(id); ok {
		return cached, nil
	}

	logs, err := downloadLogsFromRWX(id)
	if err != nil {
		return "", err
	}

	go func() {
		status, isComplete, err := fetchRunStatus(ctx, r, id)
		_ = status
		if err == nil && isComplete {
			r.logCache.set(id, logs)
		}
	}()

	return logs, nil
}

func downloadLogsFromRWX(id string) (string, error) {
	outputDir, err := os.MkdirTemp("", fmt.Sprintf("rwx-logs-%s-", id))
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(outputDir) }()

	_, err = runRWXCommand([]string{"logs", id, "--output-dir", outputDir, "--auto-extract", "--output", "json"}, 0)
	if err != nil {
		return "", err
	}

	logFiles, err := findLogFiles(outputDir)
	if err != nil {
		return "", err
	}
	if len(logFiles) == 0 {
		return "", fmt.Errorf("no log files found in downloaded output")
	}

	if len(logFiles) == 1 {
		data, err := os.ReadFile(logFiles[0])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var contents []string
	for _, f := range logFiles {
		relPath, _ := filepath.Rel(outputDir, f)
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		contents = append(contents, fmt.Sprintf("\n=== %s ===\n%s", relPath, string(data)))
	}
	return strings.Join(contents, "\n"), nil
}

func findLogFiles(dir string) ([]string, error) {
	var results []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			sub, _ := findLogFiles(fullPath)
			results = append(results, sub...)
		} else if strings.HasSuffix(entry.Name(), ".log") || strings.HasSuffix(entry.Name(), ".txt") {
			results = append(results, fullPath)
		}
	}
	return results, nil
}

// --- Log tool handlers ---

func getTaskLogs(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "task_id"))
	logs, err := downloadLogs(ctx, r, id)
	if err != nil {
		return errResult(err)
	}

	lines := strings.Split(logs, "\n")
	var failureHighlights []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
			strings.Contains(line, "✕") || strings.Contains(line, "FAIL") {
			failureHighlights = append(failureHighlights, line)
			if len(failureHighlights) >= 20 {
				break
			}
		}
	}

	exitCode := "0"
	if len(failureHighlights) > 0 {
		exitCode = "1"
	}

	truncatedLogs := logs
	if len(truncatedLogs) > maxLogSize {
		truncatedLogs = truncatedLogs[:maxLogSize]
	}

	return jsonResult(map[string]any{
		"task_id":            id,
		"exit_code":          exitCode,
		"failure_highlights": failureHighlights,
		"logs":               truncatedLogs,
	})
}

func headLogs(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "id"))
	numLines := argInt(args, "lines")
	if numLines <= 0 || numLines > maxLinesPerPage {
		numLines = maxLinesPerPage
	}
	offset := argInt(args, "offset")

	logs, err := downloadLogs(ctx, r, id)
	if err != nil {
		return errResult(err)
	}

	allLines := strings.Split(logs, "\n")
	end := offset + numLines
	if end > len(allLines) {
		end = len(allLines)
	}
	start := offset
	if start > len(allLines) {
		start = len(allLines)
	}

	headLines := allLines[start:end]
	hasMore := end < len(allLines)

	resp := map[string]any{
		"id":              id,
		"offset":          offset,
		"lines_requested": numLines,
		"lines_returned":  len(headLines),
		"total_lines":     len(allLines),
		"has_more":        hasMore,
		"logs":            strings.Join(headLines, "\n"),
	}
	if hasMore {
		resp["next_offset"] = end
	}
	return jsonResult(resp)
}

func tailLogs(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "id"))
	numLines := argInt(args, "lines")
	if numLines <= 0 || numLines > maxLinesPerPage {
		numLines = maxLinesPerPage
	}
	offset := argInt(args, "offset")

	logs, err := downloadLogs(ctx, r, id)
	if err != nil {
		return errResult(err)
	}

	allLines := strings.Split(logs, "\n")
	endIndex := len(allLines) - offset
	if endIndex < 0 {
		endIndex = 0
	}
	startIndex := endIndex - numLines
	if startIndex < 0 {
		startIndex = 0
	}

	tailLines := allLines[startIndex:endIndex]
	hasMore := startIndex > 0

	resp := map[string]any{
		"id":              id,
		"offset":          offset,
		"lines_requested": numLines,
		"lines_returned":  len(tailLines),
		"total_lines":     len(allLines),
		"has_more":        hasMore,
		"start_line":      startIndex + 1,
		"end_line":        endIndex,
		"logs":            strings.Join(tailLines, "\n"),
	}
	if hasMore {
		resp["next_offset"] = offset + len(tailLines)
	}
	return jsonResult(resp)
}

func grepLogs(ctx context.Context, r *rwx, args map[string]any) (*mcp.ToolResult, error) {
	id := extractRunID(argStr(args, "id"))
	pattern := argStr(args, "pattern")
	contextLines := argInt(args, "context")
	if contextLines <= 0 {
		contextLines = 3
	}
	page := argInt(args, "page")
	if page <= 0 {
		page = 1
	}

	logs, err := downloadLogs(ctx, r, id)
	if err != nil {
		return errResult(err)
	}

	allLines := strings.Split(logs, "\n")
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return errResult(fmt.Errorf("invalid pattern: %w", err))
	}

	var matchingIndices []int
	for i, line := range allLines {
		if re.MatchString(line) {
			matchingIndices = append(matchingIndices, i)
		}
	}

	var outputLines []string
	included := make(map[int]bool)
	for idx, matchIdx := range matchingIndices {
		startIdx := matchIdx - contextLines
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := matchIdx + contextLines
		if endIdx >= len(allLines) {
			endIdx = len(allLines) - 1
		}
		for i := startIdx; i <= endIdx; i++ {
			if !included[i] {
				included[i] = true
				prefix := "    "
				if i == matchIdx {
					prefix = ">>> "
				}
				outputLines = append(outputLines, fmt.Sprintf("%s%d: %s", prefix, i+1, allLines[i]))
			}
		}
		if idx < len(matchingIndices)-1 {
			outputLines = append(outputLines, "---")
		}
	}

	startLine := (page - 1) * maxLinesPerPage
	endLine := startLine + maxLinesPerPage
	if endLine > len(outputLines) {
		endLine = len(outputLines)
	}
	if startLine > len(outputLines) {
		startLine = len(outputLines)
	}

	paginated := outputLines[startLine:endLine]
	totalPages := (len(outputLines) + maxLinesPerPage - 1) / maxLinesPerPage
	if totalPages == 0 {
		totalPages = 1
	}
	hasMore := page < totalPages

	resp := map[string]any{
		"id":            id,
		"pattern":       pattern,
		"context":       contextLines,
		"matches_found": len(matchingIndices),
		"total_lines":   len(allLines),
		"page":          page,
		"total_pages":   totalPages,
		"has_more":      hasMore,
		"logs":          strings.Join(paginated, "\n"),
	}
	if hasMore {
		resp["next_page"] = page + 1
	}
	return jsonResult(resp)
}

// --- CLI helper ---

func runRWXCommand(args []string, timeoutMs int) (string, error) {
	cmd := exec.Command("rwx", args...) // #nosec G204 -- fixed binary name, args are controlled
	cmd.Env = os.Environ()
	if timeoutMs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		cmd = exec.CommandContext(ctx, "rwx", args...) // #nosec G204 -- fixed binary name, args are controlled
		cmd.Env = os.Environ()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return string(output), nil
		}
		return "", fmt.Errorf("rwx command failed: %w", err)
	}
	return string(output), nil
}
