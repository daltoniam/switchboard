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

// lookupIPAddr resolves hostnames for outbound policy checks. Tests may replace it.
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// UserHTTPClientConfig configures an outbound HTTP client for user-supplied URLs.
type UserHTTPClientConfig struct {
	Timeout      time.Duration
	AllowPrivate bool
	// ValidateURL checks each request URL (initial and redirect targets).
	ValidateURL func(raw string) error
	// AuthHeader, when non-empty, is stripped on requests whose origin differs from AuthOrigin.
	AuthHeader string
	AuthOrigin *url.URL
	// DisableRedirects rejects redirect responses instead of following them.
	DisableRedirects bool
}

type validatingRoundTripper struct {
	base     http.RoundTripper
	validate func(raw string) error
}

func (t *validatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.validate != nil {
		if err := t.validate(req.URL.String()); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(req)
}

type authStripRoundTripper struct {
	base   http.RoundTripper
	header string
	origin *url.URL
}

func (t *authStripRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.header != "" && t.origin != nil && !sameOrigin(t.origin, req.URL) {
		req2 := req.Clone(req.Context())
		req2.Header = req.Header.Clone()
		req2.Header.Del(t.header)
		req = req2
	}
	return t.base.RoundTrip(req)
}

// NewUserHTTPClient returns an http.Client that enforces host policy at dial time
// (mitigating DNS rebinding), validates every request URL, and drops the
// configured auth header when a request leaves AuthOrigin.
func NewUserHTTPClient(cfg UserHTTPClientConfig) *http.Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	base := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialContext(ctx, network, address, cfg.AllowPrivate)
		},
	}
	rt := http.RoundTripper(base)
	rt = &validatingRoundTripper{base: rt, validate: cfg.ValidateURL}
	if cfg.AuthHeader != "" && cfg.AuthOrigin != nil {
		rt = &authStripRoundTripper{base: rt, header: cfg.AuthHeader, origin: cfg.AuthOrigin}
	}
	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if cfg.DisableRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			if cfg.ValidateURL != nil {
				if err := cfg.ValidateURL(req.URL.String()); err != nil {
					return err
				}
			}
			if cfg.AuthHeader != "" && len(via) > 0 {
				original := via[0].URL
				if !sameOrigin(original, req.URL) {
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
	addrs, err := lookupIPAddr(ctx, host)
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
