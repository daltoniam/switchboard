package ollama

import "encoding/json"

// --- Semantic primitives — prevent mixing distinct string domains ---

// ModelName identifies a model (e.g., "gemma4:e2b", "llama3.2:7b").
type ModelName string

// ModelDigest is a SHA256 content digest identifying a specific model version.
type ModelDigest string

// ModelFamily identifies the architecture family (e.g., "gemma4", "qwen2", "llama").
type ModelFamily string

// ParameterSize is a human-readable size string (e.g., "5.1B", "7.6B").
type ParameterSize string

// QuantizationLevel describes the quantization method (e.g., "Q4_K_M", "Q8_0").
type QuantizationLevel string

// ModelFormat is the serialization format (e.g., "gguf").
type ModelFormat string

// Capability is a model capability flag (e.g., "completion", "vision", "tools", "thinking").
type Capability string

// ChatRole identifies a message participant ("system", "user", "assistant", "tool").
type ChatRole string

// DoneReason explains why generation stopped ("stop", "length", "load").
type DoneReason string

// Embedding is a single embedding vector.
type Embedding []float64

// Nanoseconds represents a duration in nanoseconds (all Ollama timing fields).
type Nanoseconds int64

// --- Shared sub-types ---

// modelDetails appears in /api/tags, /api/show, and /api/ps responses.
type modelDetails struct {
	ParentModel       ModelName         `json:"parent_model"`
	Format            ModelFormat       `json:"format"`
	Family            ModelFamily       `json:"family"`
	Families          []ModelFamily     `json:"families"`
	ParameterSize     ParameterSize     `json:"parameter_size"`
	QuantizationLevel QuantizationLevel `json:"quantization_level"`
}

// timingStats are common timing fields across chat, generate, and embed responses.
type timingStats struct {
	TotalDuration      Nanoseconds `json:"total_duration"`
	LoadDuration       Nanoseconds `json:"load_duration"`
	PromptEvalCount    int         `json:"prompt_eval_count"`
	PromptEvalDuration Nanoseconds `json:"prompt_eval_duration"`
	EvalCount          int         `json:"eval_count"`
	EvalDuration       Nanoseconds `json:"eval_duration"`
}

// --- /api/tags ---

type tagsResponse struct {
	Models []modelEntry `json:"models"`
}

type modelEntry struct {
	Name       ModelName    `json:"name"`
	Model      ModelName    `json:"model"`
	ModifiedAt string       `json:"modified_at"`
	Size       int64        `json:"size"`
	Digest     ModelDigest  `json:"digest"`
	Details    modelDetails `json:"details"`
}

// --- /api/show ---

type showResponse struct {
	Model        ModelName    `json:"model"` // injected by handler (not in upstream API response)
	License      string       `json:"license"`
	Modelfile    string       `json:"modelfile"`
	Parameters   string       `json:"parameters"`
	Template     string       `json:"template"`
	Details      modelDetails `json:"details"`
	Capabilities []Capability `json:"capabilities"`
	ModifiedAt   string       `json:"modified_at"`
	// model_info (55+ keys) and tensors (2000+ entries) intentionally omitted.
}

// --- /api/ps ---

type psResponse struct {
	Models []runningModel `json:"models"`
}

type runningModel struct {
	Name          ModelName    `json:"name"`
	Model         ModelName    `json:"model"`
	Size          int64        `json:"size"`
	Digest        ModelDigest  `json:"digest"`
	Details       modelDetails `json:"details"`
	ExpiresAt     string       `json:"expires_at"`
	SizeVRAM      int64        `json:"size_vram"`
	ContextLength int          `json:"context_length"`
}

// --- /api/version ---

type versionResponse struct {
	Version string `json:"version"`
}

// --- /api/chat ---

type chatRequest struct {
	Model     ModelName       `json:"model"`
	Messages  []chatMessage   `json:"messages"`
	Stream    bool            `json:"stream"`
	Tools     json.RawMessage `json:"tools,omitempty"`
	Format    json.RawMessage `json:"format,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
	Think     json.RawMessage `json:"think,omitempty"`
	KeepAlive json.RawMessage `json:"keep_alive,omitempty"`
}

type chatMessage struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type chatResponse struct {
	Model      ModelName  `json:"model"`
	CreatedAt  string     `json:"created_at"`
	Message    chatReply  `json:"message"`
	Done       bool       `json:"done"`
	DoneReason DoneReason `json:"done_reason"`
	timingStats
}

type chatReply struct {
	Role      ChatRole        `json:"role"`
	Content   string          `json:"content"`
	Thinking  string          `json:"thinking,omitempty"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
}

// --- /api/generate ---

type generateRequest struct {
	Model     ModelName       `json:"model"`
	Prompt    string          `json:"prompt"`
	Stream    bool            `json:"stream"`
	Suffix    string          `json:"suffix,omitempty"`
	Images    []string        `json:"images,omitempty"`
	Format    json.RawMessage `json:"format,omitempty"`
	System    string          `json:"system,omitempty"`
	Think     json.RawMessage `json:"think,omitempty"`
	Raw       bool            `json:"raw,omitempty"`
	KeepAlive json.RawMessage `json:"keep_alive,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
}

type generateResponse struct {
	Model      ModelName  `json:"model"`
	CreatedAt  string     `json:"created_at"`
	Response   string     `json:"response"`
	Thinking   string     `json:"thinking,omitempty"`
	Done       bool       `json:"done"`
	DoneReason DoneReason `json:"done_reason"`
	// context (token ID array) intentionally omitted.
	timingStats
}

// --- /api/embed ---

type embedRequest struct {
	Model      ModelName       `json:"model"`
	Input      any             `json:"input"`
	Stream     bool            `json:"stream"`
	Truncate   *bool           `json:"truncate,omitempty"`
	Dimensions *int            `json:"dimensions,omitempty"`
	KeepAlive  json.RawMessage `json:"keep_alive,omitempty"`
	Options    map[string]any  `json:"options,omitempty"`
}

type embedResponse struct {
	Model           ModelName   `json:"model"`
	Embeddings      []Embedding `json:"embeddings"`
	TotalDuration   Nanoseconds `json:"total_duration"`
	LoadDuration    Nanoseconds `json:"load_duration"`
	PromptEvalCount int         `json:"prompt_eval_count"`
}

// --- /api/copy ---

type copyRequest struct {
	Source      ModelName `json:"source"`
	Destination ModelName `json:"destination"`
}
