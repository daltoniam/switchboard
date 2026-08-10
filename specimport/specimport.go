// Package specimport turns a user-supplied API specification (OpenAPI 3.x
// or a GraphQL introspection result) into a runtime mcp.Integration.
//
// It is the "bring your own spec" adapter: instead of waiting on a
// hand-written Go integration, a customer can point Switchboard at an
// OpenAPI document or GraphQL endpoint and immediately get a set of MCP
// tools backed by that API. The produced integration follows the same
// rules as every built-in one:
//
//   - Tools are discovered lazily through the search/execute meta-tools.
//   - Credentials are injected host-side at request time and never appear
//     in tool definitions or results.
//   - Operation semantics are preserved: read-only operations (HTTP GET /
//     HEAD, GraphQL queries) are marked safe; writes (POST/PUT/PATCH/DELETE,
//     GraphQL mutations) are marked destructive so the policy layer can
//     gate them.
//
// The package depends only on the standard library plus the public
// switchboard module, so it adds no new third-party dependencies. It lives
// in its own subpackage because it wraps an external concern (arbitrary API
// specs), consistent with the repo's Standard Package Layout.
package specimport

import (
	"errors"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// SpecKind identifies the format of an imported specification.
type SpecKind string

const (
	// KindOpenAPI is an OpenAPI 3.x document (JSON).
	KindOpenAPI SpecKind = "openapi"
	// KindGraphQL is a GraphQL introspection result (JSON).
	KindGraphQL SpecKind = "graphql"
)

// Common errors returned by parsing. Callers (the web/REST import handler)
// surface these to the user verbatim, so they are written to be readable.
var (
	// ErrEmptySpec is returned when the supplied document is empty.
	ErrEmptySpec = errors.New("specimport: spec document is empty")
	// ErrNoOperations is returned when a spec parses but exposes no callable
	// operations — almost always a sign the user pasted the wrong document.
	ErrNoOperations = errors.New("specimport: spec contains no callable operations")
	// ErrUnknownKind is returned for an unrecognized SpecKind.
	ErrUnknownKind = errors.New("specimport: unknown spec kind")
)

// httpMethod classifies an operation's effect so the policy layer can gate
// writes. Read operations may auto-run; writes should require approval.
type effect string

const (
	effectRead  effect = "read"
	effectWrite effect = "write"
)

// operation is the protocol-neutral representation of a single callable
// endpoint, produced by the OpenAPI or GraphQL parser and consumed by the
// executor. Keeping this intermediate form means the execution path does
// not care which spec format produced it.
type operation struct {
	// tool is the MCP-facing definition (name, description, params).
	tool mcp.ToolDefinition
	// effect drives the destructive-hint / approval semantics.
	effect effect

	// --- transport details, populated per spec kind ---

	// httpMethod and pathTemplate are set for OpenAPI operations.
	// pathTemplate may contain {placeholders} substituted from args.
	httpMethod   string
	pathTemplate string
	// pathParams / queryParams name the args that map to URL path
	// segments and query string respectively. Anything not in either
	// (for write methods) is sent as a JSON body.
	pathParams  []string
	queryParams []string

	// gqlDocument is the GraphQL query/mutation string for GraphQL ops.
	// gqlVariables names the args forwarded as GraphQL variables.
	gqlDocument  string
	gqlVariables []string
}

// Imported is the result of parsing a spec: the protocol-neutral operation
// set plus the metadata needed to build a runtime integration. It is
// serializable so the import can be persisted and rehydrated without
// re-parsing the original document.
type Imported struct {
	// Name is the integration identifier (lowercased, sanitized). Tool
	// names are prefixed with it, matching built-in integrations.
	Name string
	// Kind records which parser produced this import.
	Kind SpecKind
	// BaseURL is the API origin for OpenAPI imports (scheme://host/basePath)
	// or the GraphQL endpoint URL for GraphQL imports.
	BaseURL string

	operations []operation
}

// Tools returns the MCP tool definitions for every imported operation.
func (im *Imported) Tools() []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, 0, len(im.operations))
	for i := range im.operations {
		out = append(out, im.operations[i].tool)
	}
	return out
}

// sanitizeName lowercases and reduces an arbitrary string to the
// [a-z0-9_] set used for integration and tool identifiers. Runs of invalid
// characters collapse to a single underscore; leading/trailing underscores
// are trimmed. Empty input yields "spec" so we never emit a blank name.
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "spec"
	}
	return out
}

// SanitizedName returns the registry identifier a spec import will use for a
// given raw name — the same transform applied to Imported.Name. Callers that
// reconcile config against the registry (live reload, the web UI) use it to
// match a SpecImportConfig to its registered integration without parsing the
// spec.
func SanitizedName(s string) string { return sanitizeName(s) }

// Parse dispatches to the OpenAPI or GraphQL parser based on kind. name is
// the caller-chosen integration name (sanitized here). doc is the raw spec
// bytes. For GraphQL, endpoint is the URL the introspection describes and is
// required; for OpenAPI it is an optional override for the server URL.
func Parse(kind SpecKind, name string, doc []byte, endpoint string) (*Imported, error) {
	if len(strings.TrimSpace(string(doc))) == 0 {
		return nil, ErrEmptySpec
	}
	switch kind {
	case KindOpenAPI:
		return parseOpenAPI(sanitizeName(name), doc, endpoint)
	case KindGraphQL:
		return parseGraphQL(sanitizeName(name), doc, endpoint)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
}
