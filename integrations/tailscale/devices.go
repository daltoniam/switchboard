package tailscale

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func devicePath(id, suffix string) string {
	return "/device/" + url.PathEscape(id) + suffix
}

func listDevices(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fields := r.Str("fields")
	hostname := r.Str("hostname")
	osName := r.Str("os")
	tags := r.StrSlice("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if fields == "" {
		fields = "default"
	}
	vals := url.Values{}
	vals.Set("fields", fields)
	if hostname != "" {
		vals.Set("hostname", hostname)
	}
	if osName != "" {
		vals.Set("os", osName)
	}
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			vals.Add("tags", tag)
		}
	}
	return rawResult(t.doJSON(ctx, http.MethodGet, t.tailnetPath("/devices")+"?"+vals.Encode(), nil))
}

func getDevice(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	fields := r.Str("fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if fields == "" {
		fields = "all"
	}
	vals := url.Values{"fields": {fields}}
	return rawResult(t.doJSON(ctx, http.MethodGet, devicePath(id, "")+"?"+vals.Encode(), nil))
}

func authorizeDevice(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	authorized := r.Bool("authorized")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if v, ok := args["authorized"]; !ok || v == nil {
		authorized = true
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, devicePath(id, "/authorized"), map[string]any{"authorized": authorized}))
}

func deleteDevice(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceAction(ctx, t, args, http.MethodDelete, "")
}

func expireDeviceKey(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceAction(ctx, t, args, http.MethodPost, "/expire")
}

func getDeviceRoutes(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceAction(ctx, t, args, http.MethodGet, "/routes")
}

func getDevicePostureAttributes(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceAction(ctx, t, args, http.MethodGet, "/attributes")
}

// deviceAction handles bodiless device endpoints that only need device_id.
func deviceAction(ctx context.Context, t *tailscale, args map[string]any, method, suffix string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, method, devicePath(id, suffix), nil))
}

func setDeviceName(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("name", name); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, devicePath(id, "/name"), map[string]any{"name": name}))
}

func setDeviceTags(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceStrList(ctx, t, args, "tags", "/tags", "tags")
}

func setDeviceRoutes(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return deviceStrList(ctx, t, args, "routes", "/routes", "routes")
}

// deviceStrList replaces a device list property (tags, routes). An explicit
// empty list is valid and clears the property, so presence is checked rather
// than length.
func deviceStrList(ctx context.Context, t *tailscale, args map[string]any, argName, suffix, bodyKey string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	values := r.StrSlice(argName)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requirePresent(args, argName); err != nil {
		return mcp.ErrResult(err)
	}
	if values == nil {
		values = []string{}
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, devicePath(id, suffix), map[string]any{bodyKey: values}))
}

func setDeviceKeyExpiry(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	disabled := r.Bool("key_expiry_disabled")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requirePresent(args, "key_expiry_disabled"); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, devicePath(id, "/key"), map[string]any{"keyExpiryDisabled": disabled}))
}

func setDeviceIP(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	ipv4 := r.Str("ipv4")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("device_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("ipv4", ipv4); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, devicePath(id, "/ip"), map[string]any{"ipv4": ipv4}))
}

func postureAttributePath(args map[string]any) (string, error) {
	r := mcp.NewArgs(args)
	id := r.Str("device_id")
	key := r.Str("attribute_key")
	if err := r.Err(); err != nil {
		return "", err
	}
	if err := requireStr("device_id", id); err != nil {
		return "", err
	}
	if err := requireStr("attribute_key", key); err != nil {
		return "", err
	}
	if !strings.HasPrefix(key, "custom:") {
		return "", fmt.Errorf("attribute_key must start with custom: (only custom posture attributes can be managed via the API)")
	}
	return devicePath(id, "/attributes/"+url.PathEscape(key)), nil
}

func setDevicePostureAttribute(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	path, err := postureAttributePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	expiry := r.Str("expiry")
	comment := r.Str("comment")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requirePresent(args, "value"); err != nil {
		return mcp.ErrResult(err)
	}
	value := args["value"]
	switch value.(type) {
	case string, bool, float64, float32, int, int32, int64:
	default:
		return mcp.ErrResult(fmt.Errorf("value must be a string, number, or boolean, got %T", value))
	}
	body := map[string]any{"value": value}
	if expiry != "" {
		body["expiry"] = expiry
	}
	if comment != "" {
		body["comment"] = comment
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, path, body))
}

func deleteDevicePostureAttribute(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	path, err := postureAttributePath(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodDelete, path, nil))
}
