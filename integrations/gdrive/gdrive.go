// Package gdrive is the Switchboard adapter for Google Drive v3.
//
// It mirrors the structure of the gmail and gcal adapters: raw HTTP against
// www.googleapis.com/drive/v3 using a Bearer token, with OAuth refresh
// handled transparently on 401 responses. The OAuth flow itself lives in
// the shared googleoauth package.
package gdrive

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("gdrive", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes
var viewSets = compactResult.Views

type gdrive struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	client       *http.Client
	baseURL      string
	uploadURL    string
	configSvc    mcp.ConfigService
	mu           sync.Mutex
}

var (
	_ mcp.FieldCompactionIntegration = (*gdrive)(nil)
	_ mcp.PlainTextCredentials       = (*gdrive)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*gdrive)(nil)
	_ compact.ToolViewsIntegration   = (*gdrive)(nil)
)

func (g *gdrive) PlainTextKeys() []string {
	return []string{"base_url", "upload_url"}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

// New returns a fresh, unconfigured gdrive integration.
func New() mcp.Integration {
	return &gdrive{
		client:    &http.Client{Timeout: 60 * time.Second},
		baseURL:   "https://www.googleapis.com/drive/v3",
		uploadURL: "https://www.googleapis.com/upload/drive/v3",
	}
}

// SetConfigService wires the registry's config service into the integration
// so refreshed access tokens can be persisted across restarts.
func SetConfigService(i mcp.Integration, svc mcp.ConfigService) {
	if g, ok := i.(*gdrive); ok {
		g.mu.Lock()
		g.configSvc = svc
		g.mu.Unlock()
	}
}

func (g *gdrive) Name() string { return "gdrive" }

func (g *gdrive) Configure(_ context.Context, creds mcp.Credentials) error {
	g.accessToken = creds["access_token"]
	g.refreshToken = creds["refresh_token"]
	g.clientID = creds[mcp.CredKeyClientID]
	g.clientSecret = creds[mcp.CredKeyClientSecret]
	if g.accessToken == "" {
		return fmt.Errorf("gdrive: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		g.baseURL = strings.TrimRight(v, "/")
	}
	if v := creds["upload_url"]; v != "" {
		g.uploadURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (g *gdrive) Healthy(ctx context.Context) bool {
	_, err := g.get(ctx, "/about?fields=user(emailAddress)")
	return err == nil
}

func (g *gdrive) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *gdrive) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

func (g *gdrive) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (g *gdrive) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (g *gdrive) Views(toolName mcp.ToolName) (compact.ViewSet, bool) {
	vs, ok := viewSets[toolName]
	return vs, ok
}

// --- HTTP helpers ---

func (g *gdrive) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.baseURL+path, body, "application/json", true, false)
}

// doRaw issues a request and returns the raw response bytes plus Content-Type.
// Used by alt=media downloads and export endpoints which return binary or
// arbitrary content (not necessarily JSON).
func (g *gdrive) doRaw(ctx context.Context, method, path string) ([]byte, string, error) {
	return g.doRawInner(ctx, method, g.baseURL+path, true)
}

// doUpload issues a request against the upload endpoint with an arbitrary
// content-type and body bytes. Used for media uploads (uploadType=media|multipart).
func (g *gdrive) doUpload(ctx context.Context, method, path, contentType string, body []byte) (json.RawMessage, error) {
	return g.doRequestInner(ctx, method, g.uploadURL+path, body, contentType, true, true)
}

func (g *gdrive) buildBody(body any, rawBody bool) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	if rawBody {
		b, ok := body.([]byte)
		if !ok {
			return nil, fmt.Errorf("gdrive: rawBody expects []byte, got %T", body)
		}
		return bytes.NewReader(b), nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (g *gdrive) doRequestInner(ctx context.Context, method, fullURL string, body any, contentType string, canRetry, rawBody bool) (json.RawMessage, error) {
	bodyReader, err := g.buildBody(body, rawBody)
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
			return g.doRequestInner(ctx, method, fullURL, body, contentType, false, rawBody)
		}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("gdrive API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gdrive API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (g *gdrive) doRawInner(ctx context.Context, method, fullURL string, canRetry bool) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, "", err
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
			return g.doRawInner(ctx, method, fullURL, false)
		}
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("gdrive API error (%d): %s", resp.StatusCode, string(data))
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func (g *gdrive) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gdrive) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "POST", path, body)
}

func (g *gdrive) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return g.doRequest(ctx, "PATCH", path, body)
}

func (g *gdrive) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

func (g *gdrive) persistToken(token string) {
	if g.configSvc == nil {
		return
	}
	ic, ok := g.configSvc.GetIntegration("gdrive")
	if !ok || ic == nil {
		return
	}
	ic.Credentials["access_token"] = token
	_ = g.configSvc.SetIntegration("gdrive", ic)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error)

// --- Argument helpers ---

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// addSupportsAllDrives appends the supportsAllDrives=true parameter when
// requested. Most write operations on shared-drive files require it.
func addSupportsAllDrives(params map[string]string, r *mcp.Args) {
	if v := r.Str("supports_all_drives"); v != "" {
		params["supportsAllDrives"] = v
	} else {
		// Default true: passes through for both My Drive and shared drives.
		params["supportsAllDrives"] = "true"
	}
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Files: search, metadata, content
	mcp.ToolName("gdrive_list_files"):    listFiles,
	mcp.ToolName("gdrive_get_file"):      getFile,
	mcp.ToolName("gdrive_download_file"): downloadFile,
	mcp.ToolName("gdrive_export_file"):   exportFile,
	mcp.ToolName("gdrive_create_file"):   createFile,
	mcp.ToolName("gdrive_update_file"):   updateFile,
	mcp.ToolName("gdrive_copy_file"):     copyFile,
	mcp.ToolName("gdrive_delete_file"):   deleteFile,
	mcp.ToolName("gdrive_trash_file"):    trashFile,
	mcp.ToolName("gdrive_untrash_file"):  untrashFile,
	mcp.ToolName("gdrive_empty_trash"):   emptyTrash,
	mcp.ToolName("gdrive_create_folder"): createFolder,
	mcp.ToolName("gdrive_generate_ids"):  generateIDs,

	// Permissions
	mcp.ToolName("gdrive_list_permissions"):  listPermissions,
	mcp.ToolName("gdrive_get_permission"):    getPermission,
	mcp.ToolName("gdrive_create_permission"): createPermission,
	mcp.ToolName("gdrive_update_permission"): updatePermission,
	mcp.ToolName("gdrive_delete_permission"): deletePermission,

	// Revisions
	mcp.ToolName("gdrive_list_revisions"):  listRevisions,
	mcp.ToolName("gdrive_get_revision"):    getRevision,
	mcp.ToolName("gdrive_update_revision"): updateRevision,
	mcp.ToolName("gdrive_delete_revision"): deleteRevision,

	// Comments + replies
	mcp.ToolName("gdrive_list_comments"):  listComments,
	mcp.ToolName("gdrive_get_comment"):    getComment,
	mcp.ToolName("gdrive_create_comment"): createComment,
	mcp.ToolName("gdrive_update_comment"): updateComment,
	mcp.ToolName("gdrive_delete_comment"): deleteComment,
	mcp.ToolName("gdrive_create_reply"):   createReply,

	// Shared Drives
	mcp.ToolName("gdrive_list_drives"):  listDrives,
	mcp.ToolName("gdrive_get_drive"):    getDrive,
	mcp.ToolName("gdrive_create_drive"): createDrive,
	mcp.ToolName("gdrive_update_drive"): updateDrive,
	mcp.ToolName("gdrive_delete_drive"): deleteDrive,
	mcp.ToolName("gdrive_hide_drive"):   hideDrive,
	mcp.ToolName("gdrive_unhide_drive"): unhideDrive,

	// About (user + quota)
	mcp.ToolName("gdrive_get_about"): getAbout,
}
