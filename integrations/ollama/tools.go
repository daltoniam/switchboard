package ollama

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Model Management ────────────────────────────────────────────
	{
		Name:        "ollama_list_models",
		Description: "List all locally installed Ollama AI models with sizes, families, parameter counts, and quantization levels. Start here to discover available models before running chat, generate, or embed.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "ollama_show_model",
		Description: "Show details of a specific Ollama model including parameters, prompt template, capabilities (vision, tools, thinking), and license. Returns rendered markdown. Use after list_models to inspect a model before using it.",
		Parameters: map[string]string{
			"model": "Model name (e.g. 'gemma3', 'llama3.2:7b'). Use list_models to discover available names.",
		},
		Required: []string{"model"},
	},
	{
		Name:        "ollama_pull_model",
		Description: "Download and install an AI model from the Ollama model registry. May take minutes for large models — do not retry on timeout. Use model names like 'gemma3', 'llama3.2', 'qwen3'. Use list_models after to confirm the download completed.",
		Parameters: map[string]string{
			"model": "Model name to download from registry (e.g. 'gemma3', 'llama3.2:7b', 'qwen3:14b')",
		},
		Required: []string{"model"},
	},
	{
		Name:        "ollama_delete_model",
		Description: "Permanently delete a locally installed Ollama model. This is irreversible — the model must be re-downloaded with pull_model to restore it. Use list_models first to verify the exact model name.",
		Parameters: map[string]string{
			"model": "Exact model name to delete. Must match a name from list_models.",
		},
		Required: []string{"model"},
	},
	{
		Name:        "ollama_copy_model",
		Description: "Create a copy of an existing local Ollama model with a new name. Use before create_model to preserve the original when customizing.",
		Parameters: map[string]string{
			"source":      "Existing model name to copy from (must be installed locally)",
			"destination": "New model name to create",
		},
		Required: []string{"source", "destination"},
	},
	{
		Name:        "ollama_create_model",
		Description: "Create a custom Ollama model from an existing base model. Embed a system prompt, override the prompt template, or change quantization. May take minutes for re-quantization — do not retry on timeout. Use show_model after to verify the result.",
		Parameters: map[string]string{
			"model":    "Name for the new custom model",
			"from":     "Base model to derive from (e.g. 'gemma3'). Must be installed locally.",
			"system":   "System prompt to embed permanently in the model",
			"template": "Go template string for prompt formatting",
			"quantize": "Quantization level to apply (e.g. 'q4_K_M', 'q8_0'). Re-quantizes the model weights.",
		},
		Required: []string{"model"},
	},
	{
		Name:        "ollama_list_running",
		Description: "List currently loaded and running Ollama models with VRAM usage, context window size, and automatic unload time. Use to check GPU memory pressure before loading additional models.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "ollama_get_version",
		Description: "Get the Ollama server version. Use to verify the server is reachable and check compatibility.",
		Parameters:  map[string]string{},
	},

	// ── Inference ───────────────────────────────────────────────────
	{
		Name:        "ollama_chat",
		Description: "Send a multi-turn chat conversation to a local Ollama model and get a complete response. Preferred over generate for conversations — maintains message history with roles. Supports tool calling, structured JSON output, vision (images in messages), and thinking/reasoning mode. Returns the full response in one call (non-streaming).",
		Parameters: map[string]string{
			"model":      "Model name (e.g. 'gemma3', 'llama3.2'). Must be installed — use list_models to check.",
			"messages":   "Array of message objects. Each has 'role' ('system', 'user', 'assistant') and 'content' (string). For vision: add 'images' array with base64-encoded strings.",
			"tools":      "Array of tool/function definitions the model may call (OpenAI function-calling format)",
			"format":     "Constrain output format: 'json' for freeform JSON, or a JSON schema object for structured output",
			"options":    "Model parameters object: temperature (0-2), top_k, top_p, seed, num_ctx (context window), num_predict (max tokens), etc.",
			"think":      "Enable chain-of-thought reasoning: true, false, 'high', 'medium', or 'low'. Response includes a 'thinking' field when enabled.",
			"keep_alive": "Duration to keep model in memory after request: '5m', '1h', or 0 to unload immediately. Default: 5 minutes.",
		},
		Required: []string{"model", "messages"},
	},
	{
		Name:        "ollama_generate",
		Description: "Generate text completion from a single prompt using a local Ollama model. Use for one-shot generation without conversation history — prefer chat for multi-turn dialogue. Supports vision (base64 images), fill-in-the-middle (prompt + suffix), structured JSON output, and thinking mode. Returns the full response in one call (non-streaming).",
		Parameters: map[string]string{
			"model":      "Model name (e.g. 'gemma3', 'llama3.2'). Must be installed — use list_models to check.",
			"prompt":     "Text prompt for generation",
			"suffix":     "Text after the insertion point for fill-in-the-middle completion. Model generates text between prompt and suffix.",
			"images":     "Array of base64-encoded images for multimodal/vision models (e.g. llava, gemma4)",
			"format":     "Constrain output format: 'json' for freeform JSON, or a JSON schema object for structured output",
			"system":     "System prompt to prepend. Overrides any system prompt embedded in the model.",
			"options":    "Model parameters object: temperature (0-2), top_k, top_p, seed, num_ctx (context window), num_predict (max tokens), etc.",
			"think":      "Enable chain-of-thought reasoning: true, false, 'high', 'medium', or 'low'. Response includes a 'thinking' field when enabled.",
			"raw":        "If true, prompt is sent directly without applying the model's chat template. Use for pre-formatted prompts only.",
			"keep_alive": "Duration to keep model in memory after request: '5m', '1h', or 0 to unload immediately. Default: 5 minutes.",
		},
		Required: []string{"model"},
	},
	{
		Name:        "ollama_embed",
		Description: "Generate vector embeddings for text using a local Ollama embedding model. For semantic search, similarity matching, and RAG pipelines. Supports single string or batch (array of strings) input. Not all models support embeddings — use an embedding model like 'all-minilm' or 'nomic-embed-text'. Returns 400 error if the model lacks embedding support.",
		Parameters: map[string]string{
			"model":      "Embedding model name (e.g. 'all-minilm', 'nomic-embed-text'). Must support embeddings — general chat models will return an error.",
			"input":      "Text to embed: a single string, or an array of strings for batch embedding",
			"truncate":   "Truncate input exceeding context window (default true). Set false to return an error instead of silently truncating.",
			"dimensions": "Custom output vector dimensions. Only supported by some models.",
			"keep_alive": "Duration to keep model in memory: '5m', '1h', or 0 to unload immediately",
			"options":    "Model parameters: seed, num_ctx, etc.",
		},
		Required: []string{"model", "input"},
	},
}
