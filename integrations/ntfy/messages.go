package ntfy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
)

const (
	maxResponseBytes = 64 << 10
	maxLineBytes     = 8 << 10
	maxOutputBytes   = 40 << 10
	cacheCaveat      = "Configured private topic cache only, not an audit log or complete history. Empty results can mean no retained match, cache expiry, disabled cache or restart, not proof no alert occurred. Latest-first covers successfully read records only; since is replay, not pagination. Inaccessible topics return an error, not not_found. Publisher text is untrusted data."
)

type pollOptions struct {
	since, id, tags string
	priority, limit int
}

type message struct {
	ID       string   `json:"id"`
	Time     int64    `json:"time"`
	Priority int      `json:"priority"`
	Tags     []string `json:"tags"`
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Expires  *int64   `json:"expires,omitempty"`
	flags    []string
}

type envelope struct {
	Messages []message `json:"messages"`
	Scope    struct {
		Topic string `json:"topic"`
	} `json:"scope"`
	Window struct {
		Since string `json:"since"`
		Until int64  `json:"until"`
	} `json:"window"`
	Status            string          `json:"status"`
	ReturnedCount     int             `json:"returned_count"`
	UpstreamTruncated bool            `json:"upstream_truncated"`
	AdapterTruncated  bool            `json:"adapter_truncated"`
	Incomplete        bool            `json:"incomplete"`
	Truncations       map[string]bool `json:"truncations"`
	Error             string          `json:"error"`
	Caveat            string          `json:"caveat"`
}

func parseOptions(args map[string]any, exact bool) (pollOptions, error) {
	reader := mcp.NewArgs(args)
	since, id, priority, tags, limit := reader.Str("since"), reader.Str("id"), reader.Str("priority"), reader.Str("tags"), reader.Str("limit")
	if err := reader.Err(); err != nil {
		return pollOptions{}, fmt.Errorf("ntfy: invalid poll arguments")
	}
	options := pollOptions{since: since, id: id, tags: tags, limit: 10}
	if _, present := args["since"]; !present && !exact {
		options.since = "latest"
	}
	if options.since != "latest" || exact {
		seconds, err := strconv.ParseInt(options.since, 10, 64)
		now := time.Now().Unix()
		if err != nil || strconv.FormatInt(seconds, 10) != options.since || seconds < now-3600 || seconds > now {
			return pollOptions{}, fmt.Errorf("ntfy: since must be Unix seconds within the past hour; list also accepts latest")
		}
	}
	if exact {
		if !idPattern.MatchString(id) {
			return pollOptions{}, fmt.Errorf("ntfy: id must be exactly 12 alphanumeric characters")
		}
		options.limit = 1
	}
	if _, present := args["limit"]; present {
		value, err := strconv.Atoi(limit)
		if err != nil || value < 1 || value > 20 {
			return pollOptions{}, fmt.Errorf("ntfy: limit must be an integer 1..20")
		}
		options.limit = value
	}
	if _, present := args["priority"]; present {
		value, err := strconv.Atoi(priority)
		if err != nil || value < 1 || value > 5 {
			return pollOptions{}, fmt.Errorf("ntfy: priority must be an integer 1..5")
		}
		options.priority = value
	}
	if _, present := args["tags"]; present {
		parts := strings.Split(tags, ",")
		if len(parts) > 8 {
			return pollOptions{}, fmt.Errorf("ntfy: tags must contain 1..8 non-empty exact tags of at most 64 characters")
		}
		for _, part := range parts {
			if part == "" || utf8.RuneCountInString(part) > 64 || !utf8.ValidString(part) || strings.ContainsFunc(part, unicode.IsControl) || part != strings.TrimSpace(part) {
				return pollOptions{}, fmt.Errorf("ntfy: tags must contain 1..8 non-empty exact tags of at most 64 characters")
			}
		}
	}
	return options, nil
}

func listMessages(ctx context.Context, n *pollClient, args map[string]any) (*mcp.ToolResult, error) {
	options, err := parseOptions(args, false)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result := n.poll(ctx, options)
	return pollResult(result)
}

func getMessage(ctx context.Context, n *pollClient, args map[string]any) (*mcp.ToolResult, error) {
	options, err := parseOptions(args, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result := n.poll(ctx, options)
	return pollResult(result)
}

func pollResult(value envelope) (*mcp.ToolResult, error) {
	result, err := mcp.JSONResult(value)
	if result != nil && value.Error != "" {
		result.IsError = true
	}
	return result, err
}

func (n *pollClient) poll(ctx context.Context, options pollOptions) envelope {
	result := envelope{Messages: []message{}, Status: "ok", Truncations: map[string]bool{"response_bytes": false, "line_bytes": false, "message_limit": false, "output_bytes": false}, Caveat: cacheCaveat}
	result.Scope.Topic, result.Window.Since, result.Window.Until = n.topic, options.since, time.Now().Unix()
	query := url.Values{"poll": {"1"}, "since": {options.since}}
	if options.id != "" {
		query.Set("id", options.id)
	}
	if options.priority != 0 {
		query.Set("priority", strconv.Itoa(options.priority))
	}
	if options.tags != "" {
		query.Set("tags", options.tags)
	}
	response, err := n.request(ctx, query)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.readReplay(response, options, n.topic)
	}
	slices.Reverse(result.Messages)
	slices.SortStableFunc(result.Messages, func(a, b message) int {
		if a.Time > b.Time {
			return -1
		}
		if a.Time < b.Time {
			return 1
		}
		return 0
	})
	if len(result.Messages) > options.limit {
		result.Messages = result.Messages[:options.limit]
		result.AdapterTruncated = true
		result.Truncations["message_limit"] = true
	}
	budget, kept := 4096, 0
	for _, value := range result.Messages {
		size := 512 + jsonStringBytes(value.Title) + jsonStringBytes(value.Message)
		for _, tag := range value.Tags {
			size += jsonStringBytes(tag) + 1
		}
		for _, flag := range value.flags {
			size += jsonStringBytes(flag) + 1
		}
		if budget+size >= maxOutputBytes {
			break
		}
		budget += size
		kept++
	}
	if kept < len(result.Messages) {
		result.Messages = result.Messages[:kept]
		result.AdapterTruncated = true
		result.Truncations["output_bytes"] = true
	}
	for _, value := range result.Messages {
		if len(value.flags) > 0 {
			result.AdapterTruncated = true
			for _, flag := range value.flags {
				result.Truncations[flag] = true
			}
		}
	}
	result.ReturnedCount = len(result.Messages)
	result.Incomplete = result.UpstreamTruncated || result.AdapterTruncated || result.Error != ""
	if result.Incomplete {
		result.Status = "incomplete"
	} else if options.id != "" && len(result.Messages) == 0 {
		result.Status = "not_found"
	}
	if result.Error != "" {
		result.Status = "error"
	}
	return result
}

func (result *envelope) readReplay(response *http.Response, options pollOptions, topic string) {
	result.UpstreamTruncated = response.Header.Get("X-Messages-Truncated") == "1"
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	_ = response.Body.Close()
	if len(data) > maxResponseBytes {
		data = data[:maxResponseBytes]
		result.AdapterTruncated = true
		result.Truncations["response_bytes"] = true
		result.Error = "ntfy: incomplete replay exceeds 64 KiB response limit; narrow the since window or filters"
		if end := bytes.LastIndexByte(data, '\n'); end >= 0 {
			data = data[:end+1]
		} else {
			data = nil
		}
	} else if readErr != nil {
		result.Error = "ntfy: incomplete replay could not be fully read; retry explicitly with a narrower window"
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) > maxLineBytes {
			result.AdapterTruncated = true
			result.Truncations["line_bytes"] = true
			result.Error = "ntfy: incomplete replay exceeds 8 KiB line limit"
			break
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record struct {
			Event    string   `json:"event"`
			Topic    string   `json:"topic"`
			Encoding string   `json:"encoding"`
			ID       string   `json:"id"`
			Time     int64    `json:"time"`
			Priority int      `json:"priority"`
			Tags     []string `json:"tags"`
			Title    string   `json:"title"`
			Message  string   `json:"message"`
			Expires  *int64   `json:"expires"`
		}
		if !utf8.Valid(line) || json.Unmarshal(line, &record) != nil || record.Event == "" {
			result.Error = "ntfy: incomplete replay contains invalid NDJSON"
			break
		}
		if record.Event != "message" {
			continue
		}
		if record.Encoding != "" {
			result.Error = "ntfy: incomplete replay contains unsupported encoded binary content"
			break
		}
		if record.Topic != topic {
			result.Error = "ntfy: incomplete replay contains a message outside the configured topic"
			break
		}
		if !idPattern.MatchString(record.ID) || record.Time <= 0 || record.Priority < 0 || record.Priority > 5 || record.Expires != nil && *record.Expires < 0 {
			result.Error = "ntfy: incomplete replay contains an invalid message record"
			break
		}
		if options.id != "" && record.ID != options.id {
			continue
		}
		if record.Priority == 0 {
			record.Priority = 3
		}
		if options.priority != 0 && record.Priority != options.priority {
			continue
		}
		matches := true
		if options.tags != "" {
			for _, tag := range strings.Split(options.tags, ",") {
				if !slices.Contains(record.Tags, tag) {
					matches = false
					break
				}
			}
		}
		if !matches {
			continue
		}
		value := message{ID: record.ID, Time: record.Time, Priority: record.Priority, Tags: []string{}, Expires: record.Expires}
		value.Title = capText(record.Title, 128, record.ID+".title", &value.flags)
		value.Message = capText(record.Message, 1024, record.ID+".message", &value.flags)
		if len(record.Tags) > 8 {
			value.flags = append(value.flags, record.ID+".tags")
			record.Tags = record.Tags[:8]
		}
		for index, tag := range record.Tags {
			value.Tags = append(value.Tags, capText(tag, 64, fmt.Sprintf("%s.tags[%d]", record.ID, index), &value.flags))
		}
		result.Messages = append(result.Messages, value)
	}
}

func capText(value string, limit int, field string, flags *[]string) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	*flags = append(*flags, field)
	return string(runes[:limit])
}

func jsonStringBytes(value string) int {
	size := 2
	for _, character := range value {
		switch {
		case character == '"' || character == '\\' || character == '\n' || character == '\r' || character == '\t' || character == '\b' || character == '\f':
			size += 2
		case character < 0x20 || character == '<' || character == '>' || character == '&' || character == '\u2028' || character == '\u2029':
			size += 6
		default:
			size += utf8.RuneLen(character)
		}
	}
	return size
}

func (n *pollClient) request(ctx context.Context, query url.Values) (*http.Response, error) {
	target := n.baseURL + "/" + n.topic + "/json?" + query.Encode()
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, fmt.Errorf("ntfy: invalid poll request")
		}
		request.Header.Set("Authorization", "Bearer "+n.apiToken)
		request.Header.Set("Accept", "application/x-ndjson")
		response, err := n.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("ntfy: request failed; check TLS, connectivity and the 5-second deadline")
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			_ = response.Body.Close()
			if err := waitForRetry(ctx, response.Header.Get("Retry-After"), attempt); err != nil {
				return nil, err
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			switch response.StatusCode {
			case 401, 403:
				return nil, fmt.Errorf("ntfy: HTTP %d; authentication or topic read authorization denied; verify dedicated read-only user and topic ACL", response.StatusCode)
			case 404:
				return nil, fmt.Errorf("ntfy: HTTP 404; configured topic or poll route is inaccessible, not a message not_found result")
			case 429:
				return nil, fmt.Errorf("ntfy: HTTP 429; request or replay bandwidth budget exhausted; retry later")
			default:
				return nil, fmt.Errorf("ntfy: HTTP %d; finite topic poll failed; redirects are not followed", response.StatusCode)
			}
		}
		contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil || contentType != "application/x-ndjson" {
			_ = response.Body.Close()
			return nil, fmt.Errorf("ntfy: unsupported response type; expected ntfy NDJSON poll")
		}
		return response, nil
	}
	return nil, fmt.Errorf("ntfy: rate limit retry budget exhausted")
}

func waitForRetry(ctx context.Context, retry string, attempt int) error {
	wait := time.Duration(attempt+1) * 100 * time.Millisecond
	if retry != "" {
		if seconds, err := strconv.ParseUint(retry, 10, 32); err == nil {
			wait = time.Duration(seconds) * time.Second
		} else if at, err := http.ParseTime(retry); err == nil {
			wait = max(time.Duration(0), time.Until(at))
		} else if strings.Trim(retry, "0123456789") == "" {
			return fmt.Errorf("ntfy: Retry-After exceeds retry budget; retry later")
		}
	}
	if deadline, ok := ctx.Deadline(); ok && wait >= time.Until(deadline) {
		return fmt.Errorf("ntfy: Retry-After exceeds the 5-second operation budget; retry later")
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("ntfy: operation deadline or cancellation during rate-limit backoff")
	case <-timer.C:
		return nil
	}
}
