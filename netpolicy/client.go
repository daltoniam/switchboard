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
	// AuthHeader and AuthValue are attached only for requests to AuthOrigin.
	AuthHeader string
	AuthValue  string
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

type authRoundTripper struct {
	base   http.RoundTripper
	header string
	value  string
	origin *url.URL
}

func (t *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header = req.Header.Clone()
	if t.header != "" && t.value != "" {
		if t.origin == nil || sameOrigin(t.origin, req.URL) {
			req2.Header.Set(t.header, t.value)
		} else {
			req2.Header.Del(t.header)
		}
	}
	return t.base.RoundTrip(req2)
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
	if cfg.AuthHeader != "" && cfg.AuthValue != "" {
		rt = &authRoundTripper{base: rt, header: cfg.AuthHeader, value: cfg.AuthValue, origin: cfg.AuthOrigin}
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
	ips, err := allowedIPAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)
		conn, err := d.DialContext(ctx, network, target)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("host %q could not be dialed", host)
	}
	return nil, lastErr
}
