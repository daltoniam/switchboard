package awmgrpc

import (
	"context"
	"net"
	"testing"

	"github.com/daltoniam/switchboard/awm"
	awmv1 "github.com/daltoniam/switchboard/gen/awm/v1"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newTestClient(t *testing.T, writes bool) (awmv1.ProjectCatalogServiceClient, awmv1.WorkProfileServiceClient, awmv1.AgentProfileServiceClient, awmv1.WorkSessionServiceClient) {
	t.Helper()
	root := t.TempDir()
	catalog := project.NewStore(root)
	require.NoError(t, catalog.Load())
	work := awm.NewStore(root)
	work.SetCatalog(catalog)
	catalog.SetDeleteGuard(work)

	listener := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	Register(srv, catalog, catalog, catalog, work, Options{CatalogEnabled: true, WritesEnabled: writes})
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return awmv1.NewProjectCatalogServiceClient(conn), awmv1.NewWorkProfileServiceClient(conn), awmv1.NewAgentProfileServiceClient(conn), awmv1.NewWorkSessionServiceClient(conn)
}

func TestGRPC_AWMHandlersShareTypedLifecycle(t *testing.T) {
	projects, profiles, agents, sessions := newTestClient(t, true)
	ctx := context.Background()

	created, err := projects.CreateProject(ctx, &awmv1.CreateProjectRequest{Definition: &awmv1.ProjectDefinition{
		Version: "1", Name: "switchboard", Description: "Switchboard",
	}})
	require.NoError(t, err)
	assert.Equal(t, "switchboard", created.Project.ProjectId)

	_, err = profiles.PutWorkProfile(ctx, &awmv1.PutWorkProfileRequest{WorkProfile: &awmv1.WorkProfile{
		Version: "1", WorkProfileId: "review", ProjectIds: []string{"switchboard"},
		DefaultPolicy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{{Capability: "network", Allowed: false}}},
	}})
	require.NoError(t, err)

	_, err = agents.PutAgentProfile(ctx, &awmv1.PutAgentProfileRequest{AgentProfile: &awmv1.AgentProfile{
		Version: "1", AgentProfileId: "reviewer", Capabilities: []string{"review"},
		Constraints: &awmv1.AgentConstraints{ProjectIds: []string{"switchboard"}},
	}})
	require.NoError(t, err)

	createdSession, err := sessions.CreateWorkSession(ctx, &awmv1.CreateWorkSessionRequest{WorkSession: &awmv1.WorkSession{
		Version: "1", WorkSessionId: "ws-1", ProjectId: "switchboard", WorkProfileId: "review",
		AgentProfileIds: []string{"reviewer"}, State: awmv1.WorkSessionState_WORK_SESSION_STATE_PROPOSED,
		Policy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{{Capability: "network", Allowed: false}}},
	}})
	require.NoError(t, err)
	assert.NotEmpty(t, createdSession.WorkSession.ProjectRevision)
	assert.NotEmpty(t, createdSession.WorkSession.ProjectSnapshotId)

	opened, err := sessions.TransitionWorkSession(ctx, &awmv1.TransitionWorkSessionRequest{
		WorkSessionId: "ws-1", State: awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN,
	})
	require.NoError(t, err)
	assert.Equal(t, awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN, opened.WorkSession.State)

	listed, err := sessions.ListWorkSessions(ctx, &awmv1.ListWorkSessionsRequest{
		State: awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN, ProjectId: "switchboard",
	})
	require.NoError(t, err)
	require.Len(t, listed.WorkSessions, 1)
	assert.Equal(t, "ws-1", listed.WorkSessions[0].WorkSessionId)

	profileList, err := profiles.ListWorkProfiles(ctx, &awmv1.ListWorkProfilesRequest{})
	require.NoError(t, err)
	require.Len(t, profileList.WorkProfiles, 1)
}

func newTestResourceClients(t *testing.T, writes bool) (awmv1.ResourceServiceClient, awmv1.ResourceBindingServiceClient, awmv1.WorkSessionServiceClient) {
	t.Helper()
	root := t.TempDir()
	catalog := project.NewStore(root)
	require.NoError(t, catalog.Load())
	work := awm.NewStore(root)
	work.SetCatalog(catalog)

	listener := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	Register(srv, catalog, catalog, catalog, work, Options{CatalogEnabled: true, WritesEnabled: writes})
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return awmv1.NewResourceServiceClient(conn), awmv1.NewResourceBindingServiceClient(conn), awmv1.NewWorkSessionServiceClient(conn)
}

func TestGRPC_ResourceBindingLifecycle(t *testing.T) {
	resources, bindings, sessions := newTestResourceClients(t, true)
	ctx := context.Background()

	put, err := resources.PutResource(ctx, &awmv1.PutResourceRequest{Resource: &awmv1.Resource{
		Version: "1", ResourceId: "repo", Uri: "file:///work/repo", Kind: "git-repository",
	}})
	require.NoError(t, err)
	assert.Equal(t, "repo", put.Resource.ResourceId)
	_, err = sessions.CreateWorkSession(ctx, &awmv1.CreateWorkSessionRequest{WorkSession: &awmv1.WorkSession{
		Version: "1", WorkSessionId: "ws", State: awmv1.WorkSessionState_WORK_SESSION_STATE_OPEN,
	}})
	require.NoError(t, err)

	created, err := bindings.CreateResourceBinding(ctx, &awmv1.CreateResourceBindingRequest{ResourceBinding: &awmv1.ResourceBinding{
		Version: "1", ResourceBindingId: "binding", WorkSessionId: "ws", ResourceId: "repo",
		ResolvedLocator: "file:///work/repo", State: awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_PROPOSED,
	}})
	require.NoError(t, err)
	assert.Equal(t, "binding", created.ResourceBinding.ResourceBindingId)

	bound, err := bindings.TransitionResourceBinding(ctx, &awmv1.TransitionResourceBindingRequest{
		ResourceBindingId: "binding", State: awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_BOUND,
	})
	require.NoError(t, err)
	assert.Equal(t, awmv1.ResourceBindingState_RESOURCE_BINDING_STATE_BOUND, bound.ResourceBinding.State)

	listed, err := bindings.ListResourceBindings(ctx, &awmv1.ListResourceBindingsRequest{WorkSessionId: "ws"})
	require.NoError(t, err)
	require.Len(t, listed.ResourceBindings, 1)
	gotBinding, err := bindings.GetResourceBinding(ctx, &awmv1.GetResourceBindingRequest{ResourceBindingId: "binding"})
	require.NoError(t, err)
	assert.Equal(t, "repo", gotBinding.ResourceBinding.ResourceId)
	locator := "file:///work/repo/worktree"
	patched, err := bindings.PatchResourceBinding(ctx, &awmv1.PatchResourceBindingRequest{
		ResourceBindingId: "binding", ResolvedLocator: &locator,
	})
	require.NoError(t, err)
	assert.Equal(t, locator, patched.ResourceBinding.ResolvedLocator)

	resourceList, err := resources.ListResources(ctx, &awmv1.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, resourceList.Resources, 1)
	got, err := resources.GetResource(ctx, &awmv1.GetResourceRequest{ResourceId: "repo"})
	require.NoError(t, err)
	assert.Equal(t, "file:///work/repo", got.Resource.Uri)

	_, err = resources.DeleteResource(ctx, &awmv1.DeleteResourceRequest{ResourceId: "repo"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = bindings.DeleteResourceBinding(ctx, &awmv1.DeleteResourceBindingRequest{ResourceBindingId: "binding"})
	require.NoError(t, err)
	_, err = resources.DeleteResource(ctx, &awmv1.DeleteResourceRequest{ResourceId: "repo"})
	require.NoError(t, err)
}

func TestGRPC_ProjectCatalogHandlers(t *testing.T) {
	projects, _, _, _ := newTestClient(t, true)
	ctx := context.Background()

	created, err := projects.CreateProject(ctx, &awmv1.CreateProjectRequest{Definition: &awmv1.ProjectDefinition{
		Version: "1", Name: "typed", DisplayName: "Typed project", Description: "before",
		Policy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{{Capability: "network", Allowed: false}}},
	}})
	require.NoError(t, err)

	listed, err := projects.ListProjects(ctx, &awmv1.ListProjectsRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Projects, 1)
	searched, err := projects.SearchProjects(ctx, &awmv1.SearchProjectsRequest{Query: "typed"})
	require.NoError(t, err)
	require.Len(t, searched.Projects, 1)

	got, err := projects.GetProject(ctx, &awmv1.GetProjectRequest{ProjectId: "typed"})
	require.NoError(t, err)
	assert.Equal(t, "before", got.Definition.Description)
	assert.Equal(t, "Typed project", got.Definition.DisplayName)
	require.Len(t, got.Definition.Policy.Capabilities, 1)

	validated, err := projects.ValidateProject(ctx, &awmv1.ValidateProjectRequest{Definition: got.Definition})
	require.NoError(t, err)
	assert.True(t, validated.Valid)

	revision, err := projects.GetProjectRevision(ctx, &awmv1.GetProjectRevisionRequest{
		ProjectId: "typed", Revision: created.Project.Revision,
	})
	require.NoError(t, err)
	assert.Equal(t, "before", revision.Definition.Description)

	diagnostics, err := projects.GetProjectDiagnostics(ctx, &awmv1.GetProjectDiagnosticsRequest{ProjectId: "typed"})
	require.NoError(t, err)
	assert.Empty(t, diagnostics.Diagnostics)

	updated, err := projects.UpdateProject(ctx, &awmv1.UpdateProjectRequest{
		ProjectId: "typed", ExpectedSourceRevision: created.Project.SourceRevision,
		Definition: &awmv1.ProjectDefinition{
			Version: "1", Name: "typed", DisplayName: "Typed project", Description: "updated",
			Policy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{{Capability: "network", Allowed: false}}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Project.Description)

	empty := ""
	patched, err := projects.PatchProject(ctx, &awmv1.PatchProjectRequest{
		ProjectId: "typed", ExpectedSourceRevision: updated.Project.SourceRevision,
		Patch: &awmv1.ProjectPatch{Description: &empty},
	})
	require.NoError(t, err)
	assert.Empty(t, patched.Project.Description)

	resolved, err := projects.ResolveProject(ctx, &awmv1.ResolveProjectRequest{ProjectId: "typed"})
	require.NoError(t, err)
	assert.Equal(t, "typed", resolved.ProjectId)

	deleted, err := projects.DeleteProject(ctx, &awmv1.DeleteProjectRequest{
		ProjectId: "typed", ExpectedSourceRevision: patched.Project.SourceRevision,
	})
	require.NoError(t, err)
	assert.True(t, deleted.Deleted)
}

func TestGRPC_WorkModelCRUDHandlers(t *testing.T) {
	_, profiles, agents, sessions := newTestClient(t, true)
	ctx := context.Background()

	_, err := profiles.PutWorkProfile(ctx, &awmv1.PutWorkProfileRequest{WorkProfile: &awmv1.WorkProfile{Version: "1", WorkProfileId: "temporary"}})
	require.NoError(t, err)
	gotProfile, err := profiles.GetWorkProfile(ctx, &awmv1.GetWorkProfileRequest{WorkProfileId: "temporary"})
	require.NoError(t, err)
	assert.Equal(t, "temporary", gotProfile.WorkProfile.WorkProfileId)
	_, err = profiles.DeleteWorkProfile(ctx, &awmv1.DeleteWorkProfileRequest{WorkProfileId: "temporary"})
	require.NoError(t, err)

	_, err = agents.PutAgentProfile(ctx, &awmv1.PutAgentProfileRequest{AgentProfile: &awmv1.AgentProfile{Version: "1", AgentProfileId: "temporary"}})
	require.NoError(t, err)
	gotAgent, err := agents.GetAgentProfile(ctx, &awmv1.GetAgentProfileRequest{AgentProfileId: "temporary"})
	require.NoError(t, err)
	assert.Equal(t, "temporary", gotAgent.AgentProfile.AgentProfileId)
	_, err = agents.DeleteAgentProfile(ctx, &awmv1.DeleteAgentProfileRequest{AgentProfileId: "temporary"})
	require.NoError(t, err)

	_, err = sessions.CreateWorkSession(ctx, &awmv1.CreateWorkSessionRequest{WorkSession: &awmv1.WorkSession{
		Version: "1", WorkSessionId: "standalone", State: awmv1.WorkSessionState_WORK_SESSION_STATE_PROPOSED,
	}})
	require.NoError(t, err)
	gotSession, err := sessions.GetWorkSession(ctx, &awmv1.GetWorkSessionRequest{WorkSessionId: "standalone"})
	require.NoError(t, err)
	assert.Equal(t, "standalone", gotSession.WorkSession.WorkSessionId)

	displayName := "patched"
	patched, err := sessions.PatchWorkSession(ctx, &awmv1.PatchWorkSessionRequest{
		WorkSessionId: "standalone", DisplayName: &displayName,
		Policy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{{Capability: "network", Allowed: false}}},
	})
	require.NoError(t, err)
	assert.Equal(t, "patched", patched.WorkSession.DisplayName)

	_, err = sessions.TransitionWorkSession(ctx, &awmv1.TransitionWorkSessionRequest{
		WorkSessionId: "standalone", State: awmv1.WorkSessionState_WORK_SESSION_STATE_ABORTED,
	})
	require.NoError(t, err)
	deleted, err := sessions.DeleteWorkSession(ctx, &awmv1.DeleteWorkSessionRequest{WorkSessionId: "standalone"})
	require.NoError(t, err)
	assert.True(t, deleted.Deleted)
}

func TestGRPC_WriteDisabledReturnsTypedStatusDetail(t *testing.T) {
	projects, _, _, _ := newTestClient(t, false)
	_, err := projects.CreateProject(context.Background(), &awmv1.CreateProjectRequest{Definition: &awmv1.ProjectDefinition{Version: "1", Name: "p"}})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Len(t, st.Details(), 1)
	detail, ok := st.Details()[0].(*awmv1.ErrorDetail)
	require.True(t, ok)
	assert.Equal(t, awmv1.ErrorCode_ERROR_CODE_WRITE_DISABLED, detail.Code)
}

func TestGRPC_RejectsDuplicatePolicyCapabilities(t *testing.T) {
	_, profiles, _, _ := newTestClient(t, true)
	_, err := profiles.PutWorkProfile(context.Background(), &awmv1.PutWorkProfileRequest{WorkProfile: &awmv1.WorkProfile{
		Version: "1", WorkProfileId: "bad", DefaultPolicy: &awmv1.PolicyDocument{Capabilities: []*awmv1.CapabilityPolicy{
			{Capability: "network", Allowed: true}, {Capability: "network", Allowed: false},
		}},
	}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
