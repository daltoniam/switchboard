package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
)

type forgejo struct {
	mu         sync.RWMutex
	baseURL    string
	token      string
	httpClient *http.Client
}

func New() mcp.Integration { return &forgejo{} }

func (f *forgejo) Name() string { return "forgejo" }

func (f *forgejo) Configure(_ context.Context, creds mcp.Credentials) error {
	baseURL, err := normalizeURL(creds["base_url"])
	if err != nil {
		return err
	}
	token := strings.TrimSpace(creds["token"])
	if token == "" || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("forgejo: token is required and must not contain whitespace or control characters")
	}
	hc := &http.Client{
		Transport:     boundedTransport{base: http.DefaultTransport},
		Timeout:       30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	f.mu.Lock()
	f.baseURL, f.token, f.httpClient = baseURL, token, hc
	f.mu.Unlock()
	return nil
}

type boundedTransport struct{ base http.RoundTripper }

func (t boundedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	resp.Body = &boundedBody{ReadCloser: resp.Body, remaining: 8 * 1024 * 1024}
	return resp, nil
}

type boundedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *boundedBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.remaining == 0 {
		var probe [1]byte
		n, err := b.ReadCloser.Read(probe[:])
		if n > 0 {
			return 0, fmt.Errorf("forgejo: upstream response exceeds 8 MiB safety limit")
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.ContainsAny(raw, "?#") || strings.ContainsFunc(raw, unicode.IsSpace) {
		return "", fmt.Errorf("forgejo: base_url must be an HTTP(S) installation URL without credentials, query, or fragment")
	}
	if strings.HasSuffix(u.Host, ":") {
		return "", fmt.Errorf("forgejo: invalid base_url port")
	}
	if port := u.Port(); port != "" {
		n, err := mcp.ArgInt(map[string]any{"port": port}, "port")
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("forgejo: invalid base_url port")
		}
	}
	path := strings.TrimRight(u.Path, "/")
	if path != "" {
		if err := validateInstallationPath(strings.TrimPrefix(path, "/")); err != nil {
			return "", fmt.Errorf("forgejo: invalid base_url installation path")
		}
	}
	u.Path = strings.TrimSuffix(path, "/api/v1")
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func (f *forgejo) client(ctx context.Context) (*sdk.Client, error) {
	if f == nil {
		return nil, mcp.ErrNotConfigured
	}
	f.mu.RLock()
	baseURL, token, hc := f.baseURL, f.token, f.httpClient
	f.mu.RUnlock()
	if hc == nil || baseURL == "" || token == "" {
		return nil, mcp.ErrNotConfigured
	}
	if ctx == nil {
		return nil, fmt.Errorf("forgejo: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sdk.NewClient(baseURL, sdk.SetForgejoVersion(""), sdk.SetToken(token), sdk.SetHTTPClient(hc), sdk.SetContext(ctx))
}

func (f *forgejo) Healthy(ctx context.Context) bool {
	c, err := f.client(ctx)
	if err != nil {
		return false
	}
	user, resp, err := c.GetMyUserInfo()
	return err == nil && resp != nil && resp.Response != nil && resp.StatusCode == http.StatusOK && user != nil && user.ID > 0
}

func (f *forgejo) Tools() []mcp.ToolDefinition { return tools }

func (f *forgejo) MaxResponseBytesForTool(name mcp.ToolName) (int, bool) {
	switch name {
	case "forgejo_get_pull_diff", "forgejo_get_file_contents":
		return 1024 * 1024, true
	default:
		return 0, false
	}
}

func (f *forgejo) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("forgejo: unknown tool: %s", name))
	}
	if err := validateArgs(name, args); err != nil {
		return mcp.ErrResult(err)
	}
	c, err := f.client(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result, err := handler(c, args)
	if !readOnly(name) {
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("forgejo: mutation outcome uncertain; automatic retry disabled, verify the remote state before retrying: %v", err))
		}
		if result != nil && result.IsError {
			return mcp.ErrResult(fmt.Errorf("forgejo: mutation outcome uncertain; automatic retry disabled, verify the remote state before retrying: %s", result.Data))
		}
	}
	return result, err
}

func readOnly(name mcp.ToolName) bool {
	switch name {
	case "forgejo_list_user_repos", "forgejo_search_repos", "forgejo_get_repo",
		"forgejo_list_org_repos", "forgejo_list_user_orgs", "forgejo_get_current_user",
		"forgejo_list_branches", "forgejo_get_branch", "forgejo_list_commits",
		"forgejo_get_commit", "forgejo_get_file_contents", "forgejo_list_directory",
		"forgejo_list_releases", "forgejo_get_release", "forgejo_list_labels",
		"forgejo_list_issues", "forgejo_get_issue", "forgejo_list_issue_comments",
		"forgejo_list_pulls", "forgejo_get_pull", "forgejo_get_pull_diff",
		"forgejo_list_pull_files", "forgejo_list_pull_reviews":
		return true
	default:
		return false
	}
}

func finish(value any, resp *sdk.Response, err error) (*mcp.ToolResult, error) {
	status := 0
	if resp != nil && resp.Response != nil {
		status = resp.StatusCode
	}
	if status >= 300 {
		if err == nil {
			err = fmt.Errorf("forgejo: HTTP %d %s", status, http.StatusText(status))
		} else {
			err = fmt.Errorf("forgejo: HTTP %d %s: %w", status, http.StatusText(status), err)
		}
		if status == 429 || status >= 500 {
			return mcp.ErrResult(&mcp.RetryableError{StatusCode: status, Err: err, RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))})
		}
	}
	if status == http.StatusNoContent {
		return mcp.JSONResult(nil)
	}
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(value)
}

func validateArgs(name mcp.ToolName, args map[string]any) error {
	var definition mcp.ToolDefinition
	for _, tool := range tools {
		if tool.Name == name {
			definition = tool
			break
		}
	}
	for _, key := range definition.Required {
		v, ok := args[key]
		if !ok || v == nil {
			return fmt.Errorf("forgejo: %s is required", key)
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			return fmt.Errorf("forgejo: %s is required", key)
		}
	}
	for key, value := range args {
		if _, ok := definition.Parameters[key]; !ok {
			return fmt.Errorf("forgejo: unknown parameter %q", key)
		}
		if value == nil {
			return fmt.Errorf("forgejo: %s must not be null", key)
		}
		if err := validateValue(name, key, value); err != nil {
			return fmt.Errorf("forgejo: %s: %w", key, err)
		}
	}
	return nil
}

func validateValue(name mcp.ToolName, key string, value any) error {
	switch key {
	case "page", "per_page", "number", "release_id", "milestone":
		if err := validateInteger(value); err != nil {
			return err
		}
		n, err := mcp.ArgInt64(map[string]any{key: value}, key)
		if err != nil {
			return err
		}
		if n < 1 || (key == "per_page" && n > 50) {
			return fmt.Errorf("must be positive (per_page maximum 50)")
		}
		return nil
	case "labels":
		if name == "forgejo_list_issues" {
			return validateStrings(value, false)
		}
		_, err := labelIDs(map[string]any{key: value})
		return err
	case "assignees":
		return validateStrings(value, true)
	case "closed", "allow_maintainer_edit", "delete_branch_after_merge", "draft", "prerelease":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("must be a string")
	}
	switch key {
	case "owner", "repo", "org", "username":
		return validatePath(s, false)
	case "path", "branch", "ref", "sha", "head", "base":
		if s == "" && (key == "path" && (name == "forgejo_list_directory" || name == "forgejo_list_commits") || key == "ref") {
			return nil
		}
		return validatePath(s, true)
	case "title", "query", "event", "state", "merge_method", "sort", "order":
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("must not be empty")
		}
	case "body":
		if name == "forgejo_create_issue_comment" && strings.TrimSpace(s) == "" {
			return fmt.Errorf("must not be empty")
		}
	}
	var allowed []string
	switch key {
	case "state":
		allowed = []string{"open", "closed"}
		if name == "forgejo_list_issues" || name == "forgejo_list_pulls" {
			allowed = append(allowed, "all")
		}
	case "event":
		allowed = []string{"APPROVED", "COMMENT", "REQUEST_CHANGES"}
	case "merge_method":
		allowed = []string{"merge", "rebase", "rebase-merge", "squash", "fast-forward-only"}
	case "order":
		allowed = []string{"asc", "desc"}
	case "sort":
		allowed = []string{"alpha", "created", "updated", "size", "id"}
		if name == "forgejo_list_pulls" {
			allowed = []string{"oldest", "recentupdate", "leastupdate", "mostcomment", "leastcomment", "priority"}
		}
	}
	if allowed != nil && !slices.Contains(allowed, s) {
		return fmt.Errorf("must be one of %s", strings.Join(allowed, ", "))
	}
	return nil
}

func validateInteger(value any) error {
	switch v := value.(type) {
	case int, int64, json.Number, string:
		_, err := mcp.ArgInt64(map[string]any{"value": value}, "value")
		return err
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v) && v >= math.MinInt64 && v < math.MaxInt64 {
			return nil
		}
	}
	return fmt.Errorf("must be an integer in int64 range")
}

func validateInstallationPath(value string) error {
	if strings.ContainsAny(value, "%?#") {
		return fmt.Errorf("installation path must not contain escapes, query, or fragment")
	}
	return validatePath(value, true)
}

func validatePath(value string, nested bool) error {
	if strings.TrimSpace(value) == "" || strings.Contains(value, "\\") || strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("must be a nonempty path without backslashes or control characters")
	}
	if !nested && strings.ContainsAny(value, "/%?#") {
		return fmt.Errorf("must be a single name without escapes, query, or fragment")
	}
	for part := range strings.SplitSeq(value, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("empty or dot path components are not allowed")
		}
	}
	return nil
}

func validateStrings(value any, paths bool) error {
	switch v := value.(type) {
	case []string:
		if v == nil {
			return fmt.Errorf("must not be null")
		}
	case []any:
		if v == nil {
			return fmt.Errorf("must not be null")
		}
	default:
		return fmt.Errorf("must be an array of strings")
	}
	values, err := mcp.ArgStrSlice(map[string]any{"value": value}, "value")
	if err != nil {
		return err
	}
	for _, s := range values {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("array entries must not be empty")
		}
		if paths {
			if err := validatePath(s, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func labelIDs(args map[string]any) ([]int64, error) {
	value, ok := args["labels"]
	if !ok {
		return nil, nil
	}
	var values []any
	switch v := value.(type) {
	case []any:
		if v == nil {
			return nil, fmt.Errorf("labels must not be null")
		}
		values = v
	case []int64:
		if v == nil {
			return nil, fmt.Errorf("labels must not be null")
		}
		values = make([]any, len(v))
		for i, id := range v {
			values[i] = id
		}
	default:
		return nil, fmt.Errorf("labels must be an array of positive integer IDs")
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		if err := validateInteger(value); err != nil {
			return nil, err
		}
		id, err := mcp.ArgInt64(map[string]any{"label": value}, "label")
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			return nil, fmt.Errorf("label IDs must be positive")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func pagination(r *mcp.Args) sdk.ListOptions {
	return sdk.ListOptions{Page: r.OptInt("page", 1), PageSize: r.OptInt("per_page", 30)}
}

type handlerFunc func(*sdk.Client, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"forgejo_list_user_repos":      listUserRepos,
	"forgejo_search_repos":         searchRepos,
	"forgejo_get_repo":             getRepo,
	"forgejo_list_org_repos":       listOrgRepos,
	"forgejo_list_user_orgs":       listUserOrgs,
	"forgejo_get_current_user":     getCurrentUser,
	"forgejo_list_branches":        listBranches,
	"forgejo_get_branch":           getBranch,
	"forgejo_list_commits":         listCommits,
	"forgejo_get_commit":           getCommit,
	"forgejo_get_file_contents":    getFileContents,
	"forgejo_list_directory":       listDirectory,
	"forgejo_list_releases":        listReleases,
	"forgejo_get_release":          getRelease,
	"forgejo_list_labels":          listLabels,
	"forgejo_list_issues":          listIssues,
	"forgejo_get_issue":            getIssue,
	"forgejo_create_issue":         createIssue,
	"forgejo_update_issue":         updateIssue,
	"forgejo_list_issue_comments":  listIssueComments,
	"forgejo_create_issue_comment": createIssueComment,
	"forgejo_list_pulls":           listPulls,
	"forgejo_get_pull":             getPull,
	"forgejo_create_pull":          createPull,
	"forgejo_update_pull":          updatePull,
	"forgejo_get_pull_diff":        getPullDiff,
	"forgejo_list_pull_files":      listPullFiles,
	"forgejo_list_pull_reviews":    listPullReviews,
	"forgejo_create_pull_review":   createPullReview,
	"forgejo_merge_pull":           mergePull,
}
