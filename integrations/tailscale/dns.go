package tailscale

import (
	"context"
	"fmt"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func getDNS(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	return rawResult(t.doJSON(ctx, http.MethodGet, t.tailnetPath("/dns/configuration"), nil))
}

func setDNSNameservers(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return setDNSList(ctx, t, args, "nameservers", "/dns/nameservers", "dns")
}

func setDNSSearchPaths(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return setDNSList(ctx, t, args, "search_paths", "/dns/searchpaths", "searchPaths")
}

// setDNSList replaces a tailnet-wide DNS list. An explicit empty list clears it.
func setDNSList(ctx context.Context, t *tailscale, args map[string]any, argName, suffix, bodyKey string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	values := r.StrSlice(argName)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requirePresent(args, argName); err != nil {
		return mcp.ErrResult(err)
	}
	if values == nil {
		values = []string{}
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, t.tailnetPath(suffix), map[string]any{bodyKey: values}))
}

func setDNSPreferences(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	magicDNS := r.Bool("magic_dns")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requirePresent(args, "magic_dns"); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, t.tailnetPath("/dns/preferences"), map[string]any{"magicDNS": magicDNS}))
}

func updateSplitDNS(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	splitDNS := r.Map("split_dns")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if len(splitDNS) == 0 {
		return mcp.ErrResult(fmt.Errorf("split_dns is required: map of domain to nameserver list (null removes a domain)"))
	}
	for domain, v := range splitDNS {
		switch v.(type) {
		case nil, []any, []string:
		default:
			return mcp.ErrResult(fmt.Errorf("split_dns[%q] must be a list of nameservers or null, got %T", domain, v))
		}
	}
	return rawResult(t.doJSON(ctx, http.MethodPatch, t.tailnetPath("/dns/split-dns"), splitDNS))
}
