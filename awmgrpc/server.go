// Package awmgrpc exposes Switchboard's implemented Agent Work Model subset
// as generated, strongly typed gRPC services.
package awmgrpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	awmv1 "github.com/daltoniam/switchboard/gen/awm/v1"
	"github.com/daltoniam/switchboard/project"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Options controls which gRPC surfaces and mutations are available.
type Options struct {
	CatalogEnabled bool
	WritesEnabled  bool
}

type service struct {
	awmv1.UnimplementedProjectCatalogServiceServer
	awmv1.UnimplementedWorkProfileServiceServer
	awmv1.UnimplementedAgentProfileServiceServer
	awmv1.UnimplementedWorkSessionServiceServer
	awmv1.UnimplementedResourceServiceServer
	awmv1.UnimplementedResourceBindingServiceServer

	catalog   project.Catalog
	writer    project.CatalogWriter
	replacer  project.CatalogReplacer
	validator project.DefinitionValidator
	work      *awm.Store
	opts      Options
}

// Register attaches the gRPC services to registrar. ProjectCatalogService is
// registered only when the corresponding MCP catalog surface is enabled. Work
// model services are always registered and share the same awm.Store as MCP.
func Register(
	registrar grpc.ServiceRegistrar,
	catalog project.Catalog,
	writer project.CatalogWriter,
	validator project.DefinitionValidator,
	work *awm.Store,
	opts Options,
) {
	s := &service{
		catalog:   catalog,
		writer:    writer,
		validator: validator,
		work:      work,
		opts:      opts,
	}
	if replacer, ok := writer.(project.CatalogReplacer); ok {
		s.replacer = replacer
	}
	if opts.CatalogEnabled {
		awmv1.RegisterProjectCatalogServiceServer(registrar, s)
	}
	awmv1.RegisterWorkProfileServiceServer(registrar, s)
	awmv1.RegisterAgentProfileServiceServer(registrar, s)
	awmv1.RegisterWorkSessionServiceServer(registrar, s)
	awmv1.RegisterResourceServiceServer(registrar, s)
	awmv1.RegisterResourceBindingServiceServer(registrar, s)
}

func (s *service) requireWork() error {
	if s.work == nil {
		return detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "work model store is unavailable")
	}
	return nil
}

func (s *service) requireCatalog() error {
	if s.catalog == nil {
		return detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "project catalog is unavailable")
	}
	return nil
}

func (s *service) requireWrite() error {
	if !s.opts.WritesEnabled {
		return detailError(codes.PermissionDenied, awmv1.ErrorCode_ERROR_CODE_WRITE_DISABLED, "canonical writes are disabled")
	}
	return nil
}

// --- Resources ---

func (s *service) ListResources(ctx context.Context, _ *awmv1.ListResourcesRequest) (*awmv1.ListResourcesResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	resources, err := s.work.ListResources(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*awmv1.Resource, 0, len(resources))
	for i := range resources {
		out = append(out, resourceToProto(resources[i]))
	}
	return &awmv1.ListResourcesResponse{Resources: out}, nil
}

func (s *service) GetResource(ctx context.Context, req *awmv1.GetResourceRequest) (*awmv1.GetResourceResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	resource, err := s.work.GetResource(ctx, strings.TrimSpace(req.GetResourceId()))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetResourceResponse{Resource: resourceToProto(resource)}, nil
}

func (s *service) PutResource(ctx context.Context, req *awmv1.PutResourceRequest) (*awmv1.PutResourceResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	resource, err := resourceFromProto(req.GetResource())
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.PutResource(ctx, resource)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PutResourceResponse{Resource: resourceToProto(stored)}, nil
}

func (s *service) DeleteResource(ctx context.Context, req *awmv1.DeleteResourceRequest) (*awmv1.DeleteResourceResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetResourceId())
	if err := s.work.DeleteResource(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteResourceResponse{ResourceId: id, Deleted: true}, nil
}

// --- Resource bindings ---

func (s *service) ListResourceBindings(ctx context.Context, req *awmv1.ListResourceBindingsRequest) (*awmv1.ListResourceBindingsResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	state, err := bindingStateFromProto(req.GetState(), true)
	if err != nil {
		return nil, invalidArgument(err)
	}
	bindings, err := s.work.ListResourceBindings(ctx, state, strings.TrimSpace(req.GetWorkSessionId()), strings.TrimSpace(req.GetResourceId()))
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*awmv1.ResourceBinding, 0, len(bindings))
	for i := range bindings {
		out = append(out, resourceBindingToProto(bindings[i]))
	}
	return &awmv1.ListResourceBindingsResponse{ResourceBindings: out}, nil
}

func (s *service) GetResourceBinding(ctx context.Context, req *awmv1.GetResourceBindingRequest) (*awmv1.GetResourceBindingResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	binding, err := s.work.GetResourceBinding(ctx, strings.TrimSpace(req.GetResourceBindingId()))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetResourceBindingResponse{ResourceBinding: resourceBindingToProto(binding)}, nil
}

func (s *service) CreateResourceBinding(ctx context.Context, req *awmv1.CreateResourceBindingRequest) (*awmv1.CreateResourceBindingResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	binding, err := resourceBindingFromProto(req.GetResourceBinding())
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.CreateResourceBinding(ctx, binding)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.CreateResourceBindingResponse{ResourceBinding: resourceBindingToProto(stored)}, nil
}

func (s *service) TransitionResourceBinding(ctx context.Context, req *awmv1.TransitionResourceBindingRequest) (*awmv1.TransitionResourceBindingResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	state, err := bindingStateFromProto(req.GetState(), false)
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.TransitionResourceBinding(ctx, strings.TrimSpace(req.GetResourceBindingId()), state)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.TransitionResourceBindingResponse{ResourceBinding: resourceBindingToProto(stored)}, nil
}

func (s *service) PatchResourceBinding(ctx context.Context, req *awmv1.PatchResourceBindingRequest) (*awmv1.PatchResourceBindingResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	var grant awm.PolicyDocument
	var err error
	if req.Grant != nil {
		grant, err = policyFromProto(req.Grant)
		if err != nil {
			return nil, invalidArgument(err)
		}
	}
	stored, err := s.work.PatchResourceBinding(ctx, strings.TrimSpace(req.GetResourceBindingId()), req.ResolvedLocator, grant)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PatchResourceBindingResponse{ResourceBinding: resourceBindingToProto(stored)}, nil
}

func (s *service) DeleteResourceBinding(ctx context.Context, req *awmv1.DeleteResourceBindingRequest) (*awmv1.DeleteResourceBindingResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetResourceBindingId())
	if err := s.work.DeleteResourceBinding(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteResourceBindingResponse{ResourceBindingId: id, Deleted: true}, nil
}

// --- Work profiles ---

func (s *service) ListWorkProfiles(ctx context.Context, _ *awmv1.ListWorkProfilesRequest) (*awmv1.ListWorkProfilesResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	profiles, err := s.work.ListWorkProfiles(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*awmv1.WorkProfile, 0, len(profiles))
	for i := range profiles {
		out = append(out, workProfileToProto(profiles[i]))
	}
	return &awmv1.ListWorkProfilesResponse{WorkProfiles: out}, nil
}

func (s *service) GetWorkProfile(ctx context.Context, req *awmv1.GetWorkProfileRequest) (*awmv1.GetWorkProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	profile, err := s.work.GetWorkProfile(ctx, strings.TrimSpace(req.GetWorkProfileId()))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetWorkProfileResponse{WorkProfile: workProfileToProto(profile)}, nil
}

func (s *service) PutWorkProfile(ctx context.Context, req *awmv1.PutWorkProfileRequest) (*awmv1.PutWorkProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	profile, err := workProfileFromProto(req.GetWorkProfile())
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.PutWorkProfile(ctx, profile)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PutWorkProfileResponse{WorkProfile: workProfileToProto(stored)}, nil
}

func (s *service) DeleteWorkProfile(ctx context.Context, req *awmv1.DeleteWorkProfileRequest) (*awmv1.DeleteWorkProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetWorkProfileId())
	if err := s.work.DeleteWorkProfile(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteWorkProfileResponse{WorkProfileId: id, Deleted: true}, nil
}

// --- Agent profiles ---

func (s *service) ListAgentProfiles(ctx context.Context, _ *awmv1.ListAgentProfilesRequest) (*awmv1.ListAgentProfilesResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	profiles, err := s.work.ListAgentProfiles(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*awmv1.AgentProfile, 0, len(profiles))
	for i := range profiles {
		out = append(out, agentProfileToProto(profiles[i]))
	}
	return &awmv1.ListAgentProfilesResponse{AgentProfiles: out}, nil
}

func (s *service) GetAgentProfile(ctx context.Context, req *awmv1.GetAgentProfileRequest) (*awmv1.GetAgentProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	profile, err := s.work.GetAgentProfile(ctx, strings.TrimSpace(req.GetAgentProfileId()))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetAgentProfileResponse{AgentProfile: agentProfileToProto(profile)}, nil
}

func (s *service) PutAgentProfile(ctx context.Context, req *awmv1.PutAgentProfileRequest) (*awmv1.PutAgentProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	profile, err := agentProfileFromProto(req.GetAgentProfile())
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.PutAgentProfile(ctx, profile)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PutAgentProfileResponse{AgentProfile: agentProfileToProto(stored)}, nil
}

func (s *service) DeleteAgentProfile(ctx context.Context, req *awmv1.DeleteAgentProfileRequest) (*awmv1.DeleteAgentProfileResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetAgentProfileId())
	if err := s.work.DeleteAgentProfile(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteAgentProfileResponse{AgentProfileId: id, Deleted: true}, nil
}

// --- Work sessions ---

func (s *service) ListWorkSessions(ctx context.Context, req *awmv1.ListWorkSessionsRequest) (*awmv1.ListWorkSessionsResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	state, err := stateFromProto(req.GetState(), true)
	if err != nil {
		return nil, invalidArgument(err)
	}
	sessions, err := s.work.ListWorkSessions(ctx, state, strings.TrimSpace(req.GetProjectId()))
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*awmv1.WorkSession, 0, len(sessions))
	for i := range sessions {
		out = append(out, workSessionToProto(sessions[i]))
	}
	return &awmv1.ListWorkSessionsResponse{WorkSessions: out}, nil
}

func (s *service) GetWorkSession(ctx context.Context, req *awmv1.GetWorkSessionRequest) (*awmv1.GetWorkSessionResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	session, err := s.work.GetWorkSession(ctx, strings.TrimSpace(req.GetWorkSessionId()))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetWorkSessionResponse{WorkSession: workSessionToProto(session)}, nil
}

func (s *service) CreateWorkSession(ctx context.Context, req *awmv1.CreateWorkSessionRequest) (*awmv1.CreateWorkSessionResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	session, err := workSessionFromProto(req.GetWorkSession())
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.CreateWorkSession(ctx, session)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.CreateWorkSessionResponse{WorkSession: workSessionToProto(stored)}, nil
}

func (s *service) TransitionWorkSession(ctx context.Context, req *awmv1.TransitionWorkSessionRequest) (*awmv1.TransitionWorkSessionResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	state, err := stateFromProto(req.GetState(), false)
	if err != nil {
		return nil, invalidArgument(err)
	}
	stored, err := s.work.TransitionWorkSession(ctx, strings.TrimSpace(req.GetWorkSessionId()), state)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.TransitionWorkSessionResponse{WorkSession: workSessionToProto(stored)}, nil
}

func (s *service) PatchWorkSession(ctx context.Context, req *awmv1.PatchWorkSessionRequest) (*awmv1.PatchWorkSessionResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	var ids *[]string
	if req.AgentProfileIds != nil {
		values := append([]string(nil), req.AgentProfileIds.Values...)
		ids = &values
	}
	var policy awm.PolicyDocument
	var err error
	if req.Policy != nil {
		policy, err = policyFromProto(req.Policy)
		if err != nil {
			return nil, invalidArgument(err)
		}
	}
	stored, err := s.work.PatchWorkSession(ctx, strings.TrimSpace(req.GetWorkSessionId()), req.DisplayName, ids, policy)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PatchWorkSessionResponse{WorkSession: workSessionToProto(stored)}, nil
}

func (s *service) DeleteWorkSession(ctx context.Context, req *awmv1.DeleteWorkSessionRequest) (*awmv1.DeleteWorkSessionResponse, error) {
	if err := s.requireWork(); err != nil {
		return nil, err
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetWorkSessionId())
	if err := s.work.DeleteWorkSession(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteWorkSessionResponse{WorkSessionId: id, Deleted: true}, nil
}

// --- Project catalog ---

func (s *service) ListProjects(ctx context.Context, req *awmv1.ListProjectsRequest) (*awmv1.ListProjectsResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	page, err := s.catalog.List(ctx, req.GetCursor())
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.ListProjectsResponse{Projects: summariesToProto(page.Projects), NextCursor: page.NextCursor}, nil
}

func (s *service) SearchProjects(ctx context.Context, req *awmv1.SearchProjectsRequest) (*awmv1.SearchProjectsResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	page, err := s.catalog.Search(ctx, project.SearchRequest{Query: req.GetQuery(), Cursor: req.GetCursor()})
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.SearchProjectsResponse{Projects: summariesToProto(page.Projects), NextCursor: page.NextCursor}, nil
}

func (s *service) GetProject(ctx context.Context, req *awmv1.GetProjectRequest) (*awmv1.GetProjectResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	snapshot, err := s.catalog.Get(ctx, project.ProjectID(strings.TrimSpace(req.GetProjectId())))
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetProjectResponse{
		ProjectId:      string(snapshot.ProjectID),
		Revision:       string(snapshot.Revision),
		SourceRevision: string(snapshot.SourceRevision),
		Definition:     definitionToProto(snapshot.Definition),
		Summary:        summaryToProto(summaryFromSnapshot(snapshot)),
		Sources:        sourcesToProto(snapshot.Sources),
		Diagnostics:    diagnosticsToProto(snapshot.Diagnostics),
		Uri:            fmt.Sprintf("project://registry/projects/%s", url.PathEscape(string(snapshot.ProjectID))),
	}, nil
}

func (s *service) ResolveProject(ctx context.Context, req *awmv1.ResolveProjectRequest) (*awmv1.ResolveProjectResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetProjectId()) == "" && strings.TrimSpace(req.GetRootUri()) == "" {
		return nil, invalidArgument(errors.New("project_id or root_uri is required"))
	}
	snapshot, err := s.catalog.Resolve(ctx, project.ResolveRequest{
		ProjectID: project.ProjectID(strings.TrimSpace(req.GetProjectId())),
		RootURI:   strings.TrimSpace(req.GetRootUri()),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.ResolveProjectResponse{
		ProjectId:     string(snapshot.ProjectID),
		Revision:      string(snapshot.Revision),
		DefinitionUri: revisionURI(snapshot.ProjectID, snapshot.Revision),
		RootUri:       snapshot.RootURI,
		Sources:       sourcesToProto(snapshot.Sources),
		Diagnostics:   diagnosticsToProto(snapshot.Diagnostics),
	}, nil
}

func (s *service) ValidateProject(ctx context.Context, req *awmv1.ValidateProjectRequest) (*awmv1.ValidateProjectResponse, error) {
	if s.validator == nil {
		return nil, detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "project validator is unavailable")
	}
	definition, err := definitionFromProto(req.GetDefinition())
	if err != nil {
		return nil, invalidArgument(err)
	}
	root, err := optionalURL(req.GetRootUri())
	if err != nil {
		return nil, invalidArgument(err)
	}
	diagnostics := s.validator.ValidateDefinition(ctx, definition, root)
	valid := true
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			valid = false
			break
		}
	}
	return &awmv1.ValidateProjectResponse{Valid: valid, Diagnostics: diagnosticsToProto(diagnostics)}, nil
}

func (s *service) CreateProject(ctx context.Context, req *awmv1.CreateProjectRequest) (*awmv1.CreateProjectResponse, error) {
	if s.writer == nil {
		return nil, detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "project writer is unavailable")
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	definition, err := definitionFromProto(req.GetDefinition())
	if err != nil {
		return nil, invalidArgument(err)
	}
	snapshot, err := s.writer.Create(ctx, project.CreateRequest{Definition: definition})
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.CreateProjectResponse{Project: summaryToProto(summaryFromSnapshot(snapshot))}, nil
}

func (s *service) UpdateProject(ctx context.Context, req *awmv1.UpdateProjectRequest) (*awmv1.UpdateProjectResponse, error) {
	snapshot, err := s.replaceProject(ctx, req.GetProjectId(), req.GetExpectedSourceRevision(), req.GetDefinition())
	if err != nil {
		return nil, err
	}
	return &awmv1.UpdateProjectResponse{Project: summaryToProto(summaryFromSnapshot(snapshot))}, nil
}

func (s *service) PatchProject(ctx context.Context, req *awmv1.PatchProjectRequest) (*awmv1.PatchProjectResponse, error) {
	if s.replacer == nil {
		return nil, detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "typed project writer is unavailable")
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	if req.GetPatch() == nil {
		return nil, invalidArgument(errors.New("patch is required"))
	}
	snapshot, err := s.replacer.PatchDefinition(ctx, project.TypedPatchRequest{
		ProjectID:              project.ProjectID(strings.TrimSpace(req.GetProjectId())),
		ExpectedSourceRevision: project.Revision(strings.TrimSpace(req.GetExpectedSourceRevision())),
		Patch: project.DefinitionPatch{
			Description: req.GetPatch().Description,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.PatchProjectResponse{Project: summaryToProto(summaryFromSnapshot(snapshot))}, nil
}

func (s *service) replaceProject(ctx context.Context, id, expected string, in *awmv1.ProjectDefinition) (project.Snapshot, error) {
	if s.replacer == nil {
		return project.Snapshot{}, detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "typed project writer is unavailable")
	}
	if err := s.requireWrite(); err != nil {
		return project.Snapshot{}, err
	}
	definition, err := definitionFromProto(in)
	if err != nil {
		return project.Snapshot{}, invalidArgument(err)
	}
	snapshot, err := s.replacer.Replace(ctx, project.ReplaceRequest{
		ProjectID:              project.ProjectID(strings.TrimSpace(id)),
		ExpectedSourceRevision: project.Revision(strings.TrimSpace(expected)),
		Definition:             definition,
	})
	if err != nil {
		return project.Snapshot{}, rpcError(err)
	}
	return snapshot, nil
}

func (s *service) DeleteProject(ctx context.Context, req *awmv1.DeleteProjectRequest) (*awmv1.DeleteProjectResponse, error) {
	if s.writer == nil {
		return nil, detailError(codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE, "project writer is unavailable")
	}
	if err := s.requireWrite(); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetProjectId())
	err := s.writer.Delete(ctx, project.DeleteRequest{
		ProjectID:                 project.ProjectID(id),
		ExpectedSourceRevision:    project.Revision(strings.TrimSpace(req.GetExpectedSourceRevision())),
		ExpectedRawSourceRevision: project.Revision(strings.TrimSpace(req.GetExpectedRawSourceRevision())),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.DeleteProjectResponse{ProjectId: id, Deleted: true}, nil
}

func (s *service) GetProjectRevision(ctx context.Context, req *awmv1.GetProjectRevisionRequest) (*awmv1.GetProjectRevisionResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	revision, err := project.ParseRevision(project.Revision(strings.TrimSpace(req.GetRevision())))
	if err != nil {
		return nil, invalidArgument(err)
	}
	snapshot, err := s.catalog.GetRevision(ctx, project.ProjectID(strings.TrimSpace(req.GetProjectId())), revision)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetProjectRevisionResponse{
		ProjectId: string(snapshot.ProjectID), Revision: string(snapshot.Revision), Definition: definitionToProto(snapshot.Definition),
	}, nil
}

func (s *service) GetProjectDiagnostics(ctx context.Context, req *awmv1.GetProjectDiagnosticsRequest) (*awmv1.GetProjectDiagnosticsResponse, error) {
	if err := s.requireCatalog(); err != nil {
		return nil, err
	}
	root, err := optionalURL(req.GetRootUri())
	if err != nil {
		return nil, invalidArgument(err)
	}
	diagnostics, err := s.catalog.Diagnostics(ctx, project.ProjectID(strings.TrimSpace(req.GetProjectId())), root)
	if err != nil {
		return nil, rpcError(err)
	}
	return &awmv1.GetProjectDiagnosticsResponse{
		ProjectId: string(diagnostics.ProjectID), Diagnostics: diagnosticsToProto(diagnostics.Diagnostics),
	}, nil
}

// --- Typed conversions ---

func policyFromProto(in *awmv1.PolicyDocument) (awm.PolicyDocument, error) {
	if in == nil {
		return nil, nil
	}
	out := make(awm.PolicyDocument, len(in.Capabilities))
	for _, capability := range in.Capabilities {
		if capability == nil || strings.TrimSpace(capability.Capability) == "" {
			return nil, errors.New("policy capability is required")
		}
		name := strings.TrimSpace(capability.Capability)
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("duplicate policy capability %q", name)
		}
		out[name] = capability.Allowed
	}
	return out, nil
}

func policyToProto(in awm.PolicyDocument) *awmv1.PolicyDocument {
	if in == nil {
		return nil
	}
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	sort.Strings(names)
	out := &awmv1.PolicyDocument{Capabilities: make([]*awmv1.CapabilityPolicy, 0, len(names))}
	for _, name := range names {
		out.Capabilities = append(out.Capabilities, &awmv1.CapabilityPolicy{Capability: name, Allowed: in[name]})
	}
	return out
}

func resourceFromProto(in *awmv1.Resource) (awm.Resource, error) {
	if in == nil {
		return awm.Resource{}, errors.New("resource is required")
	}
	return awm.Resource{
		Version: in.Version, ResourceID: in.ResourceId, URI: in.Uri,
		Kind: in.Kind, DisplayName: in.DisplayName,
	}, nil
}

func resourceToProto(in awm.Resource) *awmv1.Resource {
	return &awmv1.Resource{
		Version: in.Version, ResourceId: in.ResourceID, Uri: in.URI,
		Kind: in.Kind, DisplayName: in.DisplayName,
	}
}

func resourceBindingFromProto(in *awmv1.ResourceBinding) (awm.ResourceBinding, error) {
	if in == nil {
		return awm.ResourceBinding{}, errors.New("resource_binding is required")
	}
	state, err := bindingStateFromProto(in.State, true)
	if err != nil {
		return awm.ResourceBinding{}, err
	}
	grant, err := policyFromProto(in.Grant)
	if err != nil {
		return awm.ResourceBinding{}, err
	}
	return awm.ResourceBinding{
		Version: in.Version, ResourceBindingID: in.ResourceBindingId,
		WorkSessionID: in.WorkSessionId, ResourceID: in.ResourceId,
		Grant: grant, ResolvedLocator: in.ResolvedLocator, State: state,
	}, nil
}

func resourceBindingToProto(in awm.ResourceBinding) *awmv1.ResourceBinding {
	out := &awmv1.ResourceBinding{
		Version: in.Version, ResourceBindingId: in.ResourceBindingID,
		WorkSessionId: in.WorkSessionID, ResourceId: in.ResourceID,
		Grant: policyToProto(in.Grant), ResolvedLocator: in.ResolvedLocator,
		State: bindingStateToProto(in.State),
	}
	if !in.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(in.CreatedAt)
	}
	if !in.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(in.UpdatedAt)
	}
	return out
}

func bindingStateFromProto(in awmv1.ResourceBindingState, allowUnspecified bool) (string, error) {
	switch in {
	case awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_UNSPECIFIED:
		if allowUnspecified {
			return "", nil
		}
	case awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_PROPOSED:
		return awm.BindingStateProposed, nil
	case awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_BOUND:
		return awm.BindingStateBound, nil
	case awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_REVOKED:
		return awm.BindingStateRevoked, nil
	}
	return "", fmt.Errorf("unsupported resource binding state %s", in.String())
}

func bindingStateToProto(in string) awmv1.ResourceBindingState {
	switch in {
	case awm.BindingStateProposed:
		return awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_PROPOSED
	case awm.BindingStateBound:
		return awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_BOUND
	case awm.BindingStateRevoked:
		return awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_REVOKED
	default:
		return awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_UNSPECIFIED
	}
}

func workProfileFromProto(in *awmv1.WorkProfile) (awm.WorkProfile, error) {
	if in == nil {
		return awm.WorkProfile{}, errors.New("work_profile is required")
	}
	policy, err := policyFromProto(in.DefaultPolicy)
	if err != nil {
		return awm.WorkProfile{}, err
	}
	return awm.WorkProfile{
		Version: in.Version, WorkProfileID: in.WorkProfileId, DisplayName: in.DisplayName,
		Description: in.Description, ProjectIDs: append([]string(nil), in.ProjectIds...),
		IntendedResources: append([]string(nil), in.IntendedResources...), DefaultPolicy: policy,
	}, nil
}

func workProfileToProto(in awm.WorkProfile) *awmv1.WorkProfile {
	return &awmv1.WorkProfile{
		Version: in.Version, WorkProfileId: in.WorkProfileID, DisplayName: in.DisplayName,
		Description: in.Description, ProjectIds: append([]string(nil), in.ProjectIDs...),
		IntendedResources: append([]string(nil), in.IntendedResources...), DefaultPolicy: policyToProto(in.DefaultPolicy),
	}
}

func agentProfileFromProto(in *awmv1.AgentProfile) (awm.AgentProfile, error) {
	if in == nil {
		return awm.AgentProfile{}, errors.New("agent_profile is required")
	}
	var constraints *awm.AgentConstraints
	if in.Constraints != nil {
		constraints = &awm.AgentConstraints{ProjectIDs: append([]string(nil), in.Constraints.ProjectIds...)}
	}
	return awm.AgentProfile{
		Version: in.Version, AgentProfileID: in.AgentProfileId, DisplayName: in.DisplayName,
		Description: in.Description, Capabilities: append([]string(nil), in.Capabilities...), Constraints: constraints,
	}, nil
}

func agentProfileToProto(in awm.AgentProfile) *awmv1.AgentProfile {
	out := &awmv1.AgentProfile{
		Version: in.Version, AgentProfileId: in.AgentProfileID, DisplayName: in.DisplayName,
		Description: in.Description, Capabilities: append([]string(nil), in.Capabilities...),
	}
	if in.Constraints != nil {
		ids := append([]string(nil), in.Constraints.ProjectIDs...)
		if in.Constraints.ProjectID != "" {
			ids = append(ids, in.Constraints.ProjectID)
		}
		out.Constraints = &awmv1.AgentConstraints{ProjectIds: ids}
	}
	return out
}

func workSessionFromProto(in *awmv1.WorkSession) (awm.WorkSession, error) {
	if in == nil {
		return awm.WorkSession{}, errors.New("work_session is required")
	}
	state, err := stateFromProto(in.State, true)
	if err != nil {
		return awm.WorkSession{}, err
	}
	policy, err := policyFromProto(in.Policy)
	if err != nil {
		return awm.WorkSession{}, err
	}
	return awm.WorkSession{
		Version: in.Version, WorkSessionID: in.WorkSessionId, DisplayName: in.DisplayName,
		ProjectID: in.ProjectId, ProjectSnapshotID: in.ProjectSnapshotId, ProjectRevision: in.ProjectRevision,
		WorkProfileID: in.WorkProfileId, AgentProfileIDs: append([]string(nil), in.AgentProfileIds...), State: state, Policy: policy,
	}, nil
}

func workSessionToProto(in awm.WorkSession) *awmv1.WorkSession {
	out := &awmv1.WorkSession{
		Version: in.Version, WorkSessionId: in.WorkSessionID, DisplayName: in.DisplayName,
		ProjectId: in.ProjectID, ProjectSnapshotId: in.ProjectSnapshotID, ProjectRevision: in.ProjectRevision,
		WorkProfileId: in.WorkProfileID, AgentProfileIds: append([]string(nil), in.AgentProfileIDs...),
		State: stateToProto(in.State), Policy: policyToProto(in.Policy),
	}
	if !in.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(in.CreatedAt)
	}
	if !in.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(in.UpdatedAt)
	}
	if in.ClosedAt != nil {
		out.ClosedAt = timestamppb.New(*in.ClosedAt)
	}
	return out
}

func stateFromProto(in awmv1.WorkSessionState, allowUnspecified bool) (string, error) {
	switch in {
	case awmv1.WorkSessionState_WORK_SESSION_STATE_UNSPECIFIED:
		if allowUnspecified {
			return "", nil
		}
	case awmv1.WorkSessionState_WORK_SESSION_STATE_PROPOSED:
		return awm.StateProposed, nil
	case awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN:
		return awm.StateOpen, nil
	case awmv1.WorkSessionState_WORK_SESSION_STATE_PAUSED:
		return awm.StatePaused, nil
	case awmv1.WorkSessionState_WORK_SESSION_STATE_CLOSED:
		return awm.StateClosed, nil
	case awmv1.WorkSessionState_WORK_SESSION_STATE_ABORTED:
		return awm.StateAborted, nil
	}
	return "", fmt.Errorf("unsupported work session state %s", in.String())
}

func stateToProto(in string) awmv1.WorkSessionState {
	switch in {
	case awm.StateProposed:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_PROPOSED
	case awm.StateOpen:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN
	case awm.StatePaused:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_PAUSED
	case awm.StateClosed:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_CLOSED
	case awm.StateAborted:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_ABORTED
	default:
		return awmv1.WorkSessionState_WORK_SESSION_STATE_UNSPECIFIED
	}
}

func definitionFromProto(in *awmv1.ProjectDefinition) (project.Definition, error) {
	if in == nil {
		return project.Definition{}, errors.New("definition is required")
	}
	policy, err := policyFromProto(in.Policy)
	if err != nil {
		return project.Definition{}, err
	}
	return project.Definition{
		Version: in.Version, Name: strings.TrimSpace(in.Name), DisplayName: in.DisplayName,
		Description: in.Description, Policy: policy,
	}, nil
}

func definitionToProto(in project.Definition) *awmv1.ProjectDefinition {
	return &awmv1.ProjectDefinition{
		Version: in.Version, Name: in.Name, DisplayName: in.DisplayName,
		Description: in.Description, Policy: policyToProto(in.Policy),
	}
}

func summaryFromSnapshot(in project.Snapshot) project.ProjectSummary {
	return project.ProjectSummary{
		ProjectID: in.ProjectID, Title: in.Definition.Name, Description: in.Definition.Description,
		Revision: in.Revision, SourceRevision: in.SourceRevision,
		DefinitionURI:   fmt.Sprintf("project://registry/projects/%s/definition", url.PathEscape(string(in.ProjectID))),
		DiagnosticCount: len(in.Diagnostics),
	}
}

func summariesToProto(in []project.ProjectSummary) []*awmv1.ProjectSummary {
	out := make([]*awmv1.ProjectSummary, 0, len(in))
	for i := range in {
		out = append(out, summaryToProto(in[i]))
	}
	return out
}

func summaryToProto(in project.ProjectSummary) *awmv1.ProjectSummary {
	count := in.DiagnosticCount
	if count < 0 {
		count = 0
	}
	if count > math.MaxUint32 {
		count = math.MaxUint32
	}
	return &awmv1.ProjectSummary{
		ProjectId: string(in.ProjectID), Title: in.Title, Description: in.Description,
		Revision: string(in.Revision), SourceRevision: string(in.SourceRevision),
		DefinitionUri: in.DefinitionURI, DiagnosticCount: uint32(count), // #nosec G115 -- clamped above
	}
}

func sourcesToProto(in []project.Source) []*awmv1.Source {
	out := make([]*awmv1.Source, 0, len(in))
	for _, source := range in {
		kind := awmv1.Source_KIND_UNSPECIFIED
		switch source.Kind {
		case "user":
			kind = awmv1.Source_KIND_USER
		case "repository":
			kind = awmv1.Source_KIND_REPOSITORY
		}
		out = append(out, &awmv1.Source{Kind: kind, Uri: source.URI, Precedence: int32(source.Precedence)}) // #nosec G115 -- precedence is a small catalog layer index
	}
	return out
}

func diagnosticsToProto(in []project.Diagnostic) []*awmv1.Diagnostic {
	out := make([]*awmv1.Diagnostic, 0, len(in))
	for _, diagnostic := range in {
		severity := awmv1.Diagnostic_SEVERITY_UNSPECIFIED
		switch diagnostic.Severity {
		case "warning":
			severity = awmv1.Diagnostic_SEVERITY_WARNING
		case "error":
			severity = awmv1.Diagnostic_SEVERITY_ERROR
		}
		out = append(out, &awmv1.Diagnostic{
			Severity: severity, Code: diagnostic.Code, Message: diagnostic.Message,
			SourceUri: diagnostic.SourceURI, Path: diagnostic.Path,
		})
	}
	return out
}

func optionalURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid root_uri: %w", err)
	}
	return parsed, nil
}

func revisionURI(id project.ProjectID, revision project.Revision) string {
	return fmt.Sprintf("project://registry/projects/%s/revisions/%s", url.PathEscape(string(id)), url.PathEscape(string(revision)))
}

// --- Errors ---

func invalidArgument(err error) error {
	return detailError(codes.InvalidArgument, awmv1.ErrorCode_ERROR_CODE_INVALID_INPUT, err.Error())
}

func detailError(code codes.Code, detailCode awmv1.ErrorCode, message string) error {
	return statusWithDetail(code, message, &awmv1.ErrorDetail{Code: detailCode, Message: message})
}

func statusWithDetail(code codes.Code, message string, detail *awmv1.ErrorDetail) error {
	st := status.New(code, message)
	withDetail, err := st.WithDetails(detail)
	if err != nil {
		return st.Err()
	}
	return withDetail.Err()
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	if domain, ok := awm.AsError(err); ok {
		return awmRPCError(domain)
	}
	if domain, ok := project.AsError(err); ok {
		return projectRPCError(domain)
	}
	return detailError(codes.Internal, awmv1.ErrorCode_ERROR_CODE_INTERNAL, "internal error")
}

func awmRPCError(in *awm.Error) error {
	code := codes.Internal
	detailCode := awmv1.ErrorCode_ERROR_CODE_INTERNAL
	switch in.Code {
	case awm.CodeNotFound:
		code, detailCode = codes.NotFound, awmv1.ErrorCode_ERROR_CODE_NOT_FOUND
	case awm.CodeAlreadyExists:
		code, detailCode = codes.AlreadyExists, awmv1.ErrorCode_ERROR_CODE_ALREADY_EXISTS
	case awm.CodeInvalidInput:
		code, detailCode = codes.InvalidArgument, awmv1.ErrorCode_ERROR_CODE_INVALID_INPUT
	case awm.CodeInvalidReference:
		code, detailCode = codes.InvalidArgument, awmv1.ErrorCode_ERROR_CODE_INVALID_REFERENCE
	case awm.CodeInvalidTransition:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_INVALID_TRANSITION
	case awm.CodeReferenced:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_REFERENCED
	case awm.CodePolicyBroadening:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_POLICY_BROADENING
	case awm.CodeConflict:
		code, detailCode = codes.Aborted, awmv1.ErrorCode_ERROR_CODE_CONFLICT
	case awm.CodeMissingDefault:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_MISSING_DEFAULT_PROFILE
	case awm.CodeWriteDisabled:
		code, detailCode = codes.PermissionDenied, awmv1.ErrorCode_ERROR_CODE_WRITE_DISABLED
	case awm.CodeLockTimeout:
		code, detailCode = codes.DeadlineExceeded, awmv1.ErrorCode_ERROR_CODE_LOCK_TIMEOUT
	case awm.CodeUnavailable:
		code, detailCode = codes.Unavailable, awmv1.ErrorCode_ERROR_CODE_UNAVAILABLE
	}
	return statusWithDetail(code, in.Message, &awmv1.ErrorDetail{
		Code: detailCode, Message: in.Message, EntityKind: in.EntityKind, EntityId: in.EntityID,
		ProjectId: in.ProjectID, WorkProfileId: in.WorkProfileID, WorkSessionId: in.WorkSessionID,
		ResourceId: in.ResourceID, ResourceBindingId: in.ResourceBindingID,
		Expected: in.Expected, Current: in.Current,
	})
}

func projectRPCError(in *project.Error) error {
	code := codes.Internal
	detailCode := awmv1.ErrorCode_ERROR_CODE_INTERNAL
	switch in.Code {
	case project.CodeProjectNotFound:
		code, detailCode = codes.NotFound, awmv1.ErrorCode_ERROR_CODE_NOT_FOUND
	case project.CodeProjectAlreadyExists:
		code, detailCode = codes.AlreadyExists, awmv1.ErrorCode_ERROR_CODE_ALREADY_EXISTS
	case project.CodeInvalidDefinition:
		code, detailCode = codes.InvalidArgument, awmv1.ErrorCode_ERROR_CODE_INVALID_DEFINITION
	case project.CodeRevisionConflict:
		code, detailCode = codes.Aborted, awmv1.ErrorCode_ERROR_CODE_REVISION_CONFLICT
	case project.CodeAmbiguousProject:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_AMBIGUOUS_PROJECT
	case project.CodeInvalidRoot:
		code, detailCode = codes.InvalidArgument, awmv1.ErrorCode_ERROR_CODE_INVALID_ROOT
	case project.CodeRootProjectMismatch:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_ROOT_PROJECT_MISMATCH
	case project.CodeWriteDisabled:
		code, detailCode = codes.PermissionDenied, awmv1.ErrorCode_ERROR_CODE_WRITE_DISABLED
	case project.CodePermissionDenied:
		code, detailCode = codes.PermissionDenied, awmv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED
	case project.CodeLockTimeout:
		code, detailCode = codes.DeadlineExceeded, awmv1.ErrorCode_ERROR_CODE_LOCK_TIMEOUT
	case project.CodeReferenced:
		code, detailCode = codes.FailedPrecondition, awmv1.ErrorCode_ERROR_CODE_REFERENCED
	}
	return statusWithDetail(code, in.Message, &awmv1.ErrorDetail{
		Code: detailCode, Message: in.Message, ProjectId: string(in.ProjectID),
		Expected: string(in.ExpectedSourceRevision), Current: string(in.CurrentSourceRevision),
		RootUri: in.RootURI, Diagnostics: diagnosticsToProto(in.Diagnostics),
	})
}
