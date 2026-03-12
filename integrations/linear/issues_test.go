package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLinear creates a *linear pointing at a test server.
func newTestLinear(handler http.Handler) (*linear, *httptest.Server) {
	ts := httptest.NewServer(handler)
	return &linear{
		apiKey:     "test-key",
		authHeader: "test-key",
		client:     ts.Client(),
	}, ts
}

// gqlHandler returns an http.HandlerFunc that routes GraphQL queries to
// response functions. It decodes the request body, passes the query and
// variables to the matcher func, and writes the returned data.
func gqlHandler(fn func(query string, vars map[string]any) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		data := fn(body.Query, body.Variables)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}
}

func TestResolveStateID(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		teamID  string
		resp    any
		wantID  string
		wantErr string
	}{
		{
			name:   "found by name",
			state:  "In Progress",
			teamID: "team-1",
			resp: map[string]any{
				"workflowStates": map[string]any{
					"nodes": []map[string]any{{"id": "state-123"}},
				},
			},
			wantID: "state-123",
		},
		{
			name:   "without team filter",
			state:  "Done",
			teamID: "",
			resp: map[string]any{
				"workflowStates": map[string]any{
					"nodes": []map[string]any{{"id": "state-456"}},
				},
			},
			wantID: "state-456",
		},
		{
			name:   "not found",
			state:  "NonExistent",
			teamID: "",
			resp: map[string]any{
				"workflowStates": map[string]any{
					"nodes": []map[string]any{},
				},
			},
			wantErr: "workflow state not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, ts := newTestLinear(gqlHandler(func(query string, vars map[string]any) any {
				filter, _ := vars["filter"].(map[string]any)
				require.NotNil(t, filter)
				assert.Contains(t, filter, "name")
				if tt.teamID != "" {
					assert.Contains(t, filter, "team")
				}
				return tt.resp
			}))
			defer ts.Close()
			// Override graphqlURL by using the gql method through the test server
			// We need to temporarily redirect - use a custom approach
			origURL := graphqlURL
			setGraphqlURL(ts.URL)
			defer setGraphqlURL(origURL)

			id, err := l.resolveStateID(context.Background(), tt.state, tt.teamID)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestResolveProjectID(t *testing.T) {
	tests := []struct {
		name    string
		project string
		resp    any
		wantID  string
		wantErr string
	}{
		{
			name:    "exact match",
			project: "My Project",
			resp: map[string]any{
				"searchProjects": map[string]any{
					"nodes": []map[string]any{
						{"id": "proj-1", "name": "My Project"},
						{"id": "proj-2", "name": "My Project Extra"},
					},
				},
			},
			wantID: "proj-1",
		},
		{
			name:    "case insensitive match",
			project: "my project",
			resp: map[string]any{
				"searchProjects": map[string]any{
					"nodes": []map[string]any{
						{"id": "proj-1", "name": "My Project"},
					},
				},
			},
			wantID: "proj-1",
		},
		{
			name:    "fallback to first result",
			project: "Project",
			resp: map[string]any{
				"searchProjects": map[string]any{
					"nodes": []map[string]any{
						{"id": "proj-99", "name": "Project Alpha"},
					},
				},
			},
			wantID: "proj-99",
		},
		{
			name:    "not found",
			project: "NonExistent",
			resp: map[string]any{
				"searchProjects": map[string]any{
					"nodes": []map[string]any{},
				},
			},
			wantErr: "project not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, ts := newTestLinear(gqlHandler(func(_ string, _ map[string]any) any {
				return tt.resp
			}))
			defer ts.Close()
			origURL := graphqlURL
			setGraphqlURL(ts.URL)
			defer setGraphqlURL(origURL)

			id, err := l.resolveProjectID(context.Background(), tt.project)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestResolveUserID(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		byName  any
		byEmail any
		wantID  string
		wantErr string
	}{
		{
			name: "found by name",
			user: "John Doe",
			byName: map[string]any{
				"users": map[string]any{
					"nodes": []map[string]any{{"id": "user-1"}},
				},
			},
			wantID: "user-1",
		},
		{
			name: "found by email fallback",
			user: "john@example.com",
			byName: map[string]any{
				"users": map[string]any{"nodes": []map[string]any{}},
			},
			byEmail: map[string]any{
				"users": map[string]any{
					"nodes": []map[string]any{{"id": "user-2"}},
				},
			},
			wantID: "user-2",
		},
		{
			name: "not found",
			user: "nobody",
			byName: map[string]any{
				"users": map[string]any{"nodes": []map[string]any{}},
			},
			byEmail: map[string]any{
				"users": map[string]any{"nodes": []map[string]any{}},
			},
			wantErr: "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			l, ts := newTestLinear(gqlHandler(func(_ string, vars map[string]any) any {
				callCount++
				if callCount == 1 {
					return tt.byName
				}
				return tt.byEmail
			}))
			defer ts.Close()
			origURL := graphqlURL
			setGraphqlURL(ts.URL)
			defer setGraphqlURL(origURL)

			id, err := l.resolveUserID(context.Background(), tt.user)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestResolveLabelIDs(t *testing.T) {
	tests := []struct {
		name    string
		labels  []string
		resp    func(string, map[string]any) any
		wantIDs []string
		wantErr string
	}{
		{
			name:   "single label",
			labels: []string{"Bug"},
			resp: func(_ string, _ map[string]any) any {
				return map[string]any{
					"issueLabels": map[string]any{
						"nodes": []map[string]any{{"id": "label-1"}},
					},
				}
			},
			wantIDs: []string{"label-1"},
		},
		{
			name:   "multiple labels",
			labels: []string{"Bug", "Feature"},
			resp: func() func(string, map[string]any) any {
				call := 0
				return func(_ string, _ map[string]any) any {
					call++
					id := "label-1"
					if call == 2 {
						id = "label-2"
					}
					return map[string]any{
						"issueLabels": map[string]any{
							"nodes": []map[string]any{{"id": id}},
						},
					}
				}
			}(),
			wantIDs: []string{"label-1", "label-2"},
		},
		{
			name:   "label not found",
			labels: []string{"NonExistent"},
			resp: func(_ string, _ map[string]any) any {
				return map[string]any{
					"issueLabels": map[string]any{
						"nodes": []map[string]any{},
					},
				}
			},
			wantErr: "label not found",
		},
		{
			name:    "empty names skipped",
			labels:  []string{"", "  "},
			resp:    func(_ string, _ map[string]any) any { return nil },
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, ts := newTestLinear(gqlHandler(tt.resp))
			defer ts.Close()
			origURL := graphqlURL
			setGraphqlURL(ts.URL)
			defer setGraphqlURL(origURL)

			ids, err := l.resolveLabelIDs(context.Background(), tt.labels)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantIDs, ids)
			}
		})
	}
}

func TestUpdateIssue_SetsTeamStateProjectAssigneeLabels(t *testing.T) {
	var capturedInput map[string]any
	callCount := 0

	l, ts := newTestLinear(gqlHandler(func(query string, vars map[string]any) any {
		callCount++
		switch callCount {
		case 1: // resolveIssueID
			return map[string]any{"issue": map[string]any{"id": "issue-uuid"}}
		case 2: // resolveTeamID (by name)
			return map[string]any{"teams": map[string]any{"nodes": []map[string]any{{"id": "team-uuid"}}}}
		case 3: // resolveStateID
			return map[string]any{"workflowStates": map[string]any{"nodes": []map[string]any{{"id": "state-uuid"}}}}
		case 4: // resolveProjectID
			return map[string]any{"searchProjects": map[string]any{"nodes": []map[string]any{{"id": "proj-uuid", "name": "My Project"}}}}
		case 5: // resolveUserID (by name)
			return map[string]any{"users": map[string]any{"nodes": []map[string]any{{"id": "user-uuid"}}}}
		case 6: // resolveLabelIDs
			return map[string]any{"issueLabels": map[string]any{"nodes": []map[string]any{{"id": "label-uuid"}}}}
		default: // final issueUpdate mutation
			capturedInput, _ = vars["input"].(map[string]any)
			return map[string]any{
				"issueUpdate": map[string]any{
					"issue": map[string]any{"id": "issue-uuid", "title": "Updated"},
				},
			}
		}
	}))
	defer ts.Close()
	origURL := graphqlURL
	setGraphqlURL(ts.URL)
	defer setGraphqlURL(origURL)

	result, err := updateIssue(context.Background(), l, map[string]any{
		"id":       "ENG-123",
		"title":    "Updated",
		"team":     "Engineering",
		"state":    "In Progress",
		"project":  "My Project",
		"assignee": "John Doe",
		"labels":   "Bug",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	assert.Equal(t, "Updated", capturedInput["title"])
	assert.Equal(t, "team-uuid", capturedInput["teamId"])
	assert.Equal(t, "state-uuid", capturedInput["stateId"])
	assert.Equal(t, "proj-uuid", capturedInput["projectId"])
	assert.Equal(t, "user-uuid", capturedInput["assigneeId"])
	assert.Equal(t, []any{"label-uuid"}, capturedInput["labelIds"])
}

func TestUpdateIssue_OnlyBasicFields(t *testing.T) {
	var capturedInput map[string]any
	callCount := 0

	l, ts := newTestLinear(gqlHandler(func(_ string, vars map[string]any) any {
		callCount++
		if callCount == 1 {
			return map[string]any{"issue": map[string]any{"id": "issue-uuid"}}
		}
		capturedInput, _ = vars["input"].(map[string]any)
		return map[string]any{
			"issueUpdate": map[string]any{
				"issue": map[string]any{"id": "issue-uuid", "title": "New Title"},
			},
		}
	}))
	defer ts.Close()
	origURL := graphqlURL
	setGraphqlURL(ts.URL)
	defer setGraphqlURL(origURL)

	result, err := updateIssue(context.Background(), l, map[string]any{
		"id":    "ENG-123",
		"title": "New Title",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "New Title", capturedInput["title"])
	assert.NotContains(t, capturedInput, "teamId")
	assert.NotContains(t, capturedInput, "stateId")
	assert.NotContains(t, capturedInput, "projectId")
	assert.NotContains(t, capturedInput, "assigneeId")
	assert.NotContains(t, capturedInput, "labelIds")
}

func TestUpdateIssue_StateResolvesFromIssueTeam(t *testing.T) {
	var capturedInput map[string]any
	callCount := 0

	l, ts := newTestLinear(gqlHandler(func(_ string, vars map[string]any) any {
		callCount++
		switch callCount {
		case 1: // resolveIssueID
			return map[string]any{"issue": map[string]any{"id": "issue-uuid"}}
		case 2: // resolveIssueTeamID
			return map[string]any{"issue": map[string]any{"team": map[string]any{"id": "dest-team-uuid"}}}
		case 3: // resolveStateID (should filter by dest-team-uuid)
			filter, _ := vars["filter"].(map[string]any)
			team, _ := filter["team"].(map[string]any)
			idFilter, _ := team["id"].(map[string]any)
			assert.Equal(t, "dest-team-uuid", idFilter["eq"])
			return map[string]any{"workflowStates": map[string]any{"nodes": []map[string]any{{"id": "state-uuid"}}}}
		default: // issueUpdate mutation
			capturedInput, _ = vars["input"].(map[string]any)
			return map[string]any{
				"issueUpdate": map[string]any{
					"issue": map[string]any{"id": "issue-uuid"},
				},
			}
		}
	}))
	defer ts.Close()
	origURL := graphqlURL
	setGraphqlURL(ts.URL)
	defer setGraphqlURL(origURL)

	result, err := updateIssue(context.Background(), l, map[string]any{
		"id":    "DEST-339",
		"state": "In Progress",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "state-uuid", capturedInput["stateId"])
	assert.NotContains(t, capturedInput, "teamId")
}
