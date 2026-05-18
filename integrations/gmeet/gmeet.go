// Package gmeet is the Switchboard adapter for the Google Meet REST API v2.
//
// It mirrors the structure of the other Google Workspace adapters (gmail,
// gcal, gdrive, gdocs, gsheets, gslides, gforms, gtasks, gchat, gpeople):
// raw HTTP against meet.googleapis.com/v2 using a Bearer token, with OAuth
// refresh handled transparently on 401 responses. The OAuth flow itself
// lives in the shared googleoauth package.
//
// The Meet REST API exposes two top-level resource trees:
//
//  1. Meeting spaces (/v2/spaces) — persistent meeting rooms with a join
//     URI, dial-in PIN, and configurable access type. A space is created
//     once and may host many conferences. spaces support create / get /
//     update / endActiveConference.
//  2. Conference records (/v2/conferenceRecords) — read-only history of
//     past conferences. Each conference record has child collections for
//     participants, recordings, and transcripts. Transcripts in turn have
//     entries (one per spoken segment).
//
// Spaces use opaque names like "spaces/{spaceId}"; conference records use
// "conferenceRecords/{recordId}". Handlers accept either the full resource
// name or the bare ID — see normalizeSpaceName / normalizeConferenceRecord.
package gmeet

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("gmeet", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes
var viewSets = compactResult.Views

type gmeet struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	client       *http.Client
	baseURL      string
	configSvc    mcp.ConfigService
	mu           sync.Mutex
}

var (
	_ mcp.FieldCompactionIntegration = (*gmeet)(nil)
	_ mcp.PlainTextCredentials       = (*gmeet)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gmeet)(nil)
	_ compact.ToolViewsIntegration   = (*gmeet)(nil)
)

func (g *gmeet) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gmeet integration.
func New() mcp.Integration {
	return &gmeet{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://meet.googleapis.com/v2",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gmeet); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gmeet) Name() string { return "gmeet" }

func (g *gmeet) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gmeet: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the Meet API by listing the user's conference records
// with pageSize=1. This is cheap, idempotent, and validates the OAuth
// token. Anything other than 401/403 counts as healthy.
func (g *gmeet) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/conferenceRecords?pageSize=1", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	resp, err := g.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode != 401 && resp.StatusCode != 403
}

func (g *gmeet) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gmeet) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gmeet) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gmeet) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gmeet) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := viewSets[toolName]
	return vs, ok
}

// --- HTTP helpers ---

func (g *gmeet) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gmeet) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gmeet) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
	bodyReader, err := g.buildBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 && canRetry && g.refreshToken != "" && g.clientID != "" && g.clientSecret != "" {
		g.mu.Lock()
		currentToken := g.accessToken
		g.mu.Unlock()

		newToken, rerr := RefreshAccessToken(ctx, g.clientID, g.clientSecret, g.refreshToken)
		if rerr == nil {
			g.mu.Lock()
			if g.accessToken == currentToken {
				g.accessToken = newToken
				g.persistToken(newToken)
			}
			g.mu.Unlock()
			return g.doRequestInner(ctx, method, fullURL, body, contentType, false)
		}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gmeet API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gmeet API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gmeet) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gmeet) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gmeet) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gmeet) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gmeet")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gmeet", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gmeet, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gmeet_create_space"):            createSpace,
	mcp.ToolName("gmeet_get_space"):               getSpace,
	mcp.ToolName("gmeet_update_space"):            updateSpace,
	mcp.ToolName("gmeet_end_active_conference"):   endActiveConference,
	mcp.ToolName("gmeet_list_conference_records"): listConferenceRecords,
	mcp.ToolName("gmeet_get_conference_record"):   getConferenceRecord,
	mcp.ToolName("gmeet_list_participants"):       listParticipants,
	mcp.ToolName("gmeet_list_recordings"):         listRecordings,
	mcp.ToolName("gmeet_list_transcripts"):        listTranscripts,
	mcp.ToolName("gmeet_list_transcript_entries"): listTranscriptEntries,
}
