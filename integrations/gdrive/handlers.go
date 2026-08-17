package gdrive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Files: search & read ────────────────────────────────────────────

func listFiles(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"q":                         r.Str("q"),
		"pageSize":                  r.Str("page_size"),
		"pageToken":                 r.Str("page_token"),
		"orderBy":                   r.Str("order_by"),
		"fields":                    r.Str("fields"),
		"corpora":                   r.Str("corpora"),
		"driveId":                   r.Str("drive_id"),
		"includeItemsFromAllDrives": r.Str("include_items_from_all_drives"),
		"spaces":                    r.Str("spaces"),
	}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if params["fields"] == "" {
		params["fields"] = "nextPageToken,files(id,name,mimeType,parents,owners,modifiedTime,size,webViewLink,iconLink,trashed,starred,shared)"
	}
	data, err := g.get(ctx, "/files%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	fields := r.Str("fields")
	if fields == "" {
		fields = "*"
	}
	params := map[string]string{"fields": fields}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s%s", fid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// downloadFile downloads a non-Google file's binary content. We cap the
// download at the user's max_bytes (default 5 MB, hard ceiling 10 MB) and
// return a JSON envelope with base64 content for binary, inline text for
// text/* content types. This makes the result LLM-friendly without
// requiring tools to handle raw binary in the conversation.
func downloadFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{
		"alt":              "media",
		"acknowledgeAbuse": r.Str("acknowledge_abuse"),
	}
	addSupportsAllDrives(params, r)
	maxBytes := r.OptInt("max_bytes", 5_000_000)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if maxBytes > 10_000_000 {
		maxBytes = 10_000_000
	}
	data, ct, err := g.doRaw(ctx, "GET", fmt.Sprintf("/files/%s%s", fid, queryEncode(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return downloadEnvelope(data, ct, maxBytes)
}

func exportFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	mt := r.Str("mime_type")
	maxBytes := r.OptInt("max_bytes", 5_000_000)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if maxBytes > 10_000_000 {
		maxBytes = 10_000_000
	}
	params := map[string]string{"mimeType": mt}
	data, ct, err := g.doRaw(ctx, "GET", fmt.Sprintf("/files/%s/export%s", fid, queryEncode(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return downloadEnvelope(data, ct, maxBytes)
}

// downloadEnvelope packages binary content for LLM consumption: text/*
// content types return inline, others return base64-encoded. The caller's
// max_bytes is enforced here.
func downloadEnvelope(data []byte, contentType string, maxBytes int) (*mcp.ToolResult, error) {
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}
	envelope := map[string]any{
		"content_type": contentType,
		"bytes":        len(data),
		"truncated":    truncated,
	}
	if isTextType(contentType) {
		envelope["content"] = string(data)
	} else {
		envelope["content_base64"] = base64.StdEncoding.EncodeToString(data)
	}
	return mcp.JSONResult(envelope)
}

func isTextType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/json", "application/xml", "application/javascript",
		"application/x-yaml", "application/yaml", "application/x-sh":
		return true
	}
	return false
}

// ── Files: write ────────────────────────────────────────────────────

// buildFileMetadata assembles a Files resource from convenience args. If
// the caller passes a `body` arg, it overrides convenience args entirely.
// Content is handled separately by the upload code path.
func buildFileMetadata(r *mcp.Args) (map[string]any, error) {
	if raw := r.Str("body"); raw != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for body: %w", err)
		}
		return out, nil
	}

	body := map[string]any{}
	if v := r.Str("name"); v != "" {
		body["name"] = v
	}
	if v := r.Str("mime_type"); v != "" {
		body["mimeType"] = v
	}
	if v := r.Str("description"); v != "" {
		body["description"] = v
	}
	if parents := r.StrSlice("parents"); len(parents) > 0 {
		body["parents"] = parents
	}
	if v := r.Str("starred"); v != "" {
		body["starred"] = v == "true"
	}
	if v := r.Str("trashed"); v != "" {
		body["trashed"] = v == "true"
	}
	if v := r.Str("app_properties"); v != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("invalid JSON for app_properties: %w", err)
		}
		body["appProperties"] = m
	}
	if v := r.Str("properties"); v != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("invalid JSON for properties: %w", err)
		}
		body["properties"] = m
	}
	return body, nil
}

// resolveContent returns the file content bytes from convenience args.
// Returns (nil, nil) if no content was supplied.
func resolveContent(r *mcp.Args) ([]byte, error) {
	if v := r.Str("content_base64"); v != "" {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 for content_base64: %w", err)
		}
		return decoded, nil
	}
	if v := r.Str("content"); v != "" {
		return []byte(v), nil
	}
	return nil, nil
}

func createFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	meta, merr := buildFileMetadata(r)
	if merr != nil {
		return mcp.ErrResult(merr)
	}
	content, cerr := resolveContent(r)
	if cerr != nil {
		return mcp.ErrResult(cerr)
	}
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if content == nil {
		// Metadata-only insert hits the standard API endpoint.
		params["fields"] = "id,name,mimeType,parents"
		data, err := g.post(ctx, "/files"+queryEncode(params), meta)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	// Multipart upload: metadata + content body.
	params["uploadType"] = "multipart"
	body, contentType, berr := multipartUpload(meta, content, r.Str("mime_type"))
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	data, err := g.doUpload(ctx, "POST", "/files"+queryEncode(params), contentType, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	meta, merr := buildFileMetadata(r)
	if merr != nil {
		return mcp.ErrResult(merr)
	}
	content, cerr := resolveContent(r)
	if cerr != nil {
		return mcp.ErrResult(cerr)
	}
	params := map[string]string{
		"addParents":    r.Str("add_parents"),
		"removeParents": r.Str("remove_parents"),
	}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if content == nil {
		data, err := g.patch(ctx, fmt.Sprintf("/files/%s%s", fid, queryEncode(params)), meta)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	params["uploadType"] = "multipart"
	body, contentType, berr := multipartUpload(meta, content, r.Str("mime_type"))
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	data, err := g.doUpload(ctx, "PATCH", fmt.Sprintf("/files/%s%s", fid, queryEncode(params)), contentType, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// multipartUpload encodes a Drive multipart/related upload: metadata JSON
// followed by the file content. Returns the assembled body, the
// multipart/related content-type with boundary, and any error.
func multipartUpload(meta map[string]any, content []byte, contentMimeType string) ([]byte, string, error) {
	var buf strings.Builder
	mw := multipart.NewWriter(&stringBuilderWriter{&buf})

	metaHeader := textproto.MIMEHeader{}
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	mp, err := mw.CreatePart(metaHeader)
	if err != nil {
		return nil, "", err
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, "", err
	}
	if _, err := mp.Write(metaJSON); err != nil {
		return nil, "", err
	}

	mediaCT := contentMimeType
	if mediaCT == "" {
		if v, ok := meta["mimeType"].(string); ok {
			mediaCT = v
		}
	}
	if mediaCT == "" {
		mediaCT = "application/octet-stream"
	}
	mediaHeader := textproto.MIMEHeader{}
	mediaHeader.Set("Content-Type", mediaCT)
	mp2, err := mw.CreatePart(mediaHeader)
	if err != nil {
		return nil, "", err
	}
	if _, err := mp2.Write(content); err != nil {
		return nil, "", err
	}

	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return []byte(buf.String()), "multipart/related; boundary=" + mw.Boundary(), nil
}

// stringBuilderWriter adapts strings.Builder to io.Writer for multipart.NewWriter.
type stringBuilderWriter struct{ b *strings.Builder }

func (s *stringBuilderWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

func copyFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	body, berr := buildFileMetadata(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/files/%s/copy%s", fid, queryEncode(params)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/files/%s%s", fid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func trashFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/files/%s%s", fid, queryEncode(params)), map[string]any{"trashed": true})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func untrashFile(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/files/%s%s", fid, queryEncode(params)), map[string]any{"trashed": false})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func emptyTrash(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{"driveId": r.Str("drive_id")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/files/trash%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFolder(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"name":     r.Str("name"),
		"mimeType": "application/vnd.google-apps.folder",
	}
	if v := r.Str("description"); v != "" {
		body["description"] = v
	}
	if parents := r.StrSlice("parents"); len(parents) > 0 {
		body["parents"] = parents
	}
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/files"+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateIDs(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"count": r.Str("count"),
		"space": r.Str("space"),
		"type":  r.Str("type"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/generateIds%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Permissions ─────────────────────────────────────────────────────

func listPermissions(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{
		"pageSize":             r.Str("page_size"),
		"pageToken":            r.Str("page_token"),
		"useDomainAdminAccess": r.Str("use_domain_admin_access"),
		"fields":               "nextPageToken,permissions(id,type,role,emailAddress,domain,displayName,deleted)",
	}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/permissions%s", fid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPermission(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	pid := r.Str("permission_id")
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/permissions/%s%s", fid, pid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func buildPermissionBody(r *mcp.Args) (map[string]any, error) {
	if raw := r.Str("body"); raw != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for body: %w", err)
		}
		return out, nil
	}
	body := map[string]any{}
	if v := r.Str("role"); v != "" {
		body["role"] = v
	}
	if v := r.Str("type"); v != "" {
		body["type"] = v
	}
	if v := r.Str("email_address"); v != "" {
		body["emailAddress"] = v
	}
	if v := r.Str("domain"); v != "" {
		body["domain"] = v
	}
	if v := r.Str("allow_file_discovery"); v != "" {
		body["allowFileDiscovery"] = v == "true"
	}
	if v := r.Str("expiration_time"); v != "" {
		body["expirationTime"] = v
	}
	return body, nil
}

func createPermission(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	body, berr := buildPermissionBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	params := map[string]string{
		"sendNotificationEmail": r.Str("send_notification_email"),
		"emailMessage":          r.Str("email_message"),
		"transferOwnership":     r.Str("transfer_ownership"),
		"moveToNewOwnersRoot":   r.Str("move_to_new_owners_root"),
	}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/files/%s/permissions%s", fid, queryEncode(params)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePermission(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	pid := r.Str("permission_id")
	body, berr := buildPermissionBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	params := map[string]string{
		"transferOwnership": r.Str("transfer_ownership"),
	}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/files/%s/permissions/%s%s", fid, pid, queryEncode(params)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePermission(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	pid := r.Str("permission_id")
	params := map[string]string{}
	addSupportsAllDrives(params, r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/files/%s/permissions/%s%s", fid, pid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Revisions ───────────────────────────────────────────────────────

func listRevisions(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{
		"pageSize":  r.Str("page_size"),
		"pageToken": r.Str("page_token"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/revisions%s", fid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRevision(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	rid := r.Str("revision_id")
	params := map[string]string{"fields": r.Str("fields")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/revisions/%s%s", fid, rid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRevision(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	rid := r.Str("revision_id")
	var body map[string]any
	if raw := r.Str("body"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
	} else {
		body = map[string]any{}
		if v := r.Str("keep_forever"); v != "" {
			body["keepForever"] = v == "true"
		}
		if v := r.Str("published"); v != "" {
			body["published"] = v == "true"
		}
		if v := r.Str("publish_auto"); v != "" {
			body["publishAuto"] = v == "true"
		}
		if v := r.Str("published_outside_domain"); v != "" {
			body["publishedOutsideDomain"] = v == "true"
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/files/%s/revisions/%s", fid, rid), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteRevision(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	rid := r.Str("revision_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/files/%s/revisions/%s", fid, rid)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Comments + replies ──────────────────────────────────────────────

func listComments(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	params := map[string]string{
		"pageSize":          r.Str("page_size"),
		"pageToken":         r.Str("page_token"),
		"includeDeleted":    r.Str("include_deleted"),
		"startModifiedTime": r.Str("start_modified_time"),
		"fields":            "nextPageToken,comments(id,content,htmlContent,author,createdTime,modifiedTime,resolved,deleted,quotedFileContent,replies)",
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/comments%s", fid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getComment(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	cid := r.Str("comment_id")
	params := map[string]string{
		"includeDeleted": r.Str("include_deleted"),
		"fields":         "*",
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/files/%s/comments/%s%s", fid, cid, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createComment(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	var body map[string]any
	if raw := r.Str("body"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
	} else {
		body = map[string]any{"content": r.Str("content")}
		if v := r.Str("anchor"); v != "" {
			body["anchor"] = v
		}
		if v := r.Str("quoted_file_content"); v != "" {
			var qfc map[string]any
			if err := json.Unmarshal([]byte(v), &qfc); err != nil {
				return mcp.ErrResult(fmt.Errorf("invalid JSON for quoted_file_content: %w", err))
			}
			body["quotedFileContent"] = qfc
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/files/%s/comments?fields=*", fid), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateComment(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	cid := r.Str("comment_id")
	var body map[string]any
	if raw := r.Str("body"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
	} else {
		body = map[string]any{}
		if v := r.Str("content"); v != "" {
			body["content"] = v
		}
		if v := r.Str("resolved"); v != "" {
			body["resolved"] = v == "true"
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/files/%s/comments/%s?fields=*", fid, cid), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	cid := r.Str("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/files/%s/comments/%s", fid, cid)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createReply(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fid := r.Str("file_id")
	cid := r.Str("comment_id")
	var body map[string]any
	if raw := r.Str("body"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
	} else {
		body = map[string]any{"content": r.Str("content")}
		if v := r.Str("action"); v != "" {
			body["action"] = v
		}
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/files/%s/comments/%s/replies?fields=*", fid, cid), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Shared drives ───────────────────────────────────────────────────

func listDrives(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"pageSize":             r.Str("page_size"),
		"pageToken":            r.Str("page_token"),
		"q":                    r.Str("q"),
		"useDomainAdminAccess": r.Str("use_domain_admin_access"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/drives%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	did := r.Str("drive_id")
	params := map[string]string{"useDomainAdminAccess": r.Str("use_domain_admin_access")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/drives/%s%s", did, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{"name": r.Str("name")}
	if v := r.Str("theme_id"); v != "" {
		body["themeId"] = v
	}
	reqID := r.Str("request_id")
	if reqID == "" {
		reqID = generateRequestID()
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/drives?requestId=%s", reqID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	did := r.Str("drive_id")
	var body map[string]any
	if raw := r.Str("body"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for body: %w", err))
		}
	} else {
		body = map[string]any{}
		if v := r.Str("name"); v != "" {
			body["name"] = v
		}
		if v := r.Str("theme_id"); v != "" {
			body["themeId"] = v
		}
	}
	params := map[string]string{"useDomainAdminAccess": r.Str("use_domain_admin_access")}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.patch(ctx, fmt.Sprintf("/drives/%s%s", did, queryEncode(params)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	did := r.Str("drive_id")
	params := map[string]string{
		"allowItemDeletion":    r.Str("allow_item_deletion"),
		"useDomainAdminAccess": r.Str("use_domain_admin_access"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/drives/%s%s", did, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func hideDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	did := r.Str("drive_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/drives/%s/hide", did), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unhideDrive(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	did := r.Str("drive_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, fmt.Sprintf("/drives/%s/unhide", did), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── About ───────────────────────────────────────────────────────────

func getAbout(ctx context.Context, g *gdrive, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fields := r.Str("fields")
	if fields == "" {
		fields = "*"
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/about?fields=%s", fields)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
