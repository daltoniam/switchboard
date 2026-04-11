package pganalyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const defaultGraphQLURL = "https://app.pganalyze.com/graphql"

var (
	_ mcp.Integration                = (*pganalyze)(nil)
	_ mcp.FieldCompactionIntegration = (*pganalyze)(nil)
	_ mcp.PlainTextCredentials       = (*pganalyze)(nil)
)

type pganalyze struct {
	apiKey           string
	graphqlURL       string
	organizationSlug string
	client           *http.Client
}

func New() mcp.Integration {
	return &pganalyze{client: &http.Client{}}
}

func (p *pganalyze) Name() string { return "pganalyze" }

func (p *pganalyze) Configure(_ context.Context, creds mcp.Credentials) error {
	p.apiKey = creds["api_key"]
	if p.apiKey == "" {
		return fmt.Errorf("pganalyze: api_key is required")
	}
	p.graphqlURL = defaultGraphQLURL
	if v := creds["base_url"]; v != "" {
		v = strings.TrimRight(v, "/")
		if strings.HasSuffix(v, "/graphql") {
			p.graphqlURL = v
		} else {
			p.graphqlURL = v + "/graphql"
		}
	}
	p.organizationSlug = creds["organization_slug"]
	return nil
}

func (p *pganalyze) Healthy(ctx context.Context) bool {
	if p.apiKey == "" {
		return false
	}
	_, err := p.gql(ctx, `{ __typename }`, nil)
	return err == nil
}

func (p *pganalyze) Tools() []mcp.ToolDefinition {
	return tools
}

func (p *pganalyze) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *pganalyze) PlainTextKeys() []string {
	return []string{"base_url", "organization_slug"}
}

func (p *pganalyze) orgSlug(args map[string]any) string {
	if v, _ := mcp.ArgStr(args, "organization_slug"); v != "" {
		return v
	}
	return p.organizationSlug
}

func (p *pganalyze) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, p, args)
}

// --- GraphQL helpers ---

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *pganalyze) gql(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	body := map[string]any{"query": query}
	if variables != nil {
		body["variables"] = variables
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.graphqlURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("pganalyze API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pganalyze API error (%d): %s", resp.StatusCode, string(data))
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

type handlerFunc func(ctx context.Context, p *pganalyze, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Servers
	mcp.ToolName("pganalyze_get_servers"): getServers,

	// Issues
	mcp.ToolName("pganalyze_get_issues"): getIssues,

	// Query Stats
	mcp.ToolName("pganalyze_get_query_stats"): getQueryStats,
}
