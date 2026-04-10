package acp

import "time"

// RunStatus represents the state of an ACP run.
type RunStatus string

const (
	RunStatusCreated    RunStatus = "created"
	RunStatusInProgress RunStatus = "in-progress"
	RunStatusAwaiting   RunStatus = "awaiting"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusFailed     RunStatus = "failed"
	RunStatusCancelling RunStatus = "cancelling"
	RunStatusCancelled  RunStatus = "cancelled"
)

// IsTerminal returns true if the run status is a terminal state.
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return true
	}
	return false
}

// RunMode determines how a run is executed.
type RunMode string

const (
	RunModeSync   RunMode = "sync"
	RunModeAsync  RunMode = "async"
	RunModeStream RunMode = "stream"
)

// AgentManifest describes a remote ACP agent.
type AgentManifest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Message is the core communication unit in ACP.
type Message struct {
	Role  string        `json:"role"`
	Parts []MessagePart `json:"parts"`
}

// MessagePart is a single typed content unit within a message.
type MessagePart struct {
	ContentType string `json:"content_type"`
	Content     string `json:"content,omitempty"`
}

// Run represents an ACP agent run.
type Run struct {
	AgentName    string        `json:"agent_name"`
	RunID        string        `json:"run_id"`
	SessionID    string        `json:"session_id,omitempty"`
	Status       RunStatus     `json:"status"`
	Output       []Message     `json:"output"`
	AwaitRequest *AwaitRequest `json:"await_request,omitempty"`
	Error        *ACPError     `json:"error,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	FinishedAt   *time.Time    `json:"finished_at,omitempty"`
}

// AwaitRequest represents an agent's request for external input.
type AwaitRequest struct {
	Message *Message `json:"message,omitempty"`
}

// AwaitResume provides input to resume an awaiting run.
type AwaitResume struct {
	Message *Message `json:"message,omitempty"`
}

// ACPError represents an error returned by the ACP server.
type ACPError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

// RunCreateRequest is the body for POST /runs.
type RunCreateRequest struct {
	AgentName string    `json:"agent_name"`
	Input     []Message `json:"input"`
	SessionID string    `json:"session_id,omitempty"`
	Mode      RunMode   `json:"mode,omitempty"`
}

// RunResumeRequest is the body for POST /runs/{run_id}.
type RunResumeRequest struct {
	RunID       string       `json:"run_id"`
	AwaitResume *AwaitResume `json:"await_resume"`
	Mode        RunMode      `json:"mode,omitempty"`
}

// AgentsListResponse is the response from GET /agents.
type AgentsListResponse struct {
	Agents []AgentManifest `json:"agents"`
}

// EventType identifies the kind of streaming event.
type EventType string

const (
	EventRunCreated       EventType = "run.created"
	EventRunInProgress    EventType = "run.in-progress"
	EventRunAwaiting      EventType = "run.awaiting"
	EventRunCompleted     EventType = "run.completed"
	EventRunFailed        EventType = "run.failed"
	EventRunCancelled     EventType = "run.cancelled"
	EventMessageCreated   EventType = "message.created"
	EventMessagePart      EventType = "message.part"
	EventMessageCompleted EventType = "message.completed"
	EventError            EventType = "error"
)

// Event is a discriminated union of all streaming event types.
type Event struct {
	Type    EventType    `json:"type"`
	Run     *Run         `json:"run,omitempty"`
	Message *Message     `json:"message,omitempty"`
	Part    *MessagePart `json:"part,omitempty"`
	Error   *ACPError    `json:"error,omitempty"`
}

// ContentTypeNDJSON is the MIME type for newline-delimited JSON streams.
const ContentTypeNDJSON = "application/x-ndjson"

// TextContent extracts all text/plain content from a slice of messages.
func TextContent(messages []Message) string {
	var result string
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if part.ContentType == "text/plain" || part.ContentType == "" {
				if result != "" {
					result += "\n"
				}
				result += part.Content
			}
		}
	}
	return result
}

// NewUserMessage creates a simple text message with the "user" role.
func NewUserMessage(text string) Message {
	return Message{
		Role: "user",
		Parts: []MessagePart{
			{ContentType: "text/plain", Content: text},
		},
	}
}

// NewAgentMessage creates a simple text message with the "agent" role.
func NewAgentMessage(text string) Message {
	return Message{
		Role: "agent",
		Parts: []MessagePart{
			{ContentType: "text/plain", Content: text},
		},
	}
}
