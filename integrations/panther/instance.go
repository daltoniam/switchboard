package panther

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// instance is where one Panther deployment serves its APIs, derived from the
// single instance URL a user configures.
type instance struct {
	// url is the configured URL without query, fragment, or a trailing
	// /public/graphql or /mcp.
	url string
	// apiBase is the REST API root: https://api.<host> for SaaS and custom
	// domains, or https://<host>/v1 for Cloud Connected and self-hosted.
	apiBase string
	// graphqlURL is apiBase + /public/graphql.
	graphqlURL string
	// mcpOrigin hosts Panther Remote MCP at /mcp. It is always the api.
	// subdomain, which Remote MCP requires.
	mcpOrigin string
}

const (
	graphqlPath  = "/public/graphql"
	mcpPath      = "/mcp"
	apiSubdomain = "api."
)

// resolveInstance accepts the Panther Console URL (https://acme.runpanther.net),
// the API URL from Settings > Developer Tools > API Tokens
// (https://api.acme.runpanther.net, https://panther.acme.com/v1), or either
// with /public/graphql or /mcp appended. HTTPS is required except for
// loopback hosts, which only local test fixtures use.
func resolveInstance(raw string) (instance, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return instance{}, fmt.Errorf("panther: instance_url is required (your Panther Console or API URL, such as https://acme.runpanther.net)")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return instance{}, fmt.Errorf("panther: invalid instance_url %q: use a URL such as https://acme.runpanther.net", raw)
	}
	host := strings.ToLower(parsed.Hostname())
	loopback := isLoopback(host)
	switch {
	case parsed.Scheme == "https":
	case parsed.Scheme == "http" && loopback:
	default:
		return instance{}, fmt.Errorf("panther: instance_url must use https")
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	for _, suffix := range []string{graphqlPath, mcpPath} {
		path = strings.TrimSuffix(path, suffix)
	}
	path = strings.TrimRight(path, "/")

	hostPort := strings.ToLower(parsed.Host)
	apiHostPort := hostPort
	if !loopback && net.ParseIP(host) == nil && !strings.HasPrefix(host, apiSubdomain) {
		apiHostPort = apiSubdomain + hostPort
	}
	origin := parsed.Scheme + "://"
	apiBase := origin + apiHostPort
	if path != "" {
		// A path means the API URL of a Cloud Connected or self-hosted
		// deployment, served on the console host itself.
		apiBase = origin + hostPort + path
	}
	return instance{
		url:        origin + hostPort + path,
		apiBase:    apiBase,
		graphqlURL: apiBase + graphqlPath,
		mcpOrigin:  origin + apiHostPort,
	}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// SignInTarget returns the Remote MCP origin used for OAuth discovery and the
// MCP resource the login is bound to, for a configured instance URL.
func SignInTarget(instanceURL string) (origin, resource string, err error) {
	inst, err := resolveInstance(instanceURL)
	if err != nil {
		return "", "", err
	}
	return inst.mcpOrigin, inst.mcpOrigin + mcpPath, nil
}

// NormalizeInstanceURL validates an instance URL and returns its canonical
// form, so the setup page saves what the adapter will accept.
func NormalizeInstanceURL(raw string) (string, error) {
	inst, err := resolveInstance(raw)
	if err != nil {
		return "", err
	}
	return inst.url, nil
}

// APIBase returns the REST API root (GraphQL is at APIBase/public/graphql)
// for a configured instance URL.
func APIBase(instanceURL string) (string, error) {
	inst, err := resolveInstance(instanceURL)
	if err != nil {
		return "", err
	}
	return inst.apiBase, nil
}
