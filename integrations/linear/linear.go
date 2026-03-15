package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
)

var graphqlURL = "https://api.linear.app/graphql"

// setGraphqlURL overrides the GraphQL endpoint (tests only).
func setGraphqlURL(url string) { graphqlURL = url }

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*linear)(nil)
	_ mcp.FieldCompactionIntegration = (*linear)(nil)
)

type linear struct {
	apiKey     string
	authHeader string
	client     *http.Client

	mcpServerURL string
	remote       mcp.Integration
	useRemote    bool
}

// New creates a Linear integration. If mcpServerURL is non-empty, the
// integration supports dual-mode: remote MCP (via OAuth) or native API key.
func New(mcpServerURL ...string) mcp.Integration {
	l := &linear{client: &http.Client{}}
	if len(mcpServerURL) > 0 && mcpServerURL[0] != "" {
		l.mcpServerURL = mcpServerURL[0]
	}
	return l
}

// IsRemoteMCP reports whether this integration is currently operating in
// remote MCP mode (as opposed to native API key mode).
func IsRemoteMCP(i mcp.Integration) bool {
	if l, ok := i.(*linear); ok {
		return l.useRemote
	}
	return false
}

// MCPServerURL returns the configured remote MCP server URL, or empty.
func MCPServerURL(i mcp.Integration) string {
	if l, ok := i.(*linear); ok {
		return l.mcpServerURL
	}
	return ""
}

func newRemote(serverURL string) mcp.Integration {
	return remotemcp.New("linear", serverURL)
}

func (l *linear) Name() string { return "linear" }

func (l *linear) Configure(ctx context.Context, creds mcp.Credentials) error {
	if token := creds["mcp_access_token"]; token != "" && l.mcpServerURL != "" {
		if l.remote == nil {
			l.remote = newRemote(l.mcpServerURL)
		}
		if err := l.remote.Configure(ctx, mcp.Credentials{"access_token": token}); err != nil {
			return err
		}
		l.useRemote = true
		return nil
	}

	l.useRemote = false
	l.apiKey = creds["api_key"]
	if l.apiKey == "" {
		return fmt.Errorf("linear: api_key or mcp_access_token is required")
	}
	if strings.HasPrefix(l.apiKey, "lin_api_") {
		l.authHeader = l.apiKey
	} else {
		l.authHeader = "Bearer " + l.apiKey
	}
	return nil
}

func (l *linear) Healthy(ctx context.Context) bool {
	if l.useRemote && l.remote != nil {
		return l.remote.Healthy(ctx)
	}
	_, err := l.gql(ctx, `{ viewer { id } }`, nil)
	return err == nil
}

func (l *linear) Tools() []mcp.ToolDefinition {
	if l.useRemote && l.remote != nil {
		return l.remote.Tools()
	}
	return tools
}

func (l *linear) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (l *linear) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	if l.useRemote && l.remote != nil {
		return l.remote.Execute(ctx, toolName, args)
	}
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, l, args)
}

// --- GraphQL helpers ---

type gqlError struct {
	Message    string         `json:"message"`
	Path       []string       `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

func (e gqlError) String() string {
	s := e.Message
	if len(e.Path) > 0 {
		s += " (path: " + strings.Join(e.Path, ".") + ")"
	}
	if code, ok := e.Extensions["code"]; ok {
		s += fmt.Sprintf(" [%v]", code)
	}
	return s
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

func (l *linear) gql(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	body := map[string]any{"query": query}
	if variables != nil {
		body["variables"] = variables
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphqlURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", l.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("linear API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("linear API error (%d): %s", resp.StatusCode, string(data))
	}

	var gqlResp gqlResponse
	if err := json.Unmarshal(data, &gqlResp); err != nil {
		return data, nil
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.String()
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}
	return gqlResp.Data, nil
}

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	if mcp.IsRetryable(err) {
		return nil, err
	}
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- arg helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func optInt(args map[string]any, key string, def int) int {
	if v := argInt(args, key); v > 0 {
		return v
	}
	return def
}

// resolveTeamID looks up a team by name or key and returns its UUID.
func (l *linear) resolveTeamID(ctx context.Context, nameOrKey string) (string, error) {
	data, err := l.gql(ctx, `query($filter: TeamFilter) {
		teams(filter: $filter) { nodes { id } }
	}`, map[string]any{
		"filter": map[string]any{"name": map[string]any{"eq": nameOrKey}},
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Teams struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Teams.Nodes) > 0 {
		return resp.Teams.Nodes[0].ID, nil
	}

	data, err = l.gql(ctx, `query($filter: TeamFilter) {
		teams(filter: $filter) { nodes { id } }
	}`, map[string]any{
		"filter": map[string]any{"key": map[string]any{"eq": nameOrKey}},
	})
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Teams.Nodes) > 0 {
		return resp.Teams.Nodes[0].ID, nil
	}
	return "", fmt.Errorf("team not found: %s", nameOrKey)
}

// resolveIssueTeamID fetches the team ID for an already-resolved issue UUID.
func (l *linear) resolveIssueTeamID(ctx context.Context, issueID string) (string, error) {
	data, err := l.gql(ctx, `query($id: String!) {
		issue(id: $id) { team { id } }
	}`, map[string]any{"id": issueID})
	if err != nil {
		return "", err
	}
	var resp struct {
		Issue struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || resp.Issue.Team.ID == "" {
		return "", fmt.Errorf("could not resolve team for issue: %s", issueID)
	}
	return resp.Issue.Team.ID, nil
}

// resolveStateID looks up a workflow state by name within a team and returns its UUID.
func (l *linear) resolveStateID(ctx context.Context, name, teamID string) (string, error) {
	filter := map[string]any{"name": map[string]any{"eqIgnoreCase": name}}
	if teamID != "" {
		filter["team"] = map[string]any{"id": map[string]any{"eq": teamID}}
	}
	data, err := l.gql(ctx, `query($filter: WorkflowStateFilter) {
		workflowStates(filter: $filter) { nodes { id } }
	}`, map[string]any{"filter": filter})
	if err != nil {
		return "", err
	}
	var resp struct {
		WorkflowStates struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.WorkflowStates.Nodes) > 0 {
		return resp.WorkflowStates.Nodes[0].ID, nil
	}
	return "", fmt.Errorf("workflow state not found: %s", name)
}

// resolveProjectID looks up a project by name and returns its UUID.
func (l *linear) resolveProjectID(ctx context.Context, name string) (string, error) {
	data, err := l.gql(ctx, `query($term: String!) {
		searchProjects(term: $term, first: 5) { nodes { id name } }
	}`, map[string]any{"term": name})
	if err != nil {
		return "", err
	}
	var resp struct {
		SearchProjects struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"searchProjects"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	for _, n := range resp.SearchProjects.Nodes {
		if strings.EqualFold(n.Name, name) {
			return n.ID, nil
		}
	}
	if len(resp.SearchProjects.Nodes) > 0 {
		return resp.SearchProjects.Nodes[0].ID, nil
	}
	return "", fmt.Errorf("project not found: %s", name)
}

// resolveUserID looks up a user by name or email and returns their UUID.
func (l *linear) resolveUserID(ctx context.Context, nameOrEmail string) (string, error) {
	data, err := l.gql(ctx, `query($filter: UserFilter) {
		users(filter: $filter) { nodes { id } }
	}`, map[string]any{
		"filter": map[string]any{"name": map[string]any{"eqIgnoreCase": nameOrEmail}},
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Users struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Users.Nodes) > 0 {
		return resp.Users.Nodes[0].ID, nil
	}

	data, err = l.gql(ctx, `query($filter: UserFilter) {
		users(filter: $filter) { nodes { id } }
	}`, map[string]any{
		"filter": map[string]any{"email": map[string]any{"eqIgnoreCase": nameOrEmail}},
	})
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Users.Nodes) > 0 {
		return resp.Users.Nodes[0].ID, nil
	}
	return "", fmt.Errorf("user not found: %s", nameOrEmail)
}

// resolveLabelIDs looks up labels by name in a single batched query and returns their UUIDs.
func (l *linear) resolveLabelIDs(ctx context.Context, names []string) ([]string, error) {
	var cleaned []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			cleaned = append(cleaned, name)
		}
	}
	if len(cleaned) == 0 {
		return nil, nil
	}

	orFilters := make([]map[string]any, len(cleaned))
	for i, name := range cleaned {
		orFilters[i] = map[string]any{"name": map[string]any{"eqIgnoreCase": name}}
	}

	data, err := l.gql(ctx, `query($filter: IssueLabelFilter) {
		issueLabels(filter: $filter) { nodes { id name } }
	}`, map[string]any{
		"filter": map[string]any{"or": orFilters},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		IssueLabels struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	nameToID := make(map[string]string, len(resp.IssueLabels.Nodes))
	for _, n := range resp.IssueLabels.Nodes {
		nameToID[strings.ToLower(n.Name)] = n.ID
	}

	ids := make([]string, 0, len(cleaned))
	for _, name := range cleaned {
		id, ok := nameToID[strings.ToLower(name)]
		if !ok {
			return nil, fmt.Errorf("label not found: %s", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type handlerFunc func(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	// Issues
	"linear_list_issues":           listIssues,
	"linear_search_issues":         searchIssues,
	"linear_get_issue":             getIssue,
	"linear_create_issue":          createIssue,
	"linear_update_issue":          updateIssue,
	"linear_archive_issue":         archiveIssue,
	"linear_unarchive_issue":       unarchiveIssue,
	"linear_list_issue_comments":   listIssueComments,
	"linear_create_comment":        createComment,
	"linear_update_comment":        updateComment,
	"linear_delete_comment":        deleteComment,
	"linear_list_issue_relations":  listIssueRelations,
	"linear_create_issue_relation": createIssueRelation,
	"linear_delete_issue_relation": deleteIssueRelation,
	"linear_list_issue_labels":     listIssueLabels,
	"linear_list_attachments":      listAttachments,
	"linear_create_attachment":     createAttachment,

	// Projects
	"linear_list_projects":            listProjects,
	"linear_search_projects":          searchProjects,
	"linear_get_project":              getProject,
	"linear_create_project":           createProject,
	"linear_update_project":           updateProject,
	"linear_archive_project":          archiveProject,
	"linear_list_project_updates":     listProjectUpdates,
	"linear_create_project_update":    createProjectUpdate,
	"linear_list_project_milestones":  listProjectMilestones,
	"linear_create_project_milestone": createProjectMilestone,

	// Cycles
	"linear_list_cycles":  listCycles,
	"linear_get_cycle":    getCycle,
	"linear_create_cycle": createCycle,
	"linear_update_cycle": updateCycle,

	// Teams
	"linear_list_teams": listTeams,
	"linear_get_team":   getTeam,

	// Users
	"linear_viewer":     viewer,
	"linear_list_users": listUsers,
	"linear_get_user":   getUser,

	// Labels
	"linear_list_labels":  listLabels,
	"linear_create_label": createLabel,
	"linear_update_label": updateLabel,
	"linear_delete_label": deleteLabel,

	// Workflow States
	"linear_list_workflow_states":  listWorkflowStates,
	"linear_create_workflow_state": createWorkflowState,

	// Documents
	"linear_list_documents":   listDocuments,
	"linear_search_documents": searchDocuments,
	"linear_get_document":     getDocument,
	"linear_create_document":  createDocument,
	"linear_update_document":  updateDocument,

	// Initiatives
	"linear_list_initiatives":  listInitiatives,
	"linear_get_initiative":    getInitiative,
	"linear_create_initiative": createInitiative,
	"linear_update_initiative": updateInitiative,

	// Favorites
	"linear_list_favorites":  listFavorites,
	"linear_create_favorite": createFavorite,
	"linear_delete_favorite": deleteFavorite,

	// Webhooks
	"linear_list_webhooks":  listWebhooks,
	"linear_create_webhook": createWebhook,
	"linear_delete_webhook": deleteWebhook,

	// Notifications
	"linear_list_notifications": listNotifications,

	// Templates
	"linear_list_templates": listTemplates,

	// Organization
	"linear_get_organization": getOrganization,

	// Custom Views
	"linear_list_custom_views":  listCustomViews,
	"linear_create_custom_view": createCustomView,

	// Rate Limit
	"linear_rate_limit": rateLimitStatus,
}
