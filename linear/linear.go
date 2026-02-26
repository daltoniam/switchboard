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
)

const graphqlURL = "https://api.linear.app/graphql"

type linear struct {
	apiKey string
	client *http.Client
}

func New() mcp.Integration {
	return &linear{client: &http.Client{}}
}

func (l *linear) Name() string { return "linear" }

func (l *linear) Configure(creds mcp.Credentials) error {
	l.apiKey = creds["api_key"]
	if l.apiKey == "" {
		return fmt.Errorf("linear: api_key is required")
	}
	return nil
}

func (l *linear) Healthy(ctx context.Context) bool {
	_, err := l.gql(ctx, `{ viewer { id } }`, nil)
	return err == nil
}

func (l *linear) Tools() []mcp.ToolDefinition {
	return tools
}

func (l *linear) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, l, args)
}

// --- GraphQL helpers ---

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
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
	req.Header.Set("Authorization", l.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
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
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}
	return gqlResp.Data, nil
}

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
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

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
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
			Nodes []struct{ ID string `json:"id"` } `json:"nodes"`
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
