package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type acpIntegration struct {
	clients map[string]*client
	timeout time.Duration
}

var _ mcp.PlainTextCredentials = (*acpIntegration)(nil)
var _ mcp.PlaceholderHints = (*acpIntegration)(nil)
var _ mcp.OptionalCredentials = (*acpIntegration)(nil)

func New() mcp.Integration {
	return &acpIntegration{
		clients: make(map[string]*client),
		timeout: 120 * time.Second,
	}
}

func (a *acpIntegration) Name() string { return "acp" }

// serverEntry represents a single ACP server parsed from the config JSON.
type serverEntry struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (a *acpIntegration) Configure(_ context.Context, creds mcp.Credentials) error {
	raw := creds["config"]
	if raw == "" {
		return nil
	}

	var cfg struct {
		Servers []serverEntry `json:"servers"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("acp: invalid config JSON: %w", err)
	}

	a.clients = make(map[string]*client, len(cfg.Servers))
	for _, srv := range cfg.Servers {
		if srv.URL == "" {
			return fmt.Errorf("acp: server %q has no url", srv.Name)
		}
		name := srv.Name
		if name == "" {
			name = srv.URL
		}
		a.clients[name] = newClient(name, srv.URL, srv.Headers)
	}
	return nil
}

func (a *acpIntegration) PlainTextKeys() []string { return []string{"config"} }
func (a *acpIntegration) OptionalKeys() []string  { return []string{"config"} }
func (a *acpIntegration) Placeholders() map[string]string {
	return map[string]string{
		"config": `{"servers":[{"name":"local","url":"http://localhost:8199"}]}`,
	}
}

func (a *acpIntegration) Healthy(ctx context.Context) bool {
	if len(a.clients) == 0 {
		return true
	}
	for _, c := range a.clients {
		_, err := c.listAgents(ctx)
		if err == nil {
			return true
		}
	}
	return false
}

func (a *acpIntegration) Tools() []mcp.ToolDefinition {
	return tools
}

func (a *acpIntegration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, a, args)
}

type handlerFunc func(ctx context.Context, a *acpIntegration, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	"acp_list_agents": handleListAgents,
	"acp_run_agent":   handleRunAgent,
	"acp_resume_run":  handleResumeRun,
}

// resolveClient returns a client based on the args. Priority:
// 1. server_url — create an ephemeral client for the inline URL
// 2. server — look up a pre-configured client by name
// 3. first pre-configured client
func (a *acpIntegration) resolveClient(args map[string]any) (*client, error) {
	serverURL, _ := mcp.ArgStr(args, "server_url")
	if serverURL != "" {
		headers := parseHeaders(args)
		return newClient(serverURL, serverURL, headers), nil
	}

	serverName, _ := mcp.ArgStr(args, "server")
	if serverName != "" {
		c, ok := a.clients[serverName]
		if !ok {
			available := make([]string, 0, len(a.clients))
			for k := range a.clients {
				available = append(available, k)
			}
			return nil, fmt.Errorf("server %q not found, available: %s", serverName, strings.Join(available, ", "))
		}
		return c, nil
	}

	for _, c := range a.clients {
		return c, nil
	}
	return nil, fmt.Errorf("no ACP server specified: provide server_url or configure servers in credentials")
}

// parseHeaders extracts optional HTTP headers from the server_headers arg.
func parseHeaders(args map[string]any) map[string]string {
	raw, _ := mcp.ArgStr(args, "server_headers")
	if raw == "" {
		return nil
	}
	var headers map[string]string
	if json.Unmarshal([]byte(raw), &headers) != nil {
		return nil
	}
	return headers
}

func handleListAgents(ctx context.Context, a *acpIntegration, args map[string]any) (*mcp.ToolResult, error) {
	serverURL, _ := mcp.ArgStr(args, "server_url")
	serverName, _ := mcp.ArgStr(args, "server")

	if serverURL == "" && serverName == "" && len(a.clients) > 0 {
		var allAgents []map[string]string
		for name, c := range a.clients {
			agents, err := c.listAgents(ctx)
			if err != nil {
				allAgents = append(allAgents, map[string]string{
					"server": name,
					"error":  err.Error(),
				})
				continue
			}
			for _, agent := range agents {
				allAgents = append(allAgents, map[string]string{
					"name":        agent.Name,
					"description": agent.Description,
					"server":      name,
				})
			}
		}
		return mcp.JSONResult(allAgents)
	}

	c, err := a.resolveClient(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	agents, err := c.listAgents(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}

	summaries := make([]map[string]string, len(agents))
	for i, agent := range agents {
		summaries[i] = map[string]string{
			"name":        agent.Name,
			"description": agent.Description,
			"server":      c.name,
		}
	}
	return mcp.JSONResult(summaries)
}

func handleRunAgent(ctx context.Context, a *acpIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	agentName := r.Str("agent_name")
	input := r.Str("input")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if agentName == "" {
		return mcp.ErrResult(fmt.Errorf("agent_name is required"))
	}
	if input == "" {
		return mcp.ErrResult(fmt.Errorf("input is required"))
	}

	sessionID, _ := mcp.ArgStr(args, "session_id")

	c, err := a.resolveClient(args)
	if err != nil {
		return mcp.ErrResult(err)
	}

	messages := []Message{NewUserMessage(input)}

	events, err := c.createRunStream(ctx, agentName, messages, sessionID)
	if err != nil {
		return runSync(ctx, c, a.timeout, agentName, messages, sessionID)
	}
	return collectStreamResponse(events)
}

func handleResumeRun(ctx context.Context, a *acpIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	runID := r.Str("run_id")
	input := r.Str("input")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if runID == "" {
		return mcp.ErrResult(fmt.Errorf("run_id is required"))
	}
	if input == "" {
		return mcp.ErrResult(fmt.Errorf("input is required"))
	}

	c, err := a.resolveClient(args)
	if err != nil {
		return mcp.ErrResult(err)
	}

	resume := &AwaitResume{
		Message: &Message{
			Role:  "user",
			Parts: []MessagePart{{ContentType: "text/plain", Content: input}},
		},
	}

	events, err := c.resumeRunStream(ctx, runID, resume)
	if err != nil {
		run, syncErr := c.resumeRun(ctx, runID, resume)
		if syncErr != nil {
			return mcp.ErrResult(syncErr)
		}
		return formatRunResponse(run)
	}
	return collectStreamResponse(events)
}

func runSync(ctx context.Context, c *client, timeout time.Duration, agentName string, input []Message, sessionID string) (*mcp.ToolResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	run, err := c.createRunSync(timeoutCtx, agentName, input, sessionID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return formatRunResponse(run)
}

func collectStreamResponse(events <-chan Event) (*mcp.ToolResult, error) {
	var parts []string
	var lastRun *Run

	for event := range events {
		switch event.Type {
		case EventMessagePart:
			if event.Part != nil && event.Part.Content != "" {
				parts = append(parts, event.Part.Content)
			}
		case EventMessageCompleted:
			if event.Message != nil {
				text := TextContent([]Message{*event.Message})
				if text != "" && len(parts) == 0 {
					parts = append(parts, text)
				}
			}
		case EventRunCompleted, EventRunFailed, EventRunAwaiting, EventRunCancelled:
			lastRun = event.Run
		case EventError:
			if event.Error != nil {
				return &mcp.ToolResult{Data: event.Error.Message, IsError: true}, nil
			}
		}
	}

	if lastRun != nil && lastRun.Status == RunStatusAwaiting {
		return formatRunResponse(lastRun)
	}

	if lastRun != nil && lastRun.Status == RunStatusFailed {
		msg := "agent run failed"
		if lastRun.Error != nil {
			msg = lastRun.Error.Message
		}
		return &mcp.ToolResult{Data: msg, IsError: true}, nil
	}

	if len(parts) > 0 {
		return &mcp.ToolResult{Data: strings.Join(parts, "")}, nil
	}

	if lastRun != nil {
		return formatRunResponse(lastRun)
	}

	return &mcp.ToolResult{Data: "no response received from agent", IsError: true}, nil
}

func formatRunResponse(run *Run) (*mcp.ToolResult, error) {
	if run.Status == RunStatusAwaiting {
		awaitMsg := "The agent is waiting for additional input."
		if run.AwaitRequest != nil && run.AwaitRequest.Message != nil {
			awaitMsg = TextContent([]Message{*run.AwaitRequest.Message})
		}
		result := map[string]any{
			"status":  "awaiting",
			"run_id":  run.RunID,
			"message": awaitMsg,
		}
		return mcp.JSONResult(result)
	}

	if run.Status == RunStatusFailed {
		msg := "agent run failed"
		if run.Error != nil {
			msg = run.Error.Message
		}
		return &mcp.ToolResult{Data: msg, IsError: true}, nil
	}

	text := TextContent(run.Output)
	if text != "" {
		return &mcp.ToolResult{Data: text}, nil
	}
	return mcp.JSONResult(run)
}
