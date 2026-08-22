package server

import "net/http"

// HTTPMuxConfig is the production HTTP route table used by cmd/server and
// public-boundary tests. Tests must mount this helper rather than inventing
// a private mux so /mcp, /mcp/{project}, and later /project-catalog/mcp stay
// on one composition path.
type HTTPMuxConfig struct {
	// MCP is the global Switchboard Streamable HTTP handler mounted at /mcp.
	// Production must pass Server.StatelessHandler(), not Server.Handler().
	MCP http.Handler
	// Project is the project-scoped compatibility handler at /mcp/{project}.
	Project http.Handler
	// ProjectCatalog is the dedicated Project Catalog handler at
	// /project-catalog/mcp. Leave nil to omit the route (default-disabled).
	ProjectCatalog http.Handler
	// Web is the optional dashboard handler mounted at "/".
	Web http.Handler
}

// BuildHTTPMux mounts the production HTTP routes. Catalog is registered
// before /mcp/{project} so a literal /project-catalog/mcp never collides
// with a project named "project-catalog".
func BuildHTTPMux(cfg HTTPMuxConfig) *http.ServeMux {
	mux := http.NewServeMux()
	if cfg.ProjectCatalog != nil {
		mux.Handle("/project-catalog/mcp", cfg.ProjectCatalog)
	}
	if cfg.MCP != nil {
		mux.Handle("/mcp", cfg.MCP)
	}
	if cfg.Project != nil {
		mux.Handle("/mcp/{project}", cfg.Project)
	}
	if cfg.Web != nil {
		mux.Handle("/", cfg.Web)
	}
	return mux
}
