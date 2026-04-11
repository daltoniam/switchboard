package rwx

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	mcp "github.com/daltoniam/switchboard"
)

// cliToMCPReplacements maps rwx CLI command references to MCP tool references
// in proxied tool descriptions and results.
var cliToMCPReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile("`rwx logs[^`]*`"), "the log tools (rwx_get_task_logs, rwx_head_logs, rwx_tail_logs, rwx_grep_logs)"},
	{regexp.MustCompile("`rwx results[^`]*`"), "the rwx_get_run_results tool"},
	{regexp.MustCompile("`rwx artifacts[^`]*`"), "the rwx_get_artifacts tool"},
	{regexp.MustCompile("`rwx run[^`]*`"), "the rwx_launch_ci_run tool"},
	{regexp.MustCompile(`(?i)\brwx logs\b`), "the log tools (rwx_get_task_logs, rwx_head_logs, rwx_tail_logs, rwx_grep_logs)"},
	{regexp.MustCompile(`(?i)\brwx results\b`), "the rwx_get_run_results tool"},
	{regexp.MustCompile(`(?i)\brwx artifacts\b`), "the rwx_get_artifacts tool"},
	{regexp.MustCompile(`(?i)\brwx run\b`), "the rwx_launch_ci_run tool"},
}

func transformCLIReferences(text string) string {
	for _, r := range cliToMCPReplacements {
		text = r.pattern.ReplaceAllString(text, r.replacement)
	}
	return text
}

// proxyClient manages a subprocess running `rwx mcp serve` and proxies tool calls.
type proxyClient struct {
	mu      sync.Mutex
	proc    *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	nextID  int
	pending map[int]chan json.RawMessage
	tools   []proxyToolDef
	running bool
}

type proxyToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func newProxyClient() *proxyClient {
	return &proxyClient{
		pending: make(map[int]chan json.RawMessage),
	}
}

func (p *proxyClient) start(rwxBin string) error {
	cmd := exec.Command(rwxBin, "mcp", "serve")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start rwx mcp serve: %w", err)
	}

	p.proc = cmd
	p.stdin = stdin
	p.scanner = bufio.NewScanner(stdout)
	p.scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	p.running = true

	go p.readLoop()

	if err := p.initialize(); err != nil {
		p.stop()
		return fmt.Errorf("initialize: %w", err)
	}

	if err := p.fetchTools(); err != nil {
		p.stop()
		return fmt.Errorf("fetch tools: %w", err)
	}

	return nil
}

func (p *proxyClient) readLoop() {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue
		}
		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		p.mu.Lock()
		ch, ok := p.pending[resp.ID]
		if ok {
			delete(p.pending, resp.ID)
		}
		p.mu.Unlock()

		if ok {
			if resp.Error != nil {
				errJSON, _ := json.Marshal(map[string]string{"error": resp.Error.Message})
				ch <- json.RawMessage(errJSON)
			} else {
				ch <- resp.Result
			}
		}
	}
}

func (p *proxyClient) sendRequest(method string, params any) (json.RawMessage, error) {
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	ch := make(chan json.RawMessage, 1)
	p.pending[id] = ch
	p.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	p.mu.Lock()
	_, err = p.stdin.Write(data)
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}

	result := <-ch
	return result, nil
}

func (p *proxyClient) sendNotification(method string, params any) {
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	p.mu.Lock()
	_, _ = p.stdin.Write(data)
	p.mu.Unlock()
}

func (p *proxyClient) initialize() error {
	result, err := p.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "switchboard-rwx",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return err
	}

	var initResp struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(result, &initResp); err != nil {
		return fmt.Errorf("parse init response: %w", err)
	}
	if initResp.ProtocolVersion == "" {
		return fmt.Errorf("missing protocol version in init response")
	}

	p.sendNotification("notifications/initialized", map[string]any{})
	return nil
}

func (p *proxyClient) fetchTools() error {
	result, err := p.sendRequest("tools/list", map[string]any{})
	if err != nil {
		return err
	}

	var toolsResp struct {
		Tools []proxyToolDef `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsResp); err != nil {
		return fmt.Errorf("parse tools response: %w", err)
	}
	p.tools = toolsResp.Tools
	return nil
}

func (p *proxyClient) toolDefinitions() []mcp.ToolDefinition {
	var defs []mcp.ToolDefinition
	for _, t := range p.tools {
		params := make(map[string]string)
		if props, ok := t.InputSchema["properties"].(map[string]interface{}); ok {
			required := make(map[string]bool)
			if reqList, ok := t.InputSchema["required"].([]interface{}); ok {
				for _, r := range reqList {
					if s, ok := r.(string); ok {
						required[s] = true
					}
				}
			}
			for key, val := range props {
				desc := ""
				if propMap, ok := val.(map[string]interface{}); ok {
					if d, ok := propMap["description"].(string); ok {
						desc = transformCLIReferences(d)
					}
				}
				params[key] = desc
			}
		}
		desc := t.Description
		if desc != "" {
			desc = transformCLIReferences(desc)
		}

		var requiredFields []string
		if reqList, ok := t.InputSchema["required"].([]interface{}); ok {
			for _, r := range reqList {
				if s, ok := r.(string); ok {
					requiredFields = append(requiredFields, s)
				}
			}
		}

		defs = append(defs, mcp.ToolDefinition{
			Name:        mcp.ToolName("rwx_proxy_" + t.Name),
			Description: "[Proxied from rwx mcp serve] " + desc,
			Parameters:  params,
			Required:    requiredFields,
		})
	}
	return defs
}

func (p *proxyClient) execute(_ context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	originalName := strings.TrimPrefix(string(toolName), "rwx_proxy_")

	found := false
	for _, t := range p.tools {
		if t.Name == originalName {
			found = true
			break
		}
	}
	if !found {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}

	result, err := p.sendRequest("tools/call", map[string]any{
		"name":      originalName,
		"arguments": args,
	})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("proxy call %s: %w", originalName, err))
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		return &mcp.ToolResult{Data: transformCLIReferences(string(result))}, nil
	}

	var texts []string
	for _, c := range callResult.Content {
		if c.Type == "text" {
			texts = append(texts, transformCLIReferences(c.Text))
		}
	}

	return &mcp.ToolResult{
		Data:    strings.Join(texts, "\n"),
		IsError: callResult.IsError,
	}, nil
}

func (p *proxyClient) stop() {
	if p.proc != nil && p.proc.Process != nil {
		_ = p.proc.Process.Kill()
		_ = p.proc.Wait()
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	p.running = false
}
