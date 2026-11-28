package openproject

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
)

type resourceLink struct {
	Href  string  `json:"href"`
	Title *string `json:"title"`
}

type upstreamWorkPackage struct {
	ID          int64   `json:"id"`
	DisplayID   *string `json:"displayId"`
	Subject     *string `json:"subject"`
	UpdatedAt   *string `json:"updatedAt"`
	CreatedAt   *string `json:"createdAt"`
	StartDate   *string `json:"startDate"`
	DueDate     *string `json:"dueDate"`
	Description *struct {
		Raw *string `json:"raw"`
	} `json:"description"`
	Links struct {
		Project resourceLink `json:"project"`
		Status  resourceLink `json:"status"`
		Type    resourceLink `json:"type"`
	} `json:"_links"`
}

type workPackageSummary struct {
	ID        int64  `json:"id"`
	DisplayID string `json:"displayId"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	ProjectID int64  `json:"project_id"`
	UpdatedAt string `json:"updatedAt"`
}

type workPackageDetail struct {
	workPackageSummary
	CreatedAt            string  `json:"createdAt"`
	StartDate            *string `json:"startDate"`
	DueDate              *string `json:"dueDate"`
	Description          string  `json:"description"`
	DescriptionTruncated bool    `json:"description_truncated"`
	Truncated            bool    `json:"truncated"`
}

type workPackagePage struct {
	WorkPackages []workPackageSummary `json:"work_packages"`
	Total        int64                `json:"total"`
	Count        int64                `json:"count"`
	Page         int64                `json:"page"`
	PageSize     int64                `json:"page_size"`
	Returned     int                  `json:"returned"`
	Omitted      int                  `json:"omitted"`
	Truncated    bool                 `json:"truncated"`
	NextPage     *int64               `json:"next_page"`
}

func boundedText(text string, limit int) (string, bool) {
	if utf8.RuneCountInString(text) <= limit {
		return text, false
	}
	return string([]rune(text)[:limit]), true
}

func (o *openproject) belongsToProject(href string) bool {
	base, err := url.Parse(o.baseURL)
	if err != nil {
		return false
	}
	expected := base.Path + "/api/v3/projects/" + strconv.FormatInt(o.workspaceID, 10)
	return href == expected
}

func (o *openproject) summarize(item upstreamWorkPackage) (workPackageSummary, bool, error) {
	if item.ID < 1 || item.DisplayID == nil || item.Subject == nil || item.UpdatedAt == nil || item.Links.Status.Title == nil || item.Links.Type.Title == nil {
		return workPackageSummary{}, false, fmt.Errorf("openproject: malformed work package summary; required identity or metadata is missing")
	}
	result := workPackageSummary{ID: item.ID, ProjectID: o.workspaceID}
	truncated := false
	for _, field := range []struct {
		input  string
		output *string
	}{
		{*item.DisplayID, &result.DisplayID}, {*item.Subject, &result.Subject},
		{*item.UpdatedAt, &result.UpdatedAt}, {*item.Links.Status.Title, &result.Status}, {*item.Links.Type.Title, &result.Type},
	} {
		value, cut := boundedText(field.input, 512)
		*field.output = value
		truncated = truncated || cut
	}
	return result, truncated, nil
}

func positiveInteger(args map[string]any, key string, fallback int64) (int64, error) {
	value, present := args[key]
	if !present {
		if fallback > 0 {
			return fallback, nil
		}
		return 0, fmt.Errorf("openproject: %s is required as a positive integer", key)
	}
	switch typed := value.(type) {
	case float64:
		if typed > 9007199254740991 {
			return 0, fmt.Errorf("openproject: %s exceeds exact JSON numeric precision; pass a decimal string instead", key)
		}
	case int, int64, json.Number, string:
	default:
		return 0, fmt.Errorf("openproject: %s must be a positive integer", key)
	}
	text, err := mcp.ArgStr(args, key)
	if err != nil || text == "" || strings.Trim(text, "0123456789") != "" {
		return 0, fmt.Errorf("openproject: %s must be a positive decimal integer", key)
	}
	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil || number < 1 {
		return 0, fmt.Errorf("openproject: %s must be a positive integer within the supported range", key)
	}
	return number, nil
}

func listWorkPackages(ctx context.Context, o *openproject, args map[string]any) (*mcp.ToolResult, error) {
	page, err := positiveInteger(args, "page", 1)
	if err != nil {
		return mcp.ErrResult(err)
	}
	limit, err := positiveInteger(args, "limit", 20)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if limit > 50 || page > (1<<63-1)/50 {
		return mcp.ErrResult(fmt.Errorf("openproject: limit must be 1-50 and page must not overflow pagination"))
	}
	if value, present := args["subject_or_id"]; present {
		if _, ok := value.(string); !ok {
			return mcp.ErrResult(fmt.Errorf("openproject: subject_or_id must be text"))
		}
	}
	queryText, err := mcp.ArgStr(args, "subject_or_id")
	if err != nil || !utf8.ValidString(queryText) || utf8.RuneCountInString(queryText) > 256 {
		return mcp.ErrResult(fmt.Errorf("openproject: subject_or_id must be text of at most 256 characters"))
	}
	filters := "[]"
	if queryText != "" {
		encoded, err := json.Marshal([]map[string]any{{"subjectOrId": map[string]any{"operator": "**", "values": []string{queryText}}}})
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("openproject: unable to encode search filter"))
		}
		filters = string(encoded)
	}
	query := url.Values{
		"offset": {strconv.FormatInt(page, 10)}, "pageSize": {strconv.FormatInt(limit, 10)},
		"filters": {filters}, "sortBy": {`[["id","asc"]]`},
	}
	var response struct {
		Total    *int64 `json:"total"`
		Count    *int64 `json:"count"`
		Offset   *int64 `json:"offset"`
		PageSize *int64 `json:"pageSize"`
		Embedded struct {
			Elements []upstreamWorkPackage `json:"elements"`
		} `json:"_embedded"`
	}
	path := "/api/v3/workspaces/" + strconv.FormatInt(o.workspaceID, 10) + "/work_packages"
	if err := o.get(ctx, path, query, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Total == nil || response.Count == nil || response.Offset == nil || response.PageSize == nil || response.Embedded.Elements == nil {
		return mcp.ErrResult(fmt.Errorf("openproject: collection is missing required paging metadata or elements"))
	}
	total, count, size := *response.Total, *response.Count, *response.PageSize
	if total < 0 || count < 0 || count != int64(len(response.Embedded.Elements)) || size < 1 || size > limit || count > size || count > total || *response.Offset != page {
		return mcp.ErrResult(fmt.Errorf("openproject: invalid upstream collection paging metadata"))
	}
	result := workPackagePage{WorkPackages: make([]workPackageSummary, 0, len(response.Embedded.Elements)), Total: total, Count: count, Page: page, PageSize: size}
	for _, item := range response.Embedded.Elements {
		if !o.belongsToProject(item.Links.Project.Href) {
			result.Omitted++
			continue
		}
		summary, truncated, err := o.summarize(item)
		if err != nil {
			return mcp.ErrResult(err)
		}
		result.WorkPackages = append(result.WorkPackages, summary)
		result.Truncated = result.Truncated || truncated
	}
	result.Returned = len(result.WorkPackages)
	if page*size < total {
		next := page + 1
		result.NextPage = &next
	}
	return boundedResult(result)
}

func getWorkPackage(ctx context.Context, o *openproject, args map[string]any) (*mcp.ToolResult, error) {
	id, err := positiveInteger(args, "id", 0)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var response upstreamWorkPackage
	if err := o.get(ctx, "/api/v3/work_packages/"+strconv.FormatInt(id, 10), nil, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.ID != id || !o.belongsToProject(response.Links.Project.Href) {
		return mcp.ErrResult(fmt.Errorf("openproject: work package not found or not visible in the configured project"))
	}
	summary, truncated, err := o.summarize(response)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if response.CreatedAt == nil || response.Description == nil || response.Description.Raw == nil {
		return mcp.ErrResult(fmt.Errorf("openproject: work package is missing required detail metadata"))
	}
	created, createdCut := boundedText(*response.CreatedAt, 512)
	result := workPackageDetail{workPackageSummary: summary, CreatedAt: created, Truncated: truncated || createdCut}
	for _, field := range []struct {
		input  *string
		output **string
	}{{response.StartDate, &result.StartDate}, {response.DueDate, &result.DueDate}} {
		if field.input == nil {
			continue
		}
		value, cut := boundedText(*field.input, 512)
		*field.output = &value
		result.Truncated = result.Truncated || cut
	}
	result.Description, result.DescriptionTruncated = boundedText(*response.Description.Raw, 8000)
	result.Truncated = result.Truncated || result.DescriptionTruncated
	return boundedResult(result)
}

func boundedResult(value any) (*mcp.ToolResult, error) {
	result, err := mcp.JSONResult(value)
	if err != nil || result.IsError {
		return result, err
	}
	if len(result.Data) >= 40*1024 {
		return mcp.ErrResult(fmt.Errorf("openproject: projected response exceeds 40 KiB; request a smaller list limit or shorter source content"))
	}
	return result, nil
}
