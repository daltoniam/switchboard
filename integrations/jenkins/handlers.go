package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
)

type lastBuild struct {
	Number    int64   `json:"number"`
	Result    *string `json:"result"`
	Timestamp *int64  `json:"timestamp"`
}

type jobSummary struct {
	Name      string     `json:"name"`
	Class     string     `json:"_class"`
	Color     string     `json:"color"`
	LastBuild *lastBuild `json:"lastBuild"`
}

type buildSummary struct {
	Number    int64   `json:"number"`
	Result    *string `json:"result"`
	Timestamp *int64  `json:"timestamp"`
	Duration  *int64  `json:"duration"`
	Building  *bool   `json:"building"`
}

type pagination struct {
	ReturnedCount int  `json:"returned_count"`
	Offset        int  `json:"offset"`
	Limit         int  `json:"limit"`
	PossiblyMore  bool `json:"possibly_more"`
}

func boundedString(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= 512 && !strings.ContainsFunc(value, unicode.IsControl)
}

func validSegment(value string) bool {
	if value == "" || value == "." || value == ".." || !boundedString(value) || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	parsed, err := url.Parse(strings.ReplaceAll(value, "%", "%25"))
	return err == nil && !parsed.IsAbs() && parsed.Host == "" && !strings.Contains(value, "://")
}

func itemPath(args map[string]any, key string, required bool) (string, []string, error) {
	value, present := args[key]
	if !present && !required {
		return "", nil, nil
	}
	if text, ok := value.(string); ok {
		var decoded []string
		if json.Unmarshal([]byte(text), &decoded) != nil || decoded == nil {
			return "", nil, fmt.Errorf("jenkins: %s must be an ordered JSON array of exact item-name segments", key)
		}
		value = decoded
	}
	switch value.(type) {
	case []string, []any:
	default:
		return "", nil, fmt.Errorf("jenkins: %s must be an ordered JSON array of exact item-name segments", key)
	}
	segments, err := mcp.ArgStrSlice(map[string]any{key: value}, key)
	if err != nil || (required && len(segments) == 0) {
		return "", nil, fmt.Errorf("jenkins: %s requires exact nonempty item-name segments", key)
	}
	var path strings.Builder
	for _, segment := range segments {
		if !validSegment(segment) {
			return "", nil, fmt.Errorf("jenkins: %s contains an invalid item-name segment; use exact names of at most 512 characters, not URLs or dot paths", key)
		}
		path.WriteString("/job/")
		path.WriteString(url.PathEscape(segment))
	}
	return path.String(), segments, nil
}

func integerParameter(args map[string]any, key string, fallback int) (int, error) {
	value, present := args[key]
	if !present {
		return fallback, nil
	}
	switch value.(type) {
	case int, int64, float64, string, json.Number:
	default:
		return 0, fmt.Errorf("jenkins: %s must be an integer", key)
	}
	text, err := mcp.ArgStr(args, key)
	if err != nil {
		return 0, fmt.Errorf("jenkins: %s must be an integer", key)
	}
	n, err := strconv.ParseInt(text, 10, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("jenkins: %s must be an integer within the supported range", key)
	}
	return int(n), nil
}

func selectionWindow(args map[string]any) (pagination, error) {
	offset, err := integerParameter(args, "offset", 0)
	if err != nil {
		return pagination{}, err
	}
	limit, err := integerParameter(args, "limit", 20)
	if err != nil {
		return pagination{}, err
	}
	if offset < 0 || limit < 1 || limit > 50 || offset > int(^uint(0)>>1)-limit {
		return pagination{}, fmt.Errorf("jenkins: offset must be nonnegative without range overflow and limit must be 1-50")
	}
	return pagination{Offset: offset, Limit: limit}, nil
}

func (p *pagination) setCount(count int) error {
	if count > p.Limit || count > 50 {
		return fmt.Errorf("jenkins: response exceeds requested record limit; refusing incomplete or silently truncated output")
	}
	p.ReturnedCount, p.PossiblyMore = count, count == p.Limit
	return nil
}

func validateLastBuild(build *lastBuild) error {
	if build == nil {
		return nil
	}
	if build.Number < 1 || build.Timestamp == nil || *build.Timestamp < 0 || (build.Result != nil && !boundedString(*build.Result)) {
		return fmt.Errorf("jenkins: invalid lastBuild response; strings must be at most 512 characters")
	}
	return nil
}

func validateJob(job jobSummary) error {
	if !validSegment(job.Name) || !boundedString(job.Class) || !boundedString(job.Color) {
		return fmt.Errorf("jenkins: invalid job summary response; strings must be at most 512 characters")
	}
	return validateLastBuild(job.LastBuild)
}

func validateBuild(build buildSummary) error {
	if build.Number < 1 || build.Timestamp == nil || *build.Timestamp < 0 || build.Duration == nil || *build.Duration < 0 || build.Building == nil || (build.Result != nil && !boundedString(*build.Result)) {
		return fmt.Errorf("jenkins: invalid build summary response; strings must be at most 512 characters")
	}
	return nil
}

func listJobs(ctx context.Context, j *jenkins, args map[string]any) (*mcp.ToolResult, error) {
	path, _, err := itemPath(args, "folder_path", false)
	if err != nil {
		return mcp.ErrResult(err)
	}
	window, err := selectionWindow(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	tree := fmt.Sprintf("jobs[name,_class,color,lastBuild[number,result,timestamp]]{%d,%d}", window.Offset, window.Offset+window.Limit)
	var response struct {
		Jobs []jobSummary `json:"jobs"`
	}
	if err := j.get(ctx, path, tree, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Jobs == nil {
		return mcp.ErrResult(fmt.Errorf("jenkins: response is missing jobs array"))
	}
	if err := window.setCount(len(response.Jobs)); err != nil {
		return mcp.ErrResult(err)
	}
	for _, job := range response.Jobs {
		if err := validateJob(job); err != nil {
			return mcp.ErrResult(err)
		}
	}
	return mcp.JSONResult(struct {
		Jobs []jobSummary `json:"jobs"`
		pagination
	}{response.Jobs, window})
}

func getJob(ctx context.Context, j *jenkins, args map[string]any) (*mcp.ToolResult, error) {
	path, segments, err := itemPath(args, "job_path", true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	window, err := selectionWindow(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	tree := fmt.Sprintf("name,_class,color,buildable,inQueue,lastBuild[number,result,timestamp],builds[number,result,timestamp,duration,building]{%d,%d}", window.Offset, window.Offset+window.Limit)
	var response struct {
		jobSummary
		Buildable *bool          `json:"buildable"`
		InQueue   *bool          `json:"inQueue"`
		Builds    []buildSummary `json:"builds"`
	}
	if err := j.get(ctx, path, tree, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Buildable == nil || response.InQueue == nil {
		return mcp.ErrResult(fmt.Errorf("jenkins: response is missing buildable or inQueue state"))
	}
	if response.Builds == nil {
		return mcp.ErrResult(fmt.Errorf("jenkins: response is missing builds array"))
	}
	if response.Name != segments[len(segments)-1] {
		return mcp.ErrResult(fmt.Errorf("jenkins: response job identity does not match request"))
	}
	if err := validateJob(response.jobSummary); err != nil {
		return mcp.ErrResult(err)
	}
	if err := window.setCount(len(response.Builds)); err != nil {
		return mcp.ErrResult(err)
	}
	for _, build := range response.Builds {
		if err := validateBuild(build); err != nil {
			return mcp.ErrResult(err)
		}
	}
	return mcp.JSONResult(struct {
		jobSummary
		Buildable *bool          `json:"buildable"`
		InQueue   *bool          `json:"inQueue"`
		Builds    []buildSummary `json:"builds"`
		pagination
	}{response.jobSummary, response.Buildable, response.InQueue, response.Builds, window})
}

func getBuild(ctx context.Context, j *jenkins, args map[string]any) (*mcp.ToolResult, error) {
	path, _, err := itemPath(args, "job_path", true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	number, err := integerParameter(args, "build_number", 0)
	if err != nil || number < 1 {
		return mcp.ErrResult(fmt.Errorf("jenkins: build_number must be a positive integer within the supported range"))
	}
	var response struct {
		buildSummary
		EstimatedDuration *int64 `json:"estimatedDuration"`
	}
	if err := j.get(ctx, path+"/"+strconv.Itoa(number), "number,result,building,timestamp,duration,estimatedDuration", &response); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validateBuild(response.buildSummary); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Number != int64(number) || response.EstimatedDuration == nil || *response.EstimatedDuration < -1 {
		return mcp.ErrResult(fmt.Errorf("jenkins: invalid build identity or estimated duration in response"))
	}
	phase := "completed"
	if *response.Building {
		phase = "building"
	}
	return mcp.JSONResult(struct {
		buildSummary
		EstimatedDuration *int64 `json:"estimatedDuration"`
		Phase             string `json:"phase"`
	}{response.buildSummary, response.EstimatedDuration, phase})
}
