// Package gpeople is the Switchboard adapter for the Google People API v1.
//
// It mirrors the structure of the other Google Workspace adapters (gmail,
// gcal, gdrive, gdocs, gsheets, gslides, gforms, gtasks, gchat): raw HTTP
// against people.googleapis.com/v1 using a Bearer token, with OAuth
// refresh handled transparently on 401 responses. The OAuth flow itself
// lives in the shared googleoauth package.
//
// The Google People API exposes:
//
//  1. People — the authenticated user's contacts plus public profile data.
//     Listed under /people/me/connections, fetched at /people/{personId}.
//     Searched via /people:searchContacts.
//  2. Directory people — coworkers from a Google Workspace organization's
//     shared directory. Listed under /people:listDirectoryPeople, searched
//     via /people:searchDirectoryPeople.
//  3. Other contacts — auto-collected contacts (people the user has emailed
//     but never explicitly saved). Listed under /otherContacts.
//
// The People API requires "field masks" (personFields or readMask) on every
// read endpoint and updatePersonFields on update. The handlers pick sensible
// defaults so the LLM only has to override when it wants something unusual.
package gpeople

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

var compactResult = compact.MustLoadWithOverlay("gpeople", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes
var viewSets = compactResult.Views

type gpeople struct {
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
	_ mcp.FieldCompactionIntegration = (*gpeople)(nil)
	_ mcp.PlainTextCredentials       = (*gpeople)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gpeople)(nil)
	_ compact.ToolViewsIntegration   = (*gpeople)(nil)
)

func (g *gpeople) PlainTextKeys() []string {
	return []string{"base_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// defaultPersonFields is used when a tool's personFields/readMask arg is
// omitted. Covers the most commonly useful identity + contact fields.
const defaultPersonFields = "names,emailAddresses,phoneNumbers,organizations,addresses,biographies,urls,photos,metadata"

// New returns a fresh, unconfigured gpeople integration.
func New() mcp.Integration {
	return &gpeople{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://people.googleapis.com/v1",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gpeople); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gpeople) Name() string { return "gpeople" }

func (g *gpeople) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gpeople: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

// Healthy probes the People API by fetching the authenticated user's own
// profile (people/me) with just the names field. This is cheap, idempotent,
// and validates the OAuth token. Anything other than 401/403 counts as
// healthy.
func (g *gpeople) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+"/people/me?personFields=names", nil)
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

func (g *gpeople) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gpeople) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gpeople) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gpeople) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gpeople) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := viewSets[toolName]
	return vs, ok
}

// --- HTTP helpers ---

func (g *gpeople) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true)
}

func (g *gpeople) buildBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gpeople) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry bool) (json.RawMessage, error) {
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

		newToken, rerr := RefreshAccessToken(g.clientID, g.clientSecret, g.refreshToken)
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gpeople API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gpeople API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gpeople) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gpeople) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gpeople) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gpeople) delete(ctx context.Context, path string) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", path, nil)
}

func (g *gpeople) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gpeople")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gpeople", ic)
}

// --- Handler types ---

type handlerFunc func(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("gpeople_list_contacts"):           listContacts,
	mcp.ToolName("gpeople_search_contacts"):         searchContacts,
	mcp.ToolName("gpeople_get_person"):              getPerson,
	mcp.ToolName("gpeople_create_contact"):          createContact,
	mcp.ToolName("gpeople_update_contact"):          updateContact,
	mcp.ToolName("gpeople_delete_contact"):          deleteContact,
	mcp.ToolName("gpeople_list_directory_people"):   listDirectoryPeople,
	mcp.ToolName("gpeople_search_directory_people"): searchDirectoryPeople,
	mcp.ToolName("gpeople_list_other_contacts"):     listOtherContacts,
}
