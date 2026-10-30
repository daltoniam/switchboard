package netpolicy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRedirects = 10

// UserHTTPClientConfig configures an outbound HTTP client for user-supplied URLs.
type UserHTTPClientConfig struct {
	Timeout      time.Duration
	AllowPrivate bool
	// ValidateURL checks each request URL (initial and redirect targets).
	ValidateURL func(raw string) error
	// AuthHeader, when non-empty, is stripped on cross-origin redirects.
	AuthHeader string
}

// NewUserHTTPClient returns an http.Client that enforces host policy at dial time
// (mitigating DNS rebinding), validates redirect destinations, and drops the
// configured auth header when a redirect changes origin.
func NewUserHTTPClient(cfg UserHTTPClientConfig) *http.Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialContext(ctx, network, address, cfg.AllowPrivate)
		},
	}
	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			if cfg.ValidateURL != nil {
				if err := cfg.ValidateURL(req.URL.String()); err != nil {
					return err
				}
			}
			if cfg.AuthHeader != "" && len(via) > 0 {
				prev := via[len(via)-1].URL
				if !sameOrigin(prev, req.URL) {
					req.Header.Del(cfg.AuthHeader)
				}
			}
			return nil
		},
	}
	return client
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func sameHost(a, b string) bool {
	return strings.EqualFold(stripPort(a), stripPort(b))
}

func stripPort(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(host, "[]")
}

func dialContext(ctx context.Context, network, address string, allowPrivate bool) (net.Conn, error) {
	d := &net.Dialer{Timeout: 30 * time.Second}
	if allowPrivate {
		return d.DialContext(ctx, network, address)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if HostDisallowed(host) {
			return nil, fmt.Errorf("connection to disallowed address")
		}
		return d.DialContext(ctx, network, address)
	}
	if HostDisallowed(host) {
		return nil, fmt.Errorf("connection to disallowed host")
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses for %q", host)
	}
	var lastErr error
	for _, ipa := range addrs {
		if HostDisallowed(ipa.IP.String()) {
			lastErr = fmt.Errorf("host %q resolves to disallowed address", host)
			continue
		}
		target := net.JoinHostPort(ipa.IP.String(), port)
		conn, err := d.DialContext(ctx, network, target)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("host %q resolves only to disallowed addresses", host)
	}
	return nil, lastErr
}
