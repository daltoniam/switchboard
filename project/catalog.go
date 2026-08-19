package project

import (
	"context"
	"encoding/json"
	"net/url"
)

// ProjectID is the validated filename stem of a user-level project file.
type ProjectID string

// Revision is a content digest of the form sha256:<64-lowercase-hex>.
type Revision string

// Valid reports whether r has the frozen revision spelling.
func (r Revision) Valid() bool {
	_, err := ParseRevision(r)
	return err == nil
}

// DigestHex returns the 64-character hex digest without the sha256: prefix.
func (r Revision) DigestHex() string {
	parsed, err := ParseRevision(r)
	if err != nil {
		return ""
	}
	return string(parsed)[len("sha256:"):]
}

// Source identifies one definition layer that contributed to a snapshot.
type Source struct {
	Kind       string `json:"kind"`
	URI        string `json:"uri"`
	Precedence int    `json:"precedence"`
}

// Diagnostic is a value-free validation finding.
type Diagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	SourceURI string `json:"sourceUri,omitempty"`
	Path      string `json:"path,omitempty"`
}

// ProjectSummary is the valid-project list/search row frozen in contracts.md.
type ProjectSummary struct {
	ProjectID       ProjectID `json:"projectId"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Revision        Revision  `json:"revision"`
	SourceRevision  Revision  `json:"sourceRevision"`
	DefinitionURI   string    `json:"definitionUri"`
	DiagnosticCount int       `json:"diagnosticCount"`
}

// InvalidProjectSummary is the invalid-source list row. It never mints a
// valid effective revision or definition/context URI.
type InvalidProjectSummary struct {
	ProjectID         ProjectID    `json:"projectId"`
	Title             string       `json:"title"`
	RawSourceRevision Revision     `json:"rawSourceRevision"`
	DiagnosticCount   int          `json:"diagnosticCount"`
	DiagnosticsURI    string       `json:"diagnosticsUri"`
	Diagnostics       []Diagnostic `json:"diagnostics,omitempty"`
}

// DiagnosticsEnvelope is the standalone diagnostics resource body.
type DiagnosticsEnvelope struct {
	ProjectID   ProjectID    `json:"projectId"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Snapshot is an immutable effective project resolution.
type Snapshot struct {
	ProjectID      ProjectID
	Revision       Revision
	SourceRevision Revision
	Definition     Definition
	Sources        []Source
	Diagnostics    []Diagnostic
	RootURI        string
	UserBytes      []byte
}

// RevisionSnapshot is the immutable archive envelope: project ID, revision,
// and effective definition only.
type RevisionSnapshot struct {
	ProjectID  ProjectID
	Revision   Revision
	Definition Definition
}

// Page is a deterministic, cursor-paginated catalog listing.
type Page struct {
	Projects        []ProjectSummary
	InvalidProjects []InvalidProjectSummary
	NextCursor      string
}

// SearchRequest filters and pages catalog summaries.
type SearchRequest struct {
	Query  string
	Cursor string
}

// ResolveRequest identifies a project by ID and/or explicit file:// root.
type ResolveRequest struct {
	ProjectID ProjectID
	RootURI   string
}

// CreateRequest creates a user-level definition from a typed value.
type CreateRequest struct {
	Definition Definition
}

// PatchRequest applies an RFC 7396 merge patch to a valid user definition.
type PatchRequest struct {
	ProjectID              ProjectID
	ExpectedSourceRevision Revision
	Patch                  json.RawMessage
}

// ReplaceRequest replaces a user definition through a typed transport while
// retaining the same optimistic concurrency contract as PatchRequest.
type ReplaceRequest struct {
	ProjectID              ProjectID
	ExpectedSourceRevision Revision
	Definition             Definition
}

// DefinitionPatch is the closed, typed patch supported by the AWM gRPC API.
type DefinitionPatch struct {
	Description *string
}

// TypedPatchRequest applies a closed typed patch with optimistic concurrency.
type TypedPatchRequest struct {
	ProjectID              ProjectID
	ExpectedSourceRevision Revision
	Patch                  DefinitionPatch
}

// DeleteRequest removes a user-level file using exactly one CAS token.
type DeleteRequest struct {
	ProjectID                 ProjectID
	ExpectedSourceRevision    Revision
	ExpectedRawSourceRevision Revision
}

// PersistedUserDefinition is the exact user-layer document written on create.
type PersistedUserDefinition struct {
	Definition Definition
	Bytes      []byte
}

// ResourcePresence answers whether an independently stored Resource currently
// exists. The Resource store remains the sole authority for Resource records;
// the catalog only observes IDs through this port.
type ResourcePresence interface {
	ResourceExists(ctx context.Context, resourceID string) (bool, error)
}

// Catalog is the transport-neutral read port.
type Catalog interface {
	List(context.Context, string) (Page, error)
	Search(context.Context, SearchRequest) (Page, error)
	Get(context.Context, ProjectID) (Snapshot, error)
	GetRevision(context.Context, ProjectID, Revision) (RevisionSnapshot, error)
	Diagnostics(context.Context, ProjectID, *url.URL) (DiagnosticsEnvelope, error)
	Resolve(context.Context, ResolveRequest) (Snapshot, error)
}

// CatalogValidator validates a candidate JSON definition without mutation.
// It is retained for transports such as MCP that intentionally accept an
// untyped candidate object for diagnostic purposes.
type CatalogValidator interface {
	ValidateJSON(context.Context, json.RawMessage, *url.URL) []Diagnostic
}

// DefinitionValidator validates an already-decoded definition. Strongly typed
// transports use this port to avoid a marshal/unmarshal cycle at the boundary.
type DefinitionValidator interface {
	ValidateDefinition(context.Context, Definition, *url.URL) []Diagnostic
}

// CatalogWriter is the canonical write port. Mutations always require CAS.
type CatalogWriter interface {
	Create(context.Context, CreateRequest) (Snapshot, error)
	Patch(context.Context, PatchRequest) (Snapshot, error)
	Delete(context.Context, DeleteRequest) error
}

// CatalogReplacer is the typed full-definition update port used by gRPC.
type CatalogReplacer interface {
	Replace(context.Context, ReplaceRequest) (Snapshot, error)
	PatchDefinition(context.Context, TypedPatchRequest) (Snapshot, error)
}

// CompatibilityWriter preserves last-write-wins projectinterop behavior.
type CompatibilityWriter interface {
	CreateCompatibility(context.Context, CreateRequest) (PersistedUserDefinition, Snapshot, error)
	PatchCompatibility(context.Context, ProjectID, json.RawMessage) (Snapshot, error)
	DeleteCompatibility(context.Context, ProjectID) error
}

// CatalogStore is the filesystem-backed catalog authority.
type CatalogStore interface {
	Catalog
	CatalogValidator
	CatalogWriter
	CompatibilityWriter
	ConfigDir() string
}
