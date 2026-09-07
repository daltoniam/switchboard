package microsoft365

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func driveRoot(userID, driveID string) string {
	if driveID != "" {
		return "/drives/" + url.PathEscape(driveID)
	}
	return userPath(userID) + "/drive"
}

func encodeDrivePath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func driveItemPath(userID, driveID, itemID, path string) (string, error) {
	root := driveRoot(userID, driveID)
	if itemID != "" {
		return root + "/items/" + url.PathEscape(itemID), nil
	}
	if path != "" {
		return root + "/root:/" + encodeDrivePath(path), nil
	}
	return root + "/root", nil
}

func driveRelation(item, rel string) string {
	if strings.Contains(item, "/root:/") && !strings.HasSuffix(item, ":") {
		return item + ":/" + rel
	}
	return item + "/" + rel
}

func uploadContentPath(parent, name string) string {
	if strings.Contains(parent, "/root:/") && !strings.HasSuffix(parent, ":") {
		return parent + "/" + name + ":/content"
	}
	return parent + ":/" + name + ":/content"
}

func listDriveItems(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		driveID := r.Str("drive_id")
		itemID := r.Str("item_id")
		path := r.Str("path")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		item, err := driveItemPath(userID, driveID, itemID, path)
		if err != nil {
			return "", nil, err
		}
		return driveRelation(item, "children") + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func getDriveItem(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	driveID := r.Str("drive_id")
	itemID := r.Str("item_id")
	path := r.Str("path")
	sel := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	item, err := driveItemPath(userID, driveID, itemID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.get(ctx, item+queryEncode(map[string]string{"$select": sel}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func searchDrive(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		driveID := r.Str("drive_id")
		q := r.Str("q")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		q = strings.ReplaceAll(q, "'", "''")
		return driveRoot(userID, driveID) + "/root/search(q='" + q + "')" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func downloadDriveItem(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	driveID := r.Str("drive_id")
	itemID := r.Str("item_id")
	path := r.Str("path")
	maxBytes := r.OptInt("max_bytes", defaultDownloadBy)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if itemID == "" && path == "" {
		return mcp.ErrResult(fmt.Errorf("provide item_id or path"))
	}
	if maxBytes > maxDownloadBytes {
		maxBytes = maxDownloadBytes
	}
	item, err := driveItemPath(userID, driveID, itemID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, ct, err := m.getRaw(ctx, driveRelation(item, "content"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return downloadEnvelope(data, ct, maxBytes)
}

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
		"application/x-yaml", "application/yaml":
		return true
	}
	return false
}

func createFolder(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	driveID := r.Str("drive_id")
	itemID := r.Str("item_id")
	path := r.Str("path")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	parent, err := driveItemPath(userID, driveID, itemID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name":                              name,
		"folder":                            map[string]any{},
		"@microsoft.graph.conflictBehavior": "rename",
	}
	data, err := m.post(ctx, driveRelation(parent, "children"), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func uploadDriveItem(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	driveID := r.Str("drive_id")
	itemID := r.Str("item_id")
	path := r.Str("path")
	name := r.Str("name")
	content := r.Str("content")
	contentB64 := r.Str("content_base64")
	contentType := r.Str("content_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if content != "" && contentB64 != "" {
		return mcp.ErrResult(fmt.Errorf("provide content or content_base64, not both"))
	}
	var payload []byte
	if contentB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid content_base64: %w", err))
		}
		payload = decoded
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	} else {
		payload = []byte(content)
		if contentType == "" {
			contentType = "text/plain"
		}
	}
	if len(payload) > maxSimpleUpload {
		return mcp.ErrResult(fmt.Errorf("content is %d bytes; simple upload supports up to 4 MB", len(payload)))
	}
	parent, err := driveItemPath(userID, driveID, itemID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.put(ctx, uploadContentPath(parent, encodeDrivePath(name)), payload, contentType)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func deleteDriveItem(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	driveID := r.Str("drive_id")
	itemID := r.Str("item_id")
	path := r.Str("path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if itemID == "" && path == "" {
		return mcp.ErrResult(fmt.Errorf("provide item_id or path"))
	}
	item, err := driveItemPath(userID, driveID, itemID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.del(ctx, item)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}
