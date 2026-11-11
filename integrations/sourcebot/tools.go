package sourcebot

import mcp "github.com/daltoniam/switchboard"

const (
	repoParam = "Full repository name as Sourcebot indexes it (e.g. github.com/acme/api); see list_repos"
	refParam  = "Optional branch, tag, or commit SHA. Default: the repository's default branch"
)

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("sourcebot_search_code"),
		Description: "Search source code across every indexed repository in a self-hosted Sourcebot instance (grep-style code search over the whole codebase). Finds symbols, functions, classes, variables, strings, and code patterns, returning matching files with line numbers, code snippets, repository, language, and links. Start here for code search, finding where something is implemented or used, and exploring repositories.",
		Parameters: map[string]string{
			"query":            "Text to find in file contents. Matched literally (substring) unless regex is true. Do not escape quotes",
			"regex":            "Treat query as a regular expression (zoekt/RE2 syntax). Default: false",
			"case_sensitive":   "Match case exactly. Default: false",
			"repos":            "Optional list of repository names to search (e.g. [\"github.com/acme/api\"])",
			"languages":        "Optional list of languages to search, using Linguist names (e.g. [\"Go\", \"TypeScript\"])",
			"file_paths":       "Optional list of file path fragments to restrict matches to (e.g. [\"internal/billing/\", \"_test.go\"])",
			"ref":              "Optional branch, tag, or commit to search instead of the default branch",
			"max_matches":      "Maximum number of matches to return (1-1000). Default: 50",
			"context_lines":    "Lines of context around each match (0-20). Default: 3",
			"include_snippets": "Return matching code snippets. Set false to list only files and match counts. Default: true",
		},
		Required: []string{"query"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_list_repos"),
		Description: "List repositories (repos) indexed by Sourcebot code search, with default branch, last push and index time, fork/archived flags, and code host links. Use to discover repository names for search_code, read_file, list_tree, and list_commits filters.",
		Parameters: map[string]string{
			"query":     "Optional case-insensitive filter on repository name",
			"page":      "Page number (1-based). Default: 1",
			"per_page":  "Repositories per page (1-100). Default: 30",
			"sort":      "name or pushed (most recent commit). Default: name",
			"direction": "asc or desc. Default: asc",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_read_file"),
		Description: "Read (get) the source code contents of a file in a repository indexed by Sourcebot, optionally only a line range, with the total line count and links. Use after search_code or list_tree to view a source file.",
		Parameters: map[string]string{
			"repo":       repoParam,
			"path":       "File path relative to the repository root (e.g. internal/billing/invoice.go)",
			"ref":        refParam,
			"start_line": "Optional first line to return (1-based, inclusive)",
			"end_line":   "Optional last line to return (1-based, inclusive)",
		},
		Required: []string{"repo", "path"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_list_tree"),
		Description: "List files and directories (folder tree) in a repository indexed by Sourcebot, from the root or a subdirectory, to a chosen depth. Use to explore a codebase layout and find file paths for read_file.",
		Parameters: map[string]string{
			"repo":                repoParam,
			"path":                "Directory path relative to the repository root. Default: the root",
			"ref":                 refParam,
			"depth":               "Directory levels to walk below path (1-10). Default: 1",
			"include_files":       "Include files. Default: true",
			"include_directories": "Include directories. Default: true",
			"max_entries":         "Maximum entries to return (1-10000). Default: 1000",
		},
		Required: []string{"repo"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_list_commits"),
		Description: "List git commits (history) for a repository indexed by Sourcebot, newest first, with hash, author, date, and message. Filter by message text, author, date range, branch, or file path to find when and why code changed.",
		Parameters: map[string]string{
			"repo":     repoParam,
			"query":    "Optional case-insensitive filter on commit message",
			"author":   "Optional case-insensitive filter on author name or email",
			"since":    "Optional start date, ISO 8601 (2024-01-31) or relative (30 days ago)",
			"until":    "Optional end date, ISO 8601 or relative (yesterday)",
			"ref":      "Optional branch, tag, or commit to list history from. Default: the default branch",
			"path":     "Optional file path; only commits touching it (newer Sourcebot releases; older ones ignore it)",
			"page":     "Page number (1-based). Default: 1",
			"per_page": "Commits per page (1-100). Default: 30",
		},
		Required: []string{"repo"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_find_references"),
		Description: "Find references (usages, callers) of a code symbol such as a function, method, class, type, or variable across repositories indexed by Sourcebot, returning each file and line. Use after search_code to see where a symbol is used.",
		Parameters: map[string]string{
			"symbol":   "Exact symbol name (e.g. ComputeInvoiceTotal)",
			"language": "Language of the symbol, using the Linguist name Sourcebot reports in search results (e.g. Go, TypeScript, Python, Java)",
			"repo":     "Optional repository name to limit the lookup to",
			"ref":      "Optional branch or tag. Default: the default branch",
		},
		Required: []string{"symbol", "language"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_find_definitions"),
		Description: "Find the definition (declaration, go to definition) of a code symbol such as a function, method, class, type, or variable across repositories indexed by Sourcebot, returning the defining file and line. Use after search_code or find_references.",
		Parameters: map[string]string{
			"symbol":   "Exact symbol name (e.g. ComputeInvoiceTotal)",
			"language": "Language of the symbol, using the Linguist name Sourcebot reports in search results (e.g. Go, TypeScript, Python, Java)",
			"repo":     "Optional repository name to limit the lookup to",
			"ref":      "Optional branch or tag. Default: the default branch",
		},
		Required: []string{"symbol", "language"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_list_language_models"),
		Description: "List the AI language models configured on the Sourcebot instance for Ask Sourcebot. Use before ask_codebase to pick a provider and model; an empty list means ask_codebase is unavailable.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        mcp.ToolName("sourcebot_ask_codebase"),
		Description: "Ask Sourcebot's AI agent a natural-language question about the codebase. It searches code, reads source files, and follows symbol references, then answers in markdown with file links and a chat URL. Slow (often 30-60+ seconds) and needs a language model configured on the instance; prefer search_code for direct lookups.",
		Parameters: map[string]string{
			"query":          "Question about the code (e.g. How are invoice totals computed?)",
			"repos":          "Optional list of repository names the agent may use. Default: all repositories",
			"model_provider": "Optional language model provider from list_language_models (set with model)",
			"model":          "Optional language model ID from list_language_models (set with model_provider). Default: the first configured model",
			"visibility":     "Optional chat visibility: PRIVATE or PUBLIC (anyone with the link). Default: PRIVATE for API key users",
		},
		Required: []string{"query"},
		Access:   mcp.AccessRead,
	},
}
