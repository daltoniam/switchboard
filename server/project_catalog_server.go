package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	catalogListTTLMs     = 10_000
	catalogRevisionTTLMs = 3_600_000
)

// ProjectCatalogServer exposes the dedicated /project-catalog/mcp surface.
type ProjectCatalogServer struct {
	catalog    project.Catalog
	writer     project.CatalogWriter
	compat     project.CompatibilityWriter
	valid      project.CatalogValidator
	cfg        projectCatalogRuntime
	mcp        *mcpsdk.Server
	standalone *mcpsdk.Server
}

type projectCatalogRuntime struct {
	writesEnabled bool
}

func newProjectCatalogRuntime(cfg ProjectCatalogOptions) projectCatalogRuntime {
	return projectCatalogRuntime{writesEnabled: cfg.WritesEnabled}
}

// ProjectCatalogOptions is the runtime view of mcp.ProjectCatalogConfig.
type ProjectCatalogOptions struct {
	WritesEnabled bool
}

// NewProjectCatalogServer constructs a catalog surface that can AttachTo the
// main Switchboard MCP server or serve standalone (tests).
func NewProjectCatalogServer(cat project.Catalog, writer project.CatalogWriter, compat project.CompatibilityWriter, valid project.CatalogValidator, cfg ProjectCatalogOptions) *ProjectCatalogServer {
	s := &ProjectCatalogServer{
		catalog: cat,
		writer:  writer,
		compat:  compat,
		valid:   valid,
		cfg:     newProjectCatalogRuntime(cfg),
	}
	s.mcp = mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "switchboard-project-catalog", Version: "1"},
		&mcpsdk.ServerOptions{
			Instructions: "Project Catalog is a declarative registry of local project definitions. It is not a work session, workspace, or AgentRun runtime.",
			Capabilities: &mcpsdk.ServerCapabilities{
				Tools: &mcpsdk.ToolCapabilities{ListChanged: false},
				Resources: &mcpsdk.ResourceCapabilities{
					ListChanged: true,
					Subscribe:   true,
				},
			},
			SubscribeHandler:   func(context.Context, *mcpsdk.SubscribeRequest) error { return nil },
			UnsubscribeHandler: func(context.Context, *mcpsdk.UnsubscribeRequest) error { return nil },
		},
	)
	s.standalone = s.mcp
	s.registerResources()
	s.registerTools()
	s.mcp.AddReceivingMiddleware(s.reconcileMiddleware)
	s.mcp.AddReceivingMiddleware(s.cacheMiddleware)
	return s
}

func (s *ProjectCatalogServer) reconcileMiddleware(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		if method == "resources/list" {
			s.reconcileResources(ctx)
		}
		return next(ctx, method, req)
	}
}

func (s *ProjectCatalogServer) reconcileResources(ctx context.Context) {
	page, err := s.catalog.List(ctx, "")
	if err != nil {
		return
	}
	want := map[string]struct{}{catalogResourceURI: {}}
	s.mcp.AddResource(&mcpsdk.Resource{
		URI:      catalogResourceURI,
		Name:     "catalog",
		MIMEType: "application/json",
	}, s.handleReadResource)
	for _, p := range page.Projects {
		uri := definitionResourceURI(p.ProjectID)
		want[uri] = struct{}{}
		s.mcp.AddResource(&mcpsdk.Resource{
			URI:      uri,
			Name:     string(p.ProjectID),
			MIMEType: "application/json",
		}, s.handleReadResource)
	}
	for _, p := range page.InvalidProjects {
		uri := diagnosticsResourceURI(p.ProjectID)
		want[uri] = struct{}{}
		s.mcp.AddResource(&mcpsdk.Resource{
			URI:      uri,
			Name:     string(p.ProjectID) + "-diagnostics",
			MIMEType: "application/json",
		}, s.handleReadResource)
	}
	_ = want
}

func (s *ProjectCatalogServer) cacheMiddleware(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		res, err := next(ctx, method, req)
		if err != nil || res == nil {
			return res, err
		}
		switch method {
		case "resources/list", "resources/templates/list", "server/discover":
			setCatalogCache(res, catalogListTTLMs)
		case "resources/read":
			ttl := catalogListTTLMs
			if readReq, ok := req.(*mcpsdk.ReadResourceRequest); ok && readReq.Params != nil && strings.Contains(readReq.Params.URI, "/revisions/") {
				ttl = catalogRevisionTTLMs
			}
			setCatalogCache(res, ttl)
		}
		return res, nil
	}
}

func setCatalogCache(res mcpsdk.Result, ttl int) {
	switch v := res.(type) {
	case *mcpsdk.ListResourcesResult:
		v.CacheScope = "private"
		v.TTLMs = ttl
	case *mcpsdk.ListResourceTemplatesResult:
		v.CacheScope = "private"
		v.TTLMs = ttl
	case *mcpsdk.ReadResourceResult:
		v.CacheScope = "private"
		v.TTLMs = ttl
	case *mcpsdk.DiscoverResult:
		v.CacheScope = "private"
		v.TTLMs = ttl
	}
}

func (s *ProjectCatalogServer) registerResources() {
	s.mcp.AddResource(&mcpsdk.Resource{
		URI:         catalogResourceURI,
		Name:        "catalog",
		Description: "Current Project Catalog summaries",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/definition",
		Name:        "definition",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/diagnostics{?rootUri}",
		Name:        "diagnostics",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/revisions/{+revision}",
		Name:        "revision",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/context{?rootUri}",
		Name:        "context-manifest",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/context/{+path}{?rootUri}",
		Name:        "context-file",
		MIMEType:    "text/plain",
	}, s.handleReadResource)
}

func (s *ProjectCatalogServer) handleReadResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	parsed, err := parseCatalogURI(req.Params.URI)
	if err != nil {
		return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
	}
	switch parsed.kind {
	case "catalog":
		page, err := s.catalog.List(ctx, "")
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(map[string]any{
			"projects":        nonNil(page.Projects),
			"invalidProjects": nonNil(page.InvalidProjects),
		})
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, body), nil
	case "definition":
		snap, err := s.catalog.Get(ctx, parsed.projectID)
		if err != nil {
			return nil, resourceNotFound(req.Params.URI)
		}
		body, err := json.Marshal(map[string]any{
			"projectId":      snap.ProjectID,
			"revision":       snap.Revision,
			"sourceRevision": snap.SourceRevision,
			"definition":     snap.Definition,
			"sources":        nonNil(snap.Sources),
			"diagnostics":    nonNil(snap.Diagnostics),
		})
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, body), nil
	case "diagnostics":
		var root *url.URL
		if parsed.rootURI != "" {
			u, err := url.Parse(parsed.rootURI)
			if err != nil {
				return nil, resourceNotFound(req.Params.URI)
			}
			root = u
		}
		env, err := s.catalog.Diagnostics(ctx, parsed.projectID, root)
		if err != nil {
			return nil, resourceNotFound(req.Params.URI)
		}
		if env.Diagnostics == nil {
			env.Diagnostics = []project.Diagnostic{}
		}
		body, err := json.Marshal(env)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, body), nil
	case "revision":
		rev, err := s.catalog.GetRevision(ctx, parsed.projectID, parsed.revision)
		if err != nil {
			return nil, resourceNotFound(req.Params.URI)
		}
		body, err := json.Marshal(map[string]any{
			"projectId":  rev.ProjectID,
			"revision":   rev.Revision,
			"definition": rev.Definition,
		})
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, body), nil
	case "context":
		return s.readContextResource(ctx, req.Params.URI, parsed)
	default:
		return nil, resourceNotFound(req.Params.URI)
	}
}

func (s *ProjectCatalogServer) readContextResource(ctx context.Context, uri string, parsed parsedCatalogURI) (*mcpsdk.ReadResourceResult, error) {
	req := project.ResolveRequest{ProjectID: parsed.projectID, RootURI: parsed.rootURI}
	snap, err := s.catalog.Resolve(ctx, req)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	root := snap.Definition.ResolvedRepo()
	if parsed.rootURI != "" {
		if u, err := url.Parse(parsed.rootURI); err == nil {
			root = u.Path
		}
	}
	if parsed.path == "" {
		entries := project.AssembleManifestAtRoot(&snap.Definition, catalogConfigDir(s.catalog), root)
		type entry struct {
			Path     string `json:"path"`
			URI      string `json:"uri"`
			Source   string `json:"source"`
			MIMEType string `json:"mimeType"`
			Size     int    `json:"sizeBytes"`
		}
		out := make([]entry, 0, len(entries))
		for _, e := range entries {
			src := e.Source
			if src == "repo" {
				src = "repository"
			}
			if src == "store" {
				src = "user"
			}
			out = append(out, entry{
				Path:     e.Path,
				URI:      contextFileURI(snap.ProjectID, e.Path, snap.RootURI),
				Source:   src,
				MIMEType: e.MIMEType,
				Size:     e.Size,
			})
		}
		body, err := json.Marshal(map[string]any{
			"projectId": snap.ProjectID,
			"revision":  snap.Revision,
			"rootUri":   omitEmpty(snap.RootURI),
			"entries":   out,
		})
		if err != nil {
			return nil, err
		}
		return jsonResource(uri, body), nil
	}
	content, err := project.ReadContextFile(&snap.Definition, catalogConfigDir(s.catalog), parsed.path)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      uri,
			MIMEType: project.GuessMIME(parsed.path),
			Text:     content,
		}},
	}, nil
}

func catalogConfigDir(cat project.Catalog) string {
	if s, ok := cat.(interface{ ConfigDir() string }); ok {
		return s.ConfigDir()
	}
	return ""
}

func jsonResource(uri string, body []byte) *mcpsdk.ReadResourceResult {
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(body),
		}},
	}
}

func resourceNotFound(uri string) error {
	return mcpsdk.ResourceNotFoundError(uri)
}

func nonNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func omitEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// AttachTo registers catalog tools and resources on an existing MCP server
// (the main Switchboard /mcp endpoint).
func (s *ProjectCatalogServer) AttachTo(mcpSrv *mcpsdk.Server) {
	if s == nil || mcpSrv == nil {
		return
	}
	prev := s.mcp
	s.mcp = mcpSrv
	s.registerResources()
	s.registerTools()
	mcpSrv.AddReceivingMiddleware(s.reconcileMiddleware)
	mcpSrv.AddReceivingMiddleware(s.cacheMiddleware)
	// Keep standalone server for tests that still construct a dedicated handler.
	if prev != nil && prev != mcpSrv {
		s.standalone = prev
	}
}

// Handler returns a stateless Streamable HTTP handler for tests or optional
// dedicated mounting. No bearer token is required.
func (s *ProjectCatalogServer) Handler() http.Handler {
	target := s.mcp
	if s.standalone != nil {
		target = s.standalone
	}
	return mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return target
	}, &mcpsdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default(),
	})
}
