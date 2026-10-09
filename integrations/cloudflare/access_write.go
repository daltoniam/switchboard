package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// --- Zero Trust Access: applications and policies ---

var accessAppTypes = map[string]bool{"self_hosted": true, "saas": true, "ssh": true, "vnc": true, "app_launcher": true, "warp": true, "biso": true, "bookmark": true, "dash_sso": true, "infrastructure": true, "rdp": true}

var accessDecisions = map[string]bool{"allow": true, "deny": true, "bypass": true, "non_identity": true}

func getAccessApp(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if appID == "" {
		return mcp.ErrResult(errors.New("app_id is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/access/apps/%s", acct, url.PathEscape(appID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAccessApp(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	domain := r.Str("domain")
	appType := r.Str("type")
	sessionDuration := r.Str("session_duration")
	idps := r.StrSlice("allowed_idps")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" || domain == "" {
		return mcp.ErrResult(errors.New("name and domain are required"))
	}
	if strings.Contains(domain, "://") {
		return mcp.ErrResult(errors.New("domain is a hostname with an optional path, without a scheme (for example app.example.com/admin)"))
	}
	if appType == "" {
		appType = "self_hosted"
	}
	if !accessAppTypes[appType] {
		return mcp.ErrResult(fmt.Errorf("unsupported Access application type %q", appType))
	}
	body := map[string]any{"name": name, "domain": domain, "type": appType}
	if sessionDuration != "" {
		body["session_duration"] = sessionDuration
	}
	if len(idps) > 0 {
		body["allowed_idps"] = idps
	}
	if _, ok := args["auto_redirect_to_identity"]; ok {
		v, err := mcp.ArgBool(args, "auto_redirect_to_identity")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["auto_redirect_to_identity"] = v
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/access/apps", acct), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAccessApp(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if appID == "" {
		return mcp.ErrResult(errors.New("app_id is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/access/apps/%s", acct, url.PathEscape(appID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAccessAppPolicy(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	name := r.Str("name")
	decision := r.Str("decision")
	emails := r.StrSlice("emails")
	domains := r.StrSlice("email_domains")
	loginMethods := r.StrSlice("login_methods")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if appID == "" || name == "" {
		return mcp.ErrResult(errors.New("app_id and name are required"))
	}
	if decision == "" {
		decision = "allow"
	}
	if !accessDecisions[decision] {
		return mcp.ErrResult(fmt.Errorf("decision must be allow, deny, bypass or non_identity, not %q", decision))
	}
	var include []map[string]any
	for _, email := range emails {
		include = append(include, map[string]any{"email": map[string]any{"email": strings.TrimSpace(email)}})
	}
	for _, domain := range domains {
		include = append(include, map[string]any{"email_domain": map[string]any{"domain": strings.TrimSpace(domain)}})
	}
	for _, id := range loginMethods {
		include = append(include, map[string]any{"login_method": map[string]any{"id": strings.TrimSpace(id)}})
	}
	if _, ok := args["everyone"]; ok {
		everyone, err := mcp.ArgBool(args, "everyone")
		if err != nil {
			return mcp.ErrResult(err)
		}
		if everyone {
			include = append(include, map[string]any{"everyone": map[string]any{}})
		}
	}
	raw, err := objectList(args, "include")
	if err != nil {
		return mcp.ErrResult(err)
	}
	include = append(include, raw...)
	if len(include) == 0 {
		return mcp.ErrResult(errors.New("give at least one include rule: emails, email_domains, login_methods, everyone or include"))
	}
	body := map[string]any{"name": name, "decision": decision, "include": include}
	for _, key := range []string{"exclude", "require"} {
		rules, err := objectList(args, key)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if len(rules) > 0 {
			body[key] = rules
		}
	}
	if precedence := mcp.OptInt(args, "precedence", 0); precedence > 0 {
		body["precedence"] = precedence
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/access/apps/%s/policies", acct, url.PathEscape(appID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAccessAppPolicy(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	policyID := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if appID == "" || policyID == "" {
		return mcp.ErrResult(errors.New("app_id and policy_id are required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/access/apps/%s/policies/%s", acct, url.PathEscape(appID), url.PathEscape(policyID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Cloudflared tunnels: create, token and remote configuration ---

func createTunnel(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(errors.New("name is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, fmt.Sprintf("/accounts/%s/cfd_tunnel", acct), map[string]any{"name": name, "config_src": "cloudflare"})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTunnelToken(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	return tunnelGet(ctx, c, args, "/token")
}

func getTunnelConfig(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	return tunnelGet(ctx, c, args, "/configurations")
}

func tunnelGet(ctx context.Context, c *cloudflare, args map[string]any, suffix string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tunnelID := r.Str("tunnel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tunnelID == "" {
		return mcp.ErrResult(errors.New("tunnel_id is required"))
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/cfd_tunnel/%s%s", acct, url.PathEscape(tunnelID), suffix)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTunnelConfig(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tunnelID := r.Str("tunnel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tunnelID == "" {
		return mcp.ErrResult(errors.New("tunnel_id is required"))
	}
	ingress, err := objectList(args, "ingress")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(ingress) == 0 {
		return mcp.ErrResult(errors.New("ingress needs at least one rule"))
	}
	for i, rule := range ingress {
		if service, _ := rule["service"].(string); service == "" {
			return mcp.ErrResult(fmt.Errorf("ingress rule %d needs a service (for example http://app:8080 or http_status:404)", i))
		}
	}
	if last := ingress[len(ingress)-1]; last["hostname"] != nil || last["path"] != nil {
		ingress = append(ingress, map[string]any{"service": "http_status:404"})
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"config": map[string]any{"ingress": ingress}}
	data, err := c.put(ctx, fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", acct, url.PathEscape(tunnelID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// objectList reads a list of JSON objects (Access rules, tunnel ingress
// rules), given as an array or as a JSON string holding one, since tool
// parameters are described in text and agents often send JSON strings.
// Missing keys and blank strings return nil.
func objectList(args map[string]any, key string) ([]map[string]any, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	if text, isString := value.(string); isString {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		var parsed []any
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return nil, fmt.Errorf("parameter %q: expected a JSON array of objects: %w", key, err)
		}
		value = parsed
	}
	switch typed := value.(type) {
	case []map[string]any:
		return typed, nil
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for i, item := range typed {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("parameter %q: element %d is %T, not an object", key, i, item)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("parameter %q: expected a list of objects, got %T", key, value)
	}
}
