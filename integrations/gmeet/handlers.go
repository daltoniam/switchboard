package gmeet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// normalizeSpaceName accepts either a full space resource name
// ('spaces/{id}'), a bare space id, or a meeting code (the dashed
// 'abc-defg-hij' shape users see in invites) and returns the canonical
// 'spaces/{id}' form. The Meet API's GET /v2/spaces/{name} endpoint
// accepts all three shapes natively, so we just need to make sure the
// /spaces/ prefix is present.
func normalizeSpaceName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "spaces/") {
		return s
	}
	return "spaces/" + s
}

// normalizeConferenceRecord accepts either a full conference record
// resource name ('conferenceRecords/{id}') or a bare id and returns
// the canonical 'conferenceRecords/{id}' form.
func normalizeConferenceRecord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "conferenceRecords/") {
		return s
	}
	return "conferenceRecords/" + s
}

// parseObject accepts a JSON object argument as either a JSON-encoded
// string (the most common shape from LLM tool calls) or a pre-decoded
// map[string]any (when scripts construct the object directly). Returns
// (nil, nil) for absent/empty input; an error on invalid JSON.
func parseObject(v any, fieldName string) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return nil, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(x), &m); err != nil {
			return nil, fmt.Errorf("%s: invalid JSON: %w", fieldName, err)
		}
		return m, nil
	case map[string]any:
		if len(x) == 0 {
			return nil, nil
		}
		return x, nil
	default:
		return nil, fmt.Errorf("%s: expected JSON object or string, got %T", fieldName, v)
	}
}

// escapeResourceID escapes the trailing id segment of a resource name
// while leaving the canonical collection prefix unescaped. For inputs
// like 'spaces/abc/123' it preserves the slashes within the id (the
// Meet API uses opaque alphanumeric ids, but transcripts and entries
// are nested under multi-segment names like
// 'conferenceRecords/{cid}/transcripts/{tid}').
func escapePath(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// ── Spaces ──────────────────────────────────────────────────────────

func createSpace(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	cfg, err := parseObject(args["config"], "config")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if cfg != nil {
		body["config"] = cfg
	}
	data, err := g.post(ctx, "/spaces", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSpace(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_get_space: name is required"))
	}
	resolved := normalizeSpaceName(name)
	path := "/" + escapePath(resolved)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSpace(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	updateMask := r.Str("update_mask")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_update_space: name is required"))
	}
	if updateMask == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_update_space: update_mask is required"))
	}
	if _, ok := args["space"]; !ok {
		return mcp.ErrResult(fmt.Errorf("gmeet_update_space: space is required"))
	}
	body, err := parseObject(args["space"], "space")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(fmt.Errorf("gmeet_update_space: space is required"))
	}

	resolved := normalizeSpaceName(name)
	path := "/" + escapePath(resolved) + "?updateMask=" + url.QueryEscape(updateMask)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func endActiveConference(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_end_active_conference: name is required"))
	}
	resolved := normalizeSpaceName(name)
	path := "/" + escapePath(resolved) + ":endActiveConference"
	data, err := g.post(ctx, path, map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Conference records ──────────────────────────────────────────────

func listConferenceRecords(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filter := r.Str("filter")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	params := url.Values{}
	if filter != "" {
		params.Set("filter", filter)
	}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/conferenceRecords"
	if q := params.Encode(); q != "" {
		path += "?" + q
	}
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getConferenceRecord(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_get_conference_record: name is required"))
	}
	resolved := normalizeConferenceRecord(name)
	path := "/" + escapePath(resolved)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listParticipants(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cr := r.Str("conference_record")
	filter := r.Str("filter")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if cr == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_list_participants: conference_record is required"))
	}
	resolved := normalizeConferenceRecord(cr)

	params := url.Values{}
	if filter != "" {
		params.Set("filter", filter)
	}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/" + escapePath(resolved) + "/participants"
	if q := params.Encode(); q != "" {
		path += "?" + q
	}
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRecordings(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cr := r.Str("conference_record")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if cr == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_list_recordings: conference_record is required"))
	}
	resolved := normalizeConferenceRecord(cr)

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/" + escapePath(resolved) + "/recordings"
	if q := params.Encode(); q != "" {
		path += "?" + q
	}
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTranscripts(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cr := r.Str("conference_record")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if cr == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_list_transcripts: conference_record is required"))
	}
	resolved := normalizeConferenceRecord(cr)

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/" + escapePath(resolved) + "/transcripts"
	if q := params.Encode(); q != "" {
		path += "?" + q
	}
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTranscriptEntries(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	transcript := r.Str("transcript")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if transcript == "" {
		return mcp.ErrResult(fmt.Errorf("gmeet_list_transcript_entries: transcript is required"))
	}
	// Transcript is always 'conferenceRecords/{cid}/transcripts/{tid}'; we
	// don't auto-prefix because it has a two-segment shape.
	transcript = strings.TrimSpace(transcript)

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	path := "/" + escapePath(transcript) + "/entries"
	if q := params.Encode(); q != "" {
		path += "?" + q
	}
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
