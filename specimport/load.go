package specimport

import (
	"context"
	"fmt"
	"os"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// Load turns a single mcp.SpecImportConfig into a configured, ready-to-register
// integration. It reads the spec (inline or from disk), parses it into the
// requested kind, builds the runtime integration, and applies credentials.
//
// The returned integration is fully configured — callers register it directly.
// Load is the seam the composition root uses to wire config-declared spec
// imports; it keeps all spec-loading concerns (file I/O, kind resolution,
// credential mapping) out of main.
func Load(cfg mcp.SpecImportConfig) (*Integration, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, fmt.Errorf("specimport: config entry is missing a name")
	}

	doc, err := readSpec(cfg)
	if err != nil {
		return nil, fmt.Errorf("specimport %q: %w", name, err)
	}

	kind, err := parseKind(cfg.Kind)
	if err != nil {
		return nil, fmt.Errorf("specimport %q: %w", name, err)
	}

	im, err := Parse(kind, name, doc, strings.TrimSpace(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("specimport %q: %w", name, err)
	}

	in := NewIntegration(im)
	creds := cfg.Credentials
	if creds == nil {
		creds = mcp.Credentials{}
	}
	if err := in.Configure(context.Background(), creds); err != nil {
		return nil, fmt.Errorf("specimport %q: %w", name, err)
	}
	return in, nil
}

// readSpec resolves the spec document from either the inline Spec field or the
// Path field. Exactly one must be set.
func readSpec(cfg mcp.SpecImportConfig) ([]byte, error) {
	inline := strings.TrimSpace(cfg.Spec)
	path := strings.TrimSpace(cfg.Path)
	switch {
	case inline != "" && path != "":
		return nil, fmt.Errorf("set either spec or path, not both")
	case inline != "":
		return []byte(cfg.Spec), nil
	case path != "":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read spec file: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("spec import needs a spec or path")
	}
}

// parseKind maps the config string to a SpecKind, tolerating surrounding
// whitespace and case differences.
func parseKind(s string) (SpecKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(KindOpenAPI):
		return KindOpenAPI, nil
	case string(KindGraphQL):
		return KindGraphQL, nil
	default:
		return "", fmt.Errorf("%w: %q (want %q or %q)", ErrUnknownKind, s, KindOpenAPI, KindGraphQL)
	}
}
