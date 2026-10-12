// Package netpolicy centralizes outbound URL host checks for user-supplied
// URLs (spec import endpoints, hosted integrations, web fetch, etc.).
package netpolicy

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockedHostnames are well-known cloud metadata endpoints that must not be
// reachable via user-controlled URLs.
var blockedHostnames = map[string]bool{
	"metadata.google.internal": true,
	"metadata.goog":            true,
}

// HostDisallowed reports whether host must not be used for a user-supplied
// outbound HTTP(S) URL (loopback, RFC1918/ULA, link-local, or cloud metadata).
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
	lower := strings.ToLower(h)
	if lower == "localhost" {
		return true
	}
	if blockedHostnames[lower] {
		return true
	}
	ip := net.ParseIP(lower)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
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
	if HostDisallowed(u.Hostname()) {
		return fmt.Errorf("requests to private, loopback, link-local, or metadata addresses are not allowed")
	}
	return nil
}
