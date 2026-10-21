package posthog

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultConsoleLogLimit = 200
	maxConsoleLogLimit     = 1000
)

var (
	recordingIDPattern  = regexp.MustCompile(`^[A-Za-z0-9-]{1,100}$`)
	replayPathPattern   = regexp.MustCompile(`/project/([0-9]+)/replay(?:/([A-Za-z0-9-]+))?`)
	consoleLevels       = map[string]bool{"error": true, "warn": true, "info": true, "log": true, "debug": true}
	hogQLStringReplacer = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
)

// parseRecordingRef accepts a bare session recording ID or a PostHog replay
// URL and returns the session ID plus the project ID embedded in the URL.
func parseRecordingRef(ref string) (session, project string, err error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		u, perr := url.Parse(ref)
		if perr != nil {
			return "", "", fmt.Errorf("session_id: invalid replay URL")
		}
		if m := replayPathPattern.FindStringSubmatch(u.Path); m != nil {
			project = m[1]
			session = m[2]
		}
		if session == "" {
			session = u.Query().Get("sessionRecordingId")
		}
	} else {
		session = ref
	}
	if !recordingIDPattern.MatchString(session) {
		return "", "", fmt.Errorf("session_id must be a session recording ID or a PostHog replay URL")
	}
	return session, project, nil
}

// recordingProject resolves the session and project for a recording tool,
// preferring an explicit project_id, then the replay URL, then the default.
func (p *posthog) recordingProject(args map[string]any) (string, string, error) {
	ref, err := mcp.ArgStr(args, "session_id")
	if err != nil {
		return "", "", err
	}
	session, urlProject, err := parseRecordingRef(ref)
	if err != nil {
		return "", "", err
	}
	if explicit, _ := mcp.ArgStr(args, "project_id"); explicit == "" && urlProject != "" {
		return session, urlProject, nil
	}
	project, err := p.proj(args)
	if err != nil {
		return "", "", err
	}
	return session, project, nil
}

func hogQLString(value string) string {
	return "'" + hogQLStringReplacer.Replace(value) + "'"
}

func hogQLTime(name, value string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("%s must be an ISO 8601 timestamp such as 2026-09-10T14:14:30Z", name)
	}
	// Keep microsecond precision and pin UTC so the bound neither drops
	// fractional seconds nor shifts by the project's time zone.
	return "toDateTime64('" + t.UTC().Format("2006-01-02 15:04:05.000000") + "', 6, 'UTC')", nil
}

func getSessionConsoleLogs(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	session, projID, err := p.recordingProject(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	levelArg := r.Str("level")
	search := r.Str("search")
	after := r.Str("after")
	before := r.Str("before")
	limitArg := r.Str("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	where := []string{
		"log_source = 'session_replay'",
		"log_source_id = " + hogQLString(session),
	}
	if strings.TrimSpace(levelArg) != "" {
		var levels []string
		for _, level := range strings.Split(levelArg, ",") {
			level = strings.ToLower(strings.TrimSpace(level))
			if !consoleLevels[level] {
				return mcp.ErrResult(fmt.Errorf("level must be a comma-separated list of error, warn, info, log, debug"))
			}
			levels = append(levels, hogQLString(level))
		}
		where = append(where, "level IN ("+strings.Join(levels, ", ")+")")
	}
	if search != "" {
		where = append(where, "positionCaseInsensitive(message, "+hogQLString(search)+") > 0")
	}
	if after != "" {
		t, err := hogQLTime("after", after)
		if err != nil {
			return mcp.ErrResult(err)
		}
		where = append(where, "timestamp >= "+t)
	}
	if before != "" {
		t, err := hogQLTime("before", before)
		if err != nil {
			return mcp.ErrResult(err)
		}
		where = append(where, "timestamp <= "+t)
	}
	limit := defaultConsoleLogLimit
	if limitArg != "" {
		n, err := strconv.Atoi(strings.TrimSpace(limitArg))
		if err != nil || n < 1 || n > maxConsoleLogLimit {
			return mcp.ErrResult(fmt.Errorf("limit must be between 1 and %d", maxConsoleLogLimit))
		}
		limit = n
	}

	query := "SELECT timestamp, level, message FROM console_logs_log_entries WHERE " +
		strings.Join(where, " AND ") +
		" ORDER BY timestamp ASC LIMIT " + strconv.Itoa(limit)
	data, err := p.post(ctx, fmt.Sprintf("/api/projects/%s/query/", projID), map[string]any{
		"query": map[string]any{"kind": "HogQLQuery", "query": query},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSessionRecordings(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"person_uuid": r.Str("person_id"),
		"date_from":   r.Str("date_from"),
		"date_to":     r.Str("date_to"),
		"limit":       r.Str("limit"),
		"offset":      r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/session_recordings/%s", projID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSessionRecording(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	session, projID, err := p.recordingProject(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/session_recordings/%s/", projID, session)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
