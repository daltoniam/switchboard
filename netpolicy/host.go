// Package netpolicy centralizes outbound URL host checks for user-supplied
// URLs (spec import endpoints, hosted integrations, web fetch, etc.).
package netpolicy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// blockedHostnames are well-known cloud metadata endpoints that must not be
// reachable via user-controlled URLs.
var blockedHostnames = map[string]bool{
	"metadata.google.internal": true,
	"metadata.goog":            true,
}

// blockedCIDRs covers IPv4 ranges that must not be dialed even when they are
// not classified as loopback/private by net.IP helpers (e.g. 0.0.0.0/8).
var blockedCIDRs = func() []*net.IPNet {
	_, n0, _ := net.ParseCIDR("0.0.0.0/8")
	_, n1, _ := net.ParseCIDR("100.64.0.0/10")
	return []*net.IPNet{n0, n1}
}()

// errDisallowedHost is returned when an outbound URL targets a forbidden host or address.
var errDisallowedHost = fmt.Errorf("requests to private, loopback, link-local, or metadata addresses are not allowed")

// HostDisallowed reports whether host must not be used for a user-supplied
// outbound HTTP(S) URL (loopback, RFC1918/ULA, link-local, or cloud metadata).
// Hostname labels that are not IP literals return false here; callers must also
// invoke ResolveOutboundHost before connecting.
func HostDisallowed(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	h := host
	if strings.Contains(h, ":") {
		if name, _, err := net.SplitHostPort(h); err == nil {
			h = name
		}
	}
	h = strings.Trim(h, "[]")
	h = stripIPv6ZoneID(h)
	if strings.Contains(h, "%") {
		return true
	}
	lower := strings.ToLower(h)
	if lower == "localhost" {
		return true
	}
	if blockedHostnames[lower] {
		return true
	}
	ip := net.ParseIP(lower)
	if ip == nil {
		return blockedHostnameLabel(lower)
	}
	if ip.IsUnspecified() {
		return true
	}
	for _, n := range blockedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func blockedHostnameLabel(lower string) bool {
	return lower == "localhost" || blockedHostnames[lower]
}

func stripIPv6ZoneID(host string) string {
	if i := strings.LastIndex(host, "%"); i != -1 && strings.Contains(host, ":") {
		return host[:i]
	}
	return host
}

// ResolveOutboundHost resolves host when needed and enforces the outbound address policy.
func ResolveOutboundHost(ctx context.Context, host string) error {
	_, err := allowedIPAddrs(ctx, host)
	return err
}

func allowedIPAddrs(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}
	h := host
	if strings.Contains(h, ":") {
		if name, _, err := net.SplitHostPort(h); err == nil {
			h = name
		}
	}
	h = strings.Trim(h, "[]")
	h = stripIPv6ZoneID(h)
	if strings.Contains(h, "%") {
		return nil, errDisallowedHost
	}
	lower := strings.ToLower(h)
	if lower == "localhost" || blockedHostnames[lower] {
		return nil, errDisallowedHost
	}
	if ip := net.ParseIP(lower); ip != nil {
		if HostDisallowed(host) {
			return nil, errDisallowedHost
		}
		return []net.IP{ip}, nil
	}
	addrs, err := lookupIPAddr(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("host %q has no addresses", h)
	}
	var allowed []net.IP
	for _, a := range addrs {
		if HostDisallowed(a.IP.String()) {
			continue
		}
		allowed = append(allowed, a.IP)
	}
	if len(allowed) == 0 {
		return nil, errDisallowedHost
	}
	return allowed, nil
}

// ValidateUserHTTPURL checks that raw is an absolute http or https URL whose host
// is allowed for outbound requests from user-supplied configuration.
func ValidateUserHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("url must include a host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ResolveOutboundHost(ctx, u.Hostname())
}
