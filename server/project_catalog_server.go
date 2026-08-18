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

// ProjectDeleteGuard rejects Project deletion while retained WorkSessions reference it.
type ProjectDeleteGuard interface {
	AssertProjectDeletable(ctx context.Context, projectID string) error
}

// ProjectCatalogServer exposes the dedicated /project-catalog/mcp surface.
type ProjectCatalogServer struct {
	catalog    project.Catalog
	writer     project.CatalogWriter
	compat     project.CompatibilityWriter
	valid      project.CatalogValidator
	cfg        projectCatalogRuntime
	mcp        *mcpsdk.Server
	standalone *mcpsdk.Server
	workGuard  ProjectDeleteGuard
}

// SetWorkGuard attaches referential integrity checks for Project deletion.
func (s *ProjectCatalogServer) SetWorkGuard(g ProjectDeleteGuard) {
	s.workGuard = g
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
	s.mcp.AddResource(&mcpsdk.Resource{
		URI:      catalogResourceURI,
		Name:     "catalog",
		MIMEType: "application/json",
	}, s.handleReadResource)
	for _, p := range page.Projects {
		uri := projectResourceURI(p.ProjectID)
		s.mcp.AddResource(&mcpsdk.Resource{
			URI:      uri,
			Name:     string(p.ProjectID),
			MIMEType: "application/json",
		}, s.handleReadResource)
	}
	for _, p := range page.InvalidProjects {
		uri := diagnosticsResourceURI(p.ProjectID)
		s.mcp.AddResource(&mcpsdk.Resource{
			URI:      uri,
			Name:     string(p.ProjectID) + "-diagnostics",
			MIMEType: "application/json",
		}, s.handleReadResource)
	}
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
		URITemplate: "project://registry/projects/{projectId}",
		Name:        "project",
		Description: "Project definition envelope",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/definition",
		Name:        "definition",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/diagnostics",
		Name:        "diagnostics",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{projectId}/revisions/{+revision}",
		Name:        "revision",
		MIMEType:    "application/json",
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
	case "project", "definition":
		return s.readProjectEnvelope(ctx, req.Params.URI, parsed.projectID)
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
	default:
		return nil, resourceNotFound(req.Params.URI)
	}
}

func (s *ProjectCatalogServer) readProjectEnvelope(ctx context.Context, uri string, id project.ProjectID) (*mcpsdk.ReadResourceResult, error) {
	snap, err := s.catalog.Get(ctx, id)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	body, err := json.Marshal(map[string]any{
		"projectId":      snap.ProjectID,
		"revision":       snap.Revision,
		"sourceRevision": snap.SourceRevision,
		"definition":     snap.Definition,
		"summary":        summaryFromSnap(snap),
		"sources":        nonNil(snap.Sources),
		"diagnostics":    nonNil(snap.Diagnostics),
	})
	if err != nil {
		return nil, err
	}
	return jsonResource(uri, body), nil
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

// AttachTo registers catalog tools and resources on an existing MCP server.
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
	if prev != nil && prev != mcpSrv {
		s.standalone = prev
	}
}

// Handler returns a stateless Streamable HTTP handler (tests / optional mount).
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
