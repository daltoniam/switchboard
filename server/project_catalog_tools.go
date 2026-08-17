package server

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/daltoniam/switchboard/project"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolErrorBody struct {
	Error projectToolError `json:"error"`
}

type projectToolError struct {
	Code                   string               `json:"code"`
	Message                string               `json:"message"`
	ProjectID              project.ProjectID    `json:"projectId,omitempty"`
	ExpectedSourceRevision project.Revision     `json:"expectedSourceRevision,omitempty"`
	CurrentSourceRevision  project.Revision     `json:"currentSourceRevision,omitempty"`
	RootURI                string               `json:"rootUri,omitempty"`
	Diagnostics            []project.Diagnostic `json:"diagnostics,omitempty"`
}

type searchIn struct {
	Query  string `json:"query,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type searchOut struct {
	Projects   []project.ProjectSummary `json:"projects"`
	NextCursor string                   `json:"nextCursor,omitempty"`
}

type resolveIn struct {
	ProjectID string `json:"projectId,omitempty"`
	RootURI   string `json:"rootUri,omitempty"`
}

type resolveOut struct {
	ProjectID          project.ProjectID    `json:"projectId"`
	Revision           project.Revision     `json:"revision"`
	DefinitionURI      string               `json:"definitionUri"`
	RootURI            string               `json:"rootUri,omitempty"`
	ContextManifestURI string               `json:"contextManifestUri"`
	Sources            []project.Source     `json:"sources"`
	Diagnostics        []project.Diagnostic `json:"diagnostics"`
}

type validateIn struct {
	Definition map[string]any `json:"definition"`
	RootURI    string         `json:"rootUri,omitempty"`
}

type validateOut struct {
	Valid       bool                 `json:"valid"`
	Diagnostics []project.Diagnostic `json:"diagnostics"`
}

type createIn struct {
	Definition map[string]any `json:"definition"`
}

type createOut struct {
	Project project.ProjectSummary `json:"project"`
}

type patchIn struct {
	ProjectID              string         `json:"projectId"`
	ExpectedSourceRevision string         `json:"expectedSourceRevision"`
	Patch                  map[string]any `json:"patch"`
}

type deleteIn struct {
	ProjectID                 string `json:"projectId"`
	ExpectedSourceRevision    string `json:"expectedSourceRevision,omitempty"`
	ExpectedRawSourceRevision string `json:"expectedRawSourceRevision,omitempty"`
}

type deleteOut struct {
	ProjectID project.ProjectID `json:"projectId"`
	Deleted   bool              `json:"deleted"`
}

func (s *ProjectCatalogServer) registerTools() {
	destructive := true
	notDestructive := false
	closedWorld := false

	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.search",
		Description: "Search Project Catalog summaries. Start here to discover local projects.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.toolSearch)
	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.resolve",
		Description: "Resolve a project by id and/or explicit file:// rootUri.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.toolResolve)
	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.validate",
		Description: "Validate a candidate project definition without writing.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.toolValidate)
	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.create",
		Description: "Create a user-level project definition.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &notDestructive, IdempotentHint: false, OpenWorldHint: &closedWorld},
	}, s.toolCreate)
	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.patch",
		Description: "Patch a user-level project definition with optimistic concurrency.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, IdempotentHint: false, OpenWorldHint: &closedWorld},
	}, s.toolPatch)
	mcpsdk.AddTool(s.mcp, &mcpsdk.Tool{
		Name:        "project.delete",
		Description: "Delete a user-level project definition. Does not cascade to repo, context, or revisions.",
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, IdempotentHint: false, OpenWorldHint: &closedWorld},
	}, s.toolDelete)
}

func (s *ProjectCatalogServer) toolSearch(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchIn) (*mcpsdk.CallToolResult, searchOut, error) {
	page, err := s.catalog.Search(ctx, project.SearchRequest{Query: in.Query, Cursor: in.Cursor})
	if err != nil {
		return domainToolError[searchOut](err)
	}
	return nil, searchOut{Projects: nonNil(page.Projects), NextCursor: page.NextCursor}, nil
}

func (s *ProjectCatalogServer) toolResolve(ctx context.Context, _ *mcpsdk.CallToolRequest, in resolveIn) (*mcpsdk.CallToolResult, resolveOut, error) {
	if in.ProjectID == "" && in.RootURI == "" {
		return typedDomainError[resolveOut](projectToolError{Code: project.CodeInvalidRoot, Message: "projectId or rootUri is required"})
	}
	snap, err := s.catalog.Resolve(ctx, project.ResolveRequest{ProjectID: project.ProjectID(in.ProjectID), RootURI: in.RootURI})
	if err != nil {
		return domainToolError[resolveOut](err)
	}
	return nil, resolveOut{
		ProjectID:          snap.ProjectID,
		Revision:           snap.Revision,
		DefinitionURI:      revisionResourceURI(snap.ProjectID, snap.Revision),
		RootURI:            snap.RootURI,
		ContextManifestURI: contextManifestURI(snap.ProjectID, snap.RootURI),
		Sources:            nonNil(snap.Sources),
		Diagnostics:        nonNil(snap.Diagnostics),
	}, nil
}

func (s *ProjectCatalogServer) toolValidate(ctx context.Context, _ *mcpsdk.CallToolRequest, in validateIn) (*mcpsdk.CallToolResult, validateOut, error) {
	var root *url.URL
	if in.RootURI != "" {
		u, err := url.Parse(in.RootURI)
		if err != nil {
			return typedDomainError[validateOut](projectToolError{Code: project.CodeInvalidRoot, Message: "invalid rootUri"})
		}
		root = u
	}
	raw, err := json.Marshal(in.Definition)
	if err != nil {
		return typedDomainError[validateOut](projectToolError{Code: project.CodeInvalidDefinition, Message: "definition must be a JSON object"})
	}
	diags := s.valid.ValidateJSON(ctx, raw, root)
	if diags == nil {
		diags = []project.Diagnostic{}
	}
	valid := true
	for _, d := range diags {
		if d.Severity == "error" {
			valid = false
			break
		}
	}
	return nil, validateOut{Valid: valid, Diagnostics: diags}, nil
}

func (s *ProjectCatalogServer) toolCreate(ctx context.Context, _ *mcpsdk.CallToolRequest, in createIn) (*mcpsdk.CallToolResult, any, error) {
	if !s.cfg.writesEnabled {
		return writeDisabled()
	}
	raw, err := json.Marshal(in.Definition)
	if err != nil {
		return domainAnyError(&project.Error{Code: project.CodeInvalidDefinition, Message: "definition must be a JSON object"})
	}
	var def project.Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return domainAnyError(&project.Error{Code: project.CodeInvalidDefinition, Message: "definition must be a JSON object"})
	}
	snap, err := s.writer.Create(ctx, project.CreateRequest{Definition: def})
	if err != nil {
		return domainAnyError(err)
	}
	return nil, createOut{Project: summaryFromSnap(snap)}, nil
}

func (s *ProjectCatalogServer) toolPatch(ctx context.Context, _ *mcpsdk.CallToolRequest, in patchIn) (*mcpsdk.CallToolResult, any, error) {
	if !s.cfg.writesEnabled {
		return writeDisabled()
	}
	patchBytes, err := json.Marshal(in.Patch)
	if err != nil {
		return typedDomainError[createOut](projectToolError{Code: project.CodeInvalidDefinition, Message: "invalid patch"})
	}
	snap, err := s.writer.Patch(ctx, project.PatchRequest{
		ProjectID:              project.ProjectID(in.ProjectID),
		ExpectedSourceRevision: project.Revision(in.ExpectedSourceRevision),
		Patch:                  patchBytes,
	})
	if err != nil {
		return domainAnyError(err)
	}
	return nil, createOut{Project: summaryFromSnap(snap)}, nil
}

func (s *ProjectCatalogServer) toolDelete(ctx context.Context, _ *mcpsdk.CallToolRequest, in deleteIn) (*mcpsdk.CallToolResult, any, error) {
	if !s.cfg.writesEnabled {
		return writeDisabled()
	}
	err := s.writer.Delete(ctx, project.DeleteRequest{
		ProjectID:                 project.ProjectID(in.ProjectID),
		ExpectedSourceRevision:    project.Revision(in.ExpectedSourceRevision),
		ExpectedRawSourceRevision: project.Revision(in.ExpectedRawSourceRevision),
	})
	if err != nil {
		return domainAnyError(err)
	}
	return nil, deleteOut{ProjectID: project.ProjectID(in.ProjectID), Deleted: true}, nil
}

func summaryFromSnap(snap project.Snapshot) project.ProjectSummary {
	return project.ProjectSummary{
		ProjectID:       snap.ProjectID,
		Title:           snap.Definition.Name,
		Repo:            snap.Definition.Repo,
		Branch:          snap.Definition.Branch,
		Revision:        snap.Revision,
		SourceRevision:  snap.SourceRevision,
		DefinitionURI:   definitionResourceURI(snap.ProjectID),
		ContextURI:      contextManifestURI(snap.ProjectID, ""),
		DiagnosticCount: len(snap.Diagnostics),
	}
}

func writeDisabled() (*mcpsdk.CallToolResult, any, error) {
	return domainAnyError(&project.Error{Code: project.CodeWriteDisabled, Message: "canonical writes are disabled"})
}

func domainAnyError(err error) (*mcpsdk.CallToolResult, any, error) {
	e, ok := project.AsError(err)
	if !ok {
		e = &project.Error{Code: project.CodeInternalError, Message: "internal error"}
	}
	body := toolErrorBody{Error: projectToolError{
		Code:                   e.Code,
		Message:                e.Message,
		ProjectID:              e.ProjectID,
		ExpectedSourceRevision: e.ExpectedSourceRevision,
		CurrentSourceRevision:  e.CurrentSourceRevision,
		RootURI:                e.RootURI,
		Diagnostics:            e.Diagnostics,
	}}
	return &mcpsdk.CallToolResult{
		IsError: true,
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: e.Code + ": " + e.Message}},
	}, body, nil
}

func domainToolError[T any](err error) (*mcpsdk.CallToolResult, T, error) {
	e, ok := project.AsError(err)
	if !ok {
		return typedDomainError[T](projectToolError{Code: project.CodeInternalError, Message: "internal error"})
	}
	return typedDomainError[T](projectToolError{
		Code:                   e.Code,
		Message:                e.Message,
		ProjectID:              e.ProjectID,
		ExpectedSourceRevision: e.ExpectedSourceRevision,
		CurrentSourceRevision:  e.CurrentSourceRevision,
		RootURI:                e.RootURI,
		Diagnostics:            e.Diagnostics,
	})
}

func typedDomainError[T any](e projectToolError) (*mcpsdk.CallToolResult, T, error) {
	var zero T
	return &mcpsdk.CallToolResult{
		IsError:           true,
		StructuredContent: toolErrorBody{Error: e},
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: e.Code + ": " + e.Message},
		},
	}, zero, nil
}
