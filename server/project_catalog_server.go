package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
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
		s.mcp.AddResource(&mcpsdk.Resource{
			URI:      projectResourcesURI(p.ProjectID),
			Name:     string(p.ProjectID) + "-resources",
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

	// Minimal resources-based surface (update.md).
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{project}",
		Name:        "project",
		Description: "Project envelope: definition + summary",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{project}/resources",
		Name:        "project-resources",
		Description: "List of resources declared on a project",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{project}/resources/{resource}",
		Name:        "project-resource",
		Description: "Single resource metadata",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{project}/resources/{resource}/content",
		Name:        "project-resource-content",
		Description: "Resource content (file text, files manifest, or repo metadata)",
		MIMEType:    "application/json",
	}, s.handleReadResource)
	s.mcp.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "project://registry/projects/{project}/resources/{resource}/files/{+path}",
		Name:        "project-resource-file",
		Description: "Progressive file under a files resource",
		MIMEType:    "text/plain",
	}, s.handleReadResource)

	// Compatibility aliases retained for existing clients/tests.
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
	case "resources":
		return s.readResourcesList(ctx, req.Params.URI, parsed.projectID)
	case "resource":
		return s.readResourceMeta(ctx, req.Params.URI, parsed.projectID, parsed.resourceID)
	case "resource-content":
		return s.readResourceContent(ctx, req.Params.URI, parsed.projectID, parsed.resourceID)
	case "resource-file":
		return s.readResourceFile(ctx, req.Params.URI, parsed.projectID, parsed.resourceID, parsed.path)
	case "context":
		return s.readContextResource(ctx, req.Params.URI, parsed)
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

func (s *ProjectCatalogServer) readResourcesList(ctx context.Context, uri string, id project.ProjectID) (*mcpsdk.ReadResourceResult, error) {
	snap, err := s.catalog.Get(ctx, id)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	type item struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		URI         string `json:"uri"`
		ContentURI  string `json:"contentUri"`
		Description string `json:"description,omitempty"`
	}
	ids := make([]string, 0, len(snap.Definition.Resources))
	for rid := range snap.Definition.Resources {
		ids = append(ids, rid)
	}
	sort.Strings(ids)
	out := make([]item, 0, len(ids))
	for _, rid := range ids {
		res := snap.Definition.Resources[rid]
		out = append(out, item{
			ID:          rid,
			Type:        res.Type,
			URI:         projectResourceMetaURI(id, rid),
			ContentURI:  projectResourceContentURI(id, rid),
			Description: res.Description,
		})
	}
	body, err := json.Marshal(map[string]any{
		"projectId": id,
		"resources": out,
	})
	if err != nil {
		return nil, err
	}
	return jsonResource(uri, body), nil
}

func (s *ProjectCatalogServer) readResourceMeta(ctx context.Context, uri string, id project.ProjectID, resourceID string) (*mcpsdk.ReadResourceResult, error) {
	snap, err := s.catalog.Get(ctx, id)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	res, ok := snap.Definition.Resources[resourceID]
	if !ok {
		return nil, resourceNotFound(uri)
	}
	meta := resourceMetadataJSON(id, resourceID, &snap.Definition, res)
	body, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return jsonResource(uri, body), nil
}

func resourceMetadataJSON(projectID project.ProjectID, resourceID string, def *project.Definition, res project.Resource) map[string]any {
	meta := map[string]any{
		"id":   resourceID,
		"type": res.Type,
	}
	if res.Description != "" {
		meta["description"] = res.Description
	}
	if res.Optional {
		meta["optional"] = true
	}
	switch res.Type {
	case project.ResourceTypeRepo:
		meta["path"] = def.ResolveRepoPath(res)
		if res.Branch != "" {
			meta["branch"] = res.Branch
		}
	case project.ResourceTypeFile:
		if res.Repo != "" {
			meta["repo"] = res.Repo
		}
		meta["path"] = res.Path
	case project.ResourceTypeFiles:
		if res.Repo != "" {
			meta["repo"] = res.Repo
		}
		if res.Root != "" {
			meta["root"] = res.Root
		}
		meta["include"] = nonNil(res.Include)
		if len(res.Exclude) > 0 {
			meta["exclude"] = res.Exclude
		}
	}
	meta["contentUri"] = projectResourceContentURI(projectID, resourceID)
	return meta
}

func (s *ProjectCatalogServer) readResourceContent(ctx context.Context, uri string, id project.ProjectID, resourceID string) (*mcpsdk.ReadResourceResult, error) {
	snap, err := s.catalog.Get(ctx, id)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	res, ok := snap.Definition.Resources[resourceID]
	if !ok {
		return nil, resourceNotFound(uri)
	}
	switch res.Type {
	case project.ResourceTypeRepo:
		// Repo /content returns the same metadata JSON as the resource itself.
		meta := resourceMetadataJSON(id, resourceID, &snap.Definition, res)
		body, err := json.Marshal(meta)
		if err != nil {
			return nil, err
		}
		return jsonResource(uri, body), nil
	case project.ResourceTypeFile:
		content, mime, err := project.ReadResourceContent(&snap.Definition, resourceID)
		if err != nil {
			return nil, resourceNotFound(uri)
		}
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      uri,
				MIMEType: mime,
				Text:     content,
			}},
		}, nil
	case project.ResourceTypeFiles:
		files, err := project.ExpandFilesResource(&snap.Definition, resourceID, res, "")
		if err != nil {
			return nil, resourceNotFound(uri)
		}
		entries := make([]project.ResourceFileEntry, 0, len(files))
		for _, f := range files {
			entries = append(entries, project.ResourceFileEntry{
				Path:     f.Path,
				URI:      projectResourceFileURI(id, resourceID, f.Path),
				MIMEType: f.MIMEType,
				Size:     f.Size,
			})
		}
		manifest := project.FilesManifest{Resource: resourceID, Files: entries}
		if manifest.Files == nil {
			manifest.Files = []project.ResourceFileEntry{}
		}
		body, err := json.Marshal(manifest)
		if err != nil {
			return nil, err
		}
		return jsonResource(uri, body), nil
	default:
		return nil, resourceNotFound(uri)
	}
}

func (s *ProjectCatalogServer) readResourceFile(ctx context.Context, uri string, id project.ProjectID, resourceID, filePath string) (*mcpsdk.ReadResourceResult, error) {
	snap, err := s.catalog.Get(ctx, id)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	res, ok := snap.Definition.Resources[resourceID]
	if !ok || res.Type != project.ResourceTypeFiles {
		return nil, resourceNotFound(uri)
	}
	files, err := project.ExpandFilesResource(&snap.Definition, resourceID, res, "")
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	want := filepath.ToSlash(filePath)
	var match *project.ResourceFileEntry
	for i := range files {
		if filepath.ToSlash(files[i].Path) == want {
			match = &files[i]
			break
		}
	}
	if match == nil {
		return nil, resourceNotFound(uri)
	}
	content, err := project.ReadContextFile(&snap.Definition, catalogConfigDir(s.catalog), match.Path)
	if err != nil {
		return nil, resourceNotFound(uri)
	}
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      uri,
			MIMEType: project.GuessMIME(match.Path),
			Text:     content,
		}},
	}, nil
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
