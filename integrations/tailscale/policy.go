package tailscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// Policy files are HuJSON (JSON with comments and trailing commas). They are
// passed through as text so comments and formatting survive a get/edit/set
// round trip; converting to JSON would silently drop every comment.

func getPolicyFile(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	data, hdr, err := t.do(ctx, http.MethodGet, t.tailnetPath("/acl"), nil, requestOptions{accept: contentTypeHuJSON})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"etag":   hdr.Get("ETag"),
		"policy": string(data),
	})
}

// normalizeETag quotes a bare ETag so callers can pass either the header
// value verbatim or just the hash.
func normalizeETag(tag string) string {
	tag = strings.TrimSpace(tag)
	if strings.HasPrefix(tag, `"`) || strings.HasPrefix(tag, "W/") {
		return tag
	}
	return `"` + tag + `"`
}

func setPolicyFile(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	policy := r.Str("policy")
	ifMatch := r.Str("if_match")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("policy", policy); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("if_match", ifMatch); err != nil {
		return mcp.ErrResult(fmt.Errorf("if_match is required: pass the etag from tailscale_get_policy_file (or ts-default to replace only an untouched default policy) so concurrent edits are not overwritten"))
	}
	_, hdr, err := t.do(ctx, http.MethodPost, t.tailnetPath("/acl"), []byte(policy), requestOptions{
		contentType: contentTypeHuJSON,
		accept:      contentTypeHuJSON,
		ifMatch:     normalizeETag(ifMatch),
	})
	if err != nil {
		if strings.Contains(err.Error(), "(412)") {
			return mcp.ErrResult(fmt.Errorf("%w; the policy file changed since it was read, call tailscale_get_policy_file again and reapply the edit with the new etag", err))
		}
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"status": "updated",
		"etag":   hdr.Get("ETag"),
	})
}

func validatePolicyFile(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	policy := r.Str("policy")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var (
		body []byte
		ct   string
	)
	switch {
	case policy != "":
		body, ct = []byte(policy), contentTypeHuJSON
	case args["tests"] != nil:
		tests, err := json.Marshal(args["tests"])
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("tests: %w", err))
		}
		if !bytes.HasPrefix(bytes.TrimSpace(tests), []byte("[")) {
			return mcp.ErrResult(fmt.Errorf("tests must be an array of ACL test objects"))
		}
		body, ct = tests, contentTypeJSON
	default:
		return mcp.ErrResult(fmt.Errorf("policy or tests is required"))
	}
	data, _, err := t.do(ctx, http.MethodPost, t.tailnetPath("/acl/validate"), body, requestOptions{contentType: ct})
	if err != nil {
		return mcp.ErrResult(err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) || bytes.Equal(trimmed, []byte("null")) {
		return mcp.JSONResult(map[string]any{"valid": true})
	}
	return mcp.RawResult(data)
}

func previewPolicyRules(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	previewType := r.Str("type")
	previewFor := r.Str("preview_for")
	policy := r.Str("policy")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("type", previewType); err != nil {
		return mcp.ErrResult(err)
	}
	if previewType != "user" && previewType != "ipport" {
		return mcp.ErrResult(fmt.Errorf("type must be user or ipport, got %q", previewType))
	}
	if err := requireStr("preview_for", previewFor); err != nil {
		return mcp.ErrResult(err)
	}
	if policy == "" {
		current, _, err := t.do(ctx, http.MethodGet, t.tailnetPath("/acl"), nil, requestOptions{accept: contentTypeHuJSON})
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("fetch current policy file: %w", err))
		}
		policy = string(current)
	}
	vals := url.Values{"type": {previewType}, "previewFor": {previewFor}}
	data, _, err := t.do(ctx, http.MethodPost, t.tailnetPath("/acl/preview")+"?"+vals.Encode(), []byte(policy), requestOptions{contentType: contentTypeHuJSON})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
