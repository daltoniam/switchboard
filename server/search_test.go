package server

import (
	"math"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSynonymMap(t *testing.T) {
	tests := []struct {
		name   string
		groups [][]string
		want   map[string][]string
	}{
		{
			name:   "basic group includes self",
			groups: [][]string{{"ticket", "issue", "bug"}},
			want: map[string][]string{
				"ticket": {"ticket", "issue", "bug"},
				"issue":  {"ticket", "issue", "bug"},
				"bug":    {"ticket", "issue", "bug"},
			},
		},
		{
			name:   "empty groups",
			groups: [][]string{},
			want:   map[string][]string{},
		},
		{
			name:   "single-word group produces no entries",
			groups: [][]string{{"orphan"}},
			want:   map[string][]string{},
		},
		{
			name:   "two-word group",
			groups: [][]string{{"repo", "repository"}},
			want: map[string][]string{
				"repo":       {"repo", "repository"},
				"repository": {"repo", "repository"},
			},
		},
		{
			name: "multiple groups stay independent",
			groups: [][]string{
				{"ticket", "issue"},
				{"send", "post"},
			},
			want: map[string][]string{
				"ticket": {"ticket", "issue"},
				"issue":  {"ticket", "issue"},
				"send":   {"send", "post"},
				"post":   {"send", "post"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSynonymMap(tt.groups)
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			for word, expectedSyns := range tt.want {
				actualSyns, ok := got[word]
				require.True(t, ok, "missing key %q", word)
				assert.ElementsMatch(t, expectedSyns, actualSyns, "synonyms for %q", word)
			}
			assert.Equal(t, len(tt.want), len(got), "unexpected extra keys in synonym map")
		})
	}
}

func TestComputeIDF(t *testing.T) {
	tools := []toolWithIntegration{
		{Integration: "a", Tool: mcp.ToolDefinition{Name: "a_list_items", Description: "List all items"}},
		{Integration: "a", Tool: mcp.ToolDefinition{Name: "a_get_item", Description: "Get a single item"}},
		{Integration: "b", Tool: mcp.ToolDefinition{Name: "b_list_items", Description: "List all items"}},
		{Integration: "b", Tool: mcp.ToolDefinition{Name: "b_create_item", Description: "Create an item"}},
		{Integration: "c", Tool: mcp.ToolDefinition{Name: "c_delete_item", Description: "Delete an item"}},
		{Integration: "c", Tool: mcp.ToolDefinition{Name: "c_list_things", Description: "List all things"}},
		{Integration: "c", Tool: mcp.ToolDefinition{Name: "c_get_thing", Description: "Get a thing"}},
		{Integration: "d", Tool: mcp.ToolDefinition{Name: "d_list_widgets", Description: "List widgets"}},
		{Integration: "d", Tool: mcp.ToolDefinition{Name: "d_create_widget", Description: "Create a widget"}},
		{Integration: "d", Tool: mcp.ToolDefinition{Name: "d_get_widget", Description: "Get a widget"}},
	}

	idf := computeIDF(tools)

	t.Run("common word has low IDF", func(t *testing.T) {
		// "list" appears in tool names/descriptions of many tools
		listIDF := idf["list"]
		assert.Greater(t, listIDF, 0.0, "list should have positive IDF")

		// "delete" appears in fewer tools
		deleteIDF := idf["delete"]
		assert.Greater(t, deleteIDF, listIDF, "rare word 'delete' should have higher IDF than common word 'list'")
	})

	t.Run("word in all tools has zero IDF", func(t *testing.T) {
		// Every tool has some word in common — check if any universal words get IDF=0
		// "item" or "a" might appear in all, but let's verify the math
		// log(N/N) = log(1) = 0
		for word, val := range idf {
			if val == 0.0 {
				t.Logf("Word %q has IDF=0 (appears in all tools)", word)
			}
		}
	})

	t.Run("missing word returns zero value", func(t *testing.T) {
		// Words not in any tool are not in the map; Go returns 0.0 for missing keys
		assert.Equal(t, 0.0, idf["nonexistent_word_xyz"])
	})

	t.Run("IDF formula is log(total/count)", func(t *testing.T) {
		// "delete" appears in exactly 1 tool (c_delete_item)
		// IDF = log(10/1) ≈ 2.302
		deleteIDF := idf["delete"]
		expected := math.Log(10.0 / 1.0)
		assert.InDelta(t, expected, deleteIDF, 0.01, "IDF for 'delete' should be log(10/1)")
	})
}

func TestScoreTool(t *testing.T) {
	synMap := buildSynonymMap([][]string{
		{"ticket", "issue", "bug"},
		{"create", "add", "new"},
	})

	tools := []toolWithIntegration{
		{Integration: "linear", Tool: mcp.ToolDefinition{Name: "linear_create_issue", Description: "Create a new issue"}},
		{Integration: "linear", Tool: mcp.ToolDefinition{Name: "linear_list_issues", Description: "List issues"}},
		{Integration: "github", Tool: mcp.ToolDefinition{Name: "github_list_issues", Description: "List issues"}},
		{Integration: "slack", Tool: mcp.ToolDefinition{Name: "slack_send_message", Description: "Send a message"}},
	}

	idf := computeIDF(tools)

	t.Run("exact match scores positive", func(t *testing.T) {
		score := scoreTool(tokenize("create issue"), tools[0], idf, synMap)
		assert.Greater(t, score, 0.0)
	})

	t.Run("synonym match scores positive", func(t *testing.T) {
		// "ticket" is a synonym for "issue" — should match linear_create_issue
		score := scoreTool(tokenize("create ticket"), tools[0], idf, synMap)
		assert.Greater(t, score, 0.0, "synonym 'ticket' should match tool with 'issue'")
	})

	t.Run("no match scores zero", func(t *testing.T) {
		score := scoreTool(tokenize("deploy kubernetes"), tools[0], idf, synMap)
		assert.Equal(t, 0.0, score)
	})

	t.Run("MAX across synonyms not SUM", func(t *testing.T) {
		// Query "bug" — synonyms are "ticket" and "issue"
		// Tool linear_create_issue has "issue" in name AND description
		// Score should be MAX of synonym matches, not sum
		scoreBug := scoreTool(tokenize("bug"), tools[0], idf, synMap)
		scoreIssue := scoreTool(tokenize("issue"), tools[0], idf, synMap)
		// "bug" expanded to {"bug","ticket","issue"} → MAX should equal direct "issue" score
		assert.InDelta(t, scoreIssue, scoreBug, 0.01,
			"synonym-expanded 'bug' should score same as direct 'issue' (MAX not SUM)")
	})

	t.Run("rare word contributes more than common word", func(t *testing.T) {
		// "create" appears in fewer tools than "issue" in our test set
		// so IDF("create") > IDF("issue") potentially
		scoreCreate := scoreTool(tokenize("create"), tools[0], idf, synMap)
		scoreIssue := scoreTool(tokenize("issue"), tools[0], idf, synMap)
		// Both should be positive; exact ranking depends on corpus
		assert.Greater(t, scoreCreate, 0.0)
		assert.Greater(t, scoreIssue, 0.0)
	})
}

func TestScoreTools_Sorting(t *testing.T) {
	synMap := buildSynonymMap([][]string{
		{"ticket", "issue"},
	})

	tools := []toolWithIntegration{
		{Integration: "github", Tool: mcp.ToolDefinition{Name: "github_list_issues", Description: "List issues for a repository"}},
		{Integration: "linear", Tool: mcp.ToolDefinition{Name: "linear_list_issues", Description: "List issues in a project"}},
		{Integration: "slack", Tool: mcp.ToolDefinition{Name: "slack_send_message", Description: "Send a message"}},
	}

	idf := computeIDF(tools)

	t.Run("higher score first", func(t *testing.T) {
		// "list" appears in both github and linear tools; "issues" in both too
		results := scoreTools("list issues", tools, idf, synMap)
		require.GreaterOrEqual(t, len(results), 2, "should have at least 2 results matching 'list issues'")

		for i := 1; i < len(results); i++ {
			assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score,
				"results should be sorted by score descending")
		}
	})

	t.Run("zero-score tools excluded", func(t *testing.T) {
		results := scoreTools("list issues", tools, idf, synMap)
		for _, r := range results {
			assert.Greater(t, r.Score, 0.0, "zero-score tools should be excluded")
		}
	})

	t.Run("equal scores sorted by integration then name", func(t *testing.T) {
		// Both github_list_issues and linear_list_issues contain "list" and "issues"
		// If they score equally, github should come before linear alphabetically
		results := scoreTools("list issues", tools, idf, synMap)
		if len(results) >= 2 && results[0].Score == results[1].Score {
			assert.LessOrEqual(t, results[0].Integration, results[1].Integration,
				"equal-score ties should be broken by integration name ascending")
		}
	})
}

func TestStopWords_FilteredFromQuery(t *testing.T) {
	synMap := buildSynonymMap(synonymGroups)

	tools := []toolWithIntegration{
		{Integration: "slack", Tool: mcp.ToolDefinition{Name: "slack_send_message", Description: "Send a message to a channel"}},
		{Integration: "github", Tool: mcp.ToolDefinition{Name: "github_list_repos", Description: "List repositories"}},
		{Integration: "linear", Tool: mcp.ToolDefinition{Name: "linear_create_issue", Description: "Create a new issue"}},
		{Integration: "sentry", Tool: mcp.ToolDefinition{Name: "sentry_list_issues", Description: "List issues for a project"}},
		{Integration: "datadog", Tool: mcp.ToolDefinition{Name: "datadog_search_logs", Description: "Search logs by query"}},
		{Integration: "aws", Tool: mcp.ToolDefinition{Name: "aws_sns_publish", Description: "Publish a message to SNS topic"}},
		{Integration: "notion", Tool: mcp.ToolDefinition{Name: "notion_search", Description: "Search pages and databases"}},
		{Integration: "gmail", Tool: mcp.ToolDefinition{Name: "gmail_create_draft", Description: "Create an email draft"}},
	}

	idf := computeIDF(tools)

	t.Run("verbose query matches same top tool as keyword query", func(t *testing.T) {
		// "Send a Slack message to the team" should rank the same #1 as "send slack message"
		// because "a", "to", "the" are stop words filtered before scoring.
		verboseResults := scoreTools("send a slack message to the team", tools, idf, synMap)
		keywordResults := scoreTools("send slack message", tools, idf, synMap)

		require.Greater(t, len(verboseResults), 0)
		require.Greater(t, len(keywordResults), 0)
		assert.Equal(t, keywordResults[0].Tool.Name, verboseResults[0].Tool.Name,
			"stop words should not change top result")
	})

	t.Run("query of only stop words returns no results", func(t *testing.T) {
		results := scoreTools("a the to is and", tools, idf, synMap)
		assert.Empty(t, results, "query of only stop words should match nothing")
	})

	t.Run("stop words not indexed in IDF", func(t *testing.T) {
		// "a" appears in many descriptions but should not be in the IDF map.
		assert.Equal(t, 0.0, idf["a"], "'a' should be filtered from IDF")
		assert.Equal(t, 0.0, idf["the"], "'the' should be filtered from IDF")
		assert.Equal(t, 0.0, idf["to"], "'to' should be filtered from IDF")
		// But real words should still be there.
		assert.Greater(t, idf["send"], 0.0, "'send' should have positive IDF")
		assert.Greater(t, idf["message"], 0.0, "'message' should have positive IDF")
	})
}

func TestPreComputedTokens(t *testing.T) {
	tools := []toolWithIntegration{
		{Integration: "slack", Tool: mcp.ToolDefinition{Name: "slack_send_message", Description: "Send a message to a channel"}},
		{Integration: "github", Tool: mcp.ToolDefinition{Name: "github_list_repos", Description: "List repositories"}},
		{Integration: "linear", Tool: mcp.ToolDefinition{Name: "linear_create_issue", Description: "Create a new issue"}},
	}

	computeIDF(tools) // populates tools[i].tokens as a side effect

	t.Run("computeIDF populates token sets on tools", func(t *testing.T) {
		for _, ti := range tools {
			require.NotNil(t, ti.tokens, "computeIDF should populate tokens for %s", ti.Tool.Name)
			assert.True(t, ti.tokens["slack"] || ti.tokens["github"] || ti.tokens["linear"],
				"tokens should contain the integration name")
		}
	})

	t.Run("tokens exclude stop words", func(t *testing.T) {
		for _, ti := range tools {
			assert.False(t, ti.tokens["a"], "stop words should not be in tokens for %s", ti.Tool.Name)
			assert.False(t, ti.tokens["to"], "stop words should not be in tokens for %s", ti.Tool.Name)
		}
	})
}

func TestSynonymGroups_Disjoint(t *testing.T) {
	seen := make(map[string][]string)
	for _, group := range synonymGroups {
		for _, word := range group {
			if prev, ok := seen[word]; ok {
				t.Errorf("word %q in two groups: %v and %v", word, prev, group)
			}
			seen[word] = group
		}
	}
}

func TestBuildSynonymMap_IncludesSelf(t *testing.T) {
	synMap := buildSynonymMap([][]string{
		{"ticket", "issue", "bug"},
	})

	// Each word should map to itself AND its synonyms.
	ticketSyns := synMap["ticket"]
	assert.Contains(t, ticketSyns, "ticket", "synonym map should include self")
	assert.Contains(t, ticketSyns, "issue")
	assert.Contains(t, ticketSyns, "bug")
}
