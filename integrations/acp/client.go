package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// client is an HTTP client for the ACP protocol.
type client struct {
	name         string
	baseURL      string
	httpClient   *http.Client
	streamClient *http.Client
	headers      map[string]string
}

func newClient(name, baseURL string, headers map[string]string) *client {
	transport := http.DefaultTransport
	return &client{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: transport,
		},
		streamClient: &http.Client{
			Timeout:   0,
			Transport: transport,
		},
		headers: headers,
	}
}

// listAgents returns agents available on the ACP server.
func (c *client) listAgents(ctx context.Context) ([]AgentManifest, error) {
	url := fmt.Sprintf("%s/agents?limit=100&offset=0", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result AgentsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Agents, nil
}

// createRunSync creates a run and blocks until completion.
func (c *client) createRunSync(ctx context.Context, agentName string, input []Message, sessionID string) (*Run, error) {
	body := RunCreateRequest{
		AgentName: agentName,
		Input:     input,
		SessionID: sessionID,
		Mode:      RunModeSync,
	}
	return c.createRun(ctx, body)
}

// createRunStream creates a run and returns a channel of streaming events.
func (c *client) createRunStream(ctx context.Context, agentName string, input []Message, sessionID string) (<-chan Event, error) {
	body := RunCreateRequest{
		AgentName: agentName,
		Input:     input,
		SessionID: sessionID,
		Mode:      RunModeStream,
	}
	return c.doStream(ctx, fmt.Sprintf("%s/runs", c.baseURL), body)
}

// resumeRun resumes an awaiting run with new input (sync mode).
func (c *client) resumeRun(ctx context.Context, runID string, resume *AwaitResume) (*Run, error) {
	body := RunResumeRequest{
		RunID:       runID,
		AwaitResume: resume,
		Mode:        RunModeSync,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/runs/%s", c.baseURL, runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resume run: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &run, nil
}

// resumeRunStream resumes an awaiting run and returns a channel of streaming events.
func (c *client) resumeRunStream(ctx context.Context, runID string, resume *AwaitResume) (<-chan Event, error) {
	body := RunResumeRequest{
		RunID:       runID,
		AwaitResume: resume,
		Mode:        RunModeStream,
	}
	return c.doStream(ctx, fmt.Sprintf("%s/runs/%s", c.baseURL, runID), body)
}

func (c *client) createRun(ctx context.Context, body RunCreateRequest) (*Run, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/runs", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &run, nil
}

func (c *client) doStream(ctx context.Context, url string, body any) (<-chan Event, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", ContentTypeNDJSON)
	c.setHeaders(req)

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, c.readError(resp)
	}

	return parseStream(ctx, resp.Body), nil
}

func (c *client) setHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

func (c *client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var acpErr ACPError
	if json.Unmarshal(body, &acpErr) == nil && acpErr.Message != "" {
		return fmt.Errorf("ACP error (HTTP %d): %s", resp.StatusCode, acpErr.Message)
	}
	return fmt.Errorf("ACP error (HTTP %d): %s", resp.StatusCode, string(body))
}

// parseStream reads an NDJSON stream and emits typed events.
func parseStream(ctx context.Context, r io.ReadCloser) <-chan Event {
	ch := make(chan Event, 16)
	go func() {
		defer close(ch)
		defer func() { _ = r.Close() }()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				ch <- Event{
					Type:  EventError,
					Error: &ACPError{Message: fmt.Sprintf("failed to parse event: %v", err)},
				}
				continue
			}
			ch <- event
		}
	}()
	return ch
}
