package sourcebot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultMaxMatches   = 50
	maxMaxMatches       = 1000
	defaultContextLines = 3
	maxContextLines     = 20
	defaultPerPage      = 30
	maxPerPage          = 100
	defaultTreeDepth    = 1
	maxTreeDepth        = 10
	defaultTreeEntries  = 1000
	maxTreeEntries      = 10000
)

func present(args map[string]any, key string) bool {
	v, ok := args[key]
	return ok && v != nil
}

func requireStr(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func queryString(params url.Values) string {
	for key, values := range params {
		if len(values) == 0 || values[0] == "" {
			params.Del(key)
		}
	}
	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
}

func clamp(value, lowest, highest int) int {
	return max(lowest, min(value, highest))
}

type location struct {
	LineNumber int `json:"lineNumber"`
}

type sourceRange struct {
	Start location `json:"start"`
}

type searchAPIResponse struct {
	Stats struct {
		ActualMatchCount int `json:"actualMatchCount"`
		TotalMatchCount  int `json:"totalMatchCount"`
		FileCount        int `json:"fileCount"`
	} `json:"stats"`
	Files []struct {
		FileName struct {
			Text string `json:"text"`
		} `json:"fileName"`
		WebURL         string   `json:"webUrl"`
		ExternalWebURL string   `json:"externalWebUrl"`
		Repository     string   `json:"repository"`
		RepositoryID   int      `json:"repositoryId"`
		Language       string   `json:"language"`
		Branches       []string `json:"branches"`
		Chunks         []struct {
			Content      string        `json:"content"`
			MatchRanges  []sourceRange `json:"matchRanges"`
			ContentStart location      `json:"contentStart"`
		} `json:"chunks"`
	} `json:"files"`
	IsSearchExhaustive bool `json:"isSearchExhaustive"`
}

type searchResult struct {
	Query              string       `json:"query"`
	MatchCount         int          `json:"matchCount"`
	TotalMatchCount    int          `json:"totalMatchCount"`
	FileCount          int          `json:"fileCount"`
	IsSearchExhaustive bool         `json:"isSearchExhaustive"`
	Files              []searchFile `json:"files"`
}

type searchFile struct {
	Repository     string        `json:"repository"`
	RepositoryID   int           `json:"repositoryId"`
	FileName       string        `json:"fileName"`
	Language       string        `json:"language,omitempty"`
	WebURL         string        `json:"webUrl"`
	ExternalWebURL string        `json:"externalWebUrl,omitempty"`
	Branches       []string      `json:"branches,omitempty"`
	MatchCount     int           `json:"matchCount"`
	Chunks         []searchChunk `json:"chunks,omitempty"`
}

type searchChunk struct {
	StartLine  int    `json:"startLine"`
	MatchLines []int  `json:"matchLines"`
	Content    string `json:"content"`
}

type searchFilters struct {
	regex     bool
	repos     []string
	languages []string
	filePaths []string
	ref       string
}

// buildSearchQuery turns a plain query plus filters into Sourcebot's zoekt
// query syntax, the same way Sourcebot's own MCP server does.
func buildSearchQuery(query string, f searchFilters) string {
	q := query
	if !f.regex {
		escaped := strings.ReplaceAll(query, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		q = `"` + escaped + `"`
	}
	if repos := nonEmpty(f.repos); len(repos) > 0 {
		q += " (repo:" + strings.Join(quoteAll(repos), " or repo:") + ")"
	}
	if langs := nonEmpty(f.languages); len(langs) > 0 {
		q += " (lang:" + strings.Join(langs, " or lang:") + ")"
	}
	if paths := nonEmpty(f.filePaths); len(paths) > 0 {
		q += " (file:" + strings.Join(quoteAll(paths), " or file:") + ")"
	}
	if ref := strings.TrimSpace(f.ref); ref != "" {
		q += " ( rev:" + ref + " )"
	}
	return q
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = regexp.QuoteMeta(v)
	}
	return out
}

func searchCode(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	filters := searchFilters{
		regex:     r.Bool("regex"),
		repos:     r.StrSlice("repos"),
		languages: r.StrSlice("languages"),
		filePaths: r.StrSlice("file_paths"),
		ref:       r.Str("ref"),
	}
	caseSensitive := r.Bool("case_sensitive")
	maxMatches := r.Int("max_matches")
	contextLines := r.Int("context_lines")
	includeSnippets := r.Bool("include_snippets")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("query", query); err != nil {
		return mcp.ErrResult(err)
	}
	if !present(args, "max_matches") {
		maxMatches = defaultMaxMatches
	}
	if maxMatches < 1 {
		return mcp.ErrResult(fmt.Errorf("max_matches must be at least 1"))
	}
	maxMatches = min(maxMatches, maxMaxMatches)
	if !present(args, "context_lines") {
		contextLines = defaultContextLines
	}
	if contextLines < 0 {
		return mcp.ErrResult(fmt.Errorf("context_lines must not be negative"))
	}
	contextLines = min(contextLines, maxContextLines)
	if !present(args, "include_snippets") {
		includeSnippets = true
	}

	zoektQuery := buildSearchQuery(query, filters)
	body := map[string]any{
		"query":                    zoektQuery,
		"matches":                  maxMatches,
		"contextLines":             contextLines,
		"isRegexEnabled":           filters.regex,
		"isCaseSensitivityEnabled": caseSensitive,
	}
	var resp searchAPIResponse
	if err := s.postJSON(ctx, s.client, "/api/search", body, &resp); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(toSearchResult(zoektQuery, resp, includeSnippets))
}

func toSearchResult(query string, resp searchAPIResponse, includeSnippets bool) searchResult {
	out := searchResult{
		Query:              query,
		MatchCount:         resp.Stats.ActualMatchCount,
		TotalMatchCount:    resp.Stats.TotalMatchCount,
		FileCount:          resp.Stats.FileCount,
		IsSearchExhaustive: resp.IsSearchExhaustive,
		Files:              make([]searchFile, 0, len(resp.Files)),
	}
	for _, f := range resp.Files {
		file := searchFile{
			Repository:     f.Repository,
			RepositoryID:   f.RepositoryID,
			FileName:       f.FileName.Text,
			Language:       f.Language,
			WebURL:         f.WebURL,
			ExternalWebURL: f.ExternalWebURL,
			Branches:       f.Branches,
		}
		for _, c := range f.Chunks {
			file.MatchCount += len(c.MatchRanges)
			if !includeSnippets {
				continue
			}
			file.Chunks = append(file.Chunks, searchChunk{
				StartLine:  c.ContentStart.LineNumber,
				MatchLines: matchLines(c.MatchRanges),
				Content:    c.Content,
			})
		}
		out.Files = append(out.Files, file)
	}
	return out
}

func matchLines(ranges []sourceRange) []int {
	seen := map[int]bool{}
	lines := make([]int, 0, len(ranges))
	for _, rg := range ranges {
		if n := rg.Start.LineNumber; !seen[n] {
			seen[n] = true
			lines = append(lines, n)
		}
	}
	sort.Ints(lines)
	return lines
}

type listResult struct {
	TotalCount int  `json:"totalCount"`
	Page       int  `json:"page"`
	PerPage    int  `json:"perPage"`
	HasMore    bool `json:"hasMore"`
}

func newListResult(header http.Header, page, perPage, returned int) listResult {
	total, ok := totalCount(header)
	if !ok {
		total = (page-1)*perPage + returned
	}
	return listResult{TotalCount: total, Page: page, PerPage: perPage, HasMore: page*perPage < total}
}

func listRepos(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", defaultPerPage)
	sortBy := r.Str("sort")
	direction := r.Str("direction")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	perPage = min(perPage, maxPerPage)
	if sortBy != "" && sortBy != "name" && sortBy != "pushed" {
		return mcp.ErrResult(fmt.Errorf("sort must be name or pushed"))
	}
	if direction != "" && direction != "asc" && direction != "desc" {
		return mcp.ErrResult(fmt.Errorf("direction must be asc or desc"))
	}
	params := url.Values{
		"query":     {query},
		"page":      {strconv.Itoa(page)},
		"perPage":   {strconv.Itoa(perPage)},
		"sort":      {sortBy},
		"direction": {direction},
	}
	var repos []json.RawMessage
	header, err := s.getJSON(ctx, "/api/repos"+queryString(params), &repos)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if repos == nil {
		repos = []json.RawMessage{}
	}
	return mcp.JSONResult(struct {
		Repos []json.RawMessage `json:"repos"`
		listResult
	}{repos, newListResult(header, page, perPage, len(repos))})
}

type fileSource struct {
	Source         string `json:"source"`
	Language       string `json:"language"`
	Path           string `json:"path"`
	Repo           string `json:"repo"`
	WebURL         string `json:"webUrl"`
	ExternalWebURL string `json:"externalWebUrl"`
}

type readFileResult struct {
	Repo           string `json:"repo"`
	Path           string `json:"path"`
	Language       string `json:"language,omitempty"`
	WebURL         string `json:"webUrl"`
	ExternalWebURL string `json:"externalWebUrl,omitempty"`
	TotalLines     int    `json:"totalLines"`
	StartLine      int    `json:"startLine"`
	EndLine        int    `json:"endLine"`
	Source         string `json:"source"`
}

// sliceLines returns lines start..end (1-based, inclusive) of src. A zero
// start or end means the beginning or end of the file.
func sliceLines(src string, start, end int) (text string, total, first, last int, err error) {
	lines := strings.SplitAfter(src, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total = len(lines)
	if start < 0 || end < 0 {
		return "", total, 0, 0, fmt.Errorf("start_line and end_line must be positive")
	}
	first = max(start, 1)
	last = end
	if last == 0 || last > total {
		last = total
	}
	if total == 0 {
		return "", 0, 0, 0, nil
	}
	if first > total {
		return "", total, 0, 0, fmt.Errorf("start_line %d is past the end of the file (%d lines)", first, total)
	}
	if last < first {
		return "", total, 0, 0, fmt.Errorf("end_line %d is before start_line %d", end, first)
	}
	return strings.Join(lines[first-1:last], ""), total, first, last, nil
}

func readFile(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	repo := r.Str("repo")
	path := r.Str("path")
	ref := r.Str("ref")
	start := r.Int("start_line")
	end := r.Int("end_line")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("repo", repo); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("path", path); err != nil {
		return mcp.ErrResult(err)
	}
	if start < 0 || end < 0 {
		return mcp.ErrResult(fmt.Errorf("start_line and end_line must be positive"))
	}
	params := url.Values{"repo": {repo}, "path": {strings.TrimPrefix(path, "/")}, "ref": {ref}}
	var file fileSource
	if _, err := s.getJSON(ctx, "/api/source"+queryString(params), &file); err != nil {
		return mcp.ErrResult(err)
	}
	text, total, first, last, err := sliceLines(file.Source, start, end)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(readFileResult{
		Repo:           file.Repo,
		Path:           file.Path,
		Language:       file.Language,
		WebURL:         file.WebURL,
		ExternalWebURL: file.ExternalWebURL,
		TotalLines:     total,
		StartLine:      first,
		EndLine:        last,
		Source:         text,
	})
}

type treeNode struct {
	Type     string     `json:"type"`
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Children []treeNode `json:"children"`
}

type treeEntry struct {
	Type       string `json:"type"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	ParentPath string `json:"parentPath"`
	Depth      int    `json:"depth"`
}

type treeResult struct {
	Repo          string      `json:"repo"`
	Ref           string      `json:"ref"`
	Path          string      `json:"path"`
	Entries       []treeEntry `json:"entries"`
	TotalReturned int         `json:"totalReturned"`
	Truncated     bool        `json:"truncated"`
}

type treeOptions struct {
	depth              int
	maxEntries         int
	includeFiles       bool
	includeDirectories bool
}

// treeFetcher returns the tree spanning the given directory paths, as
// Sourcebot's /api/tree does.
type treeFetcher func(ctx context.Context, paths []string) (treeNode, error)

func normalizeTreePath(p string) string {
	return strings.Trim(strings.TrimSpace(p), "/")
}

func joinTreePath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func indexTree(root *treeNode) map[string]*treeNode {
	index := map[string]*treeNode{}
	var visit func(node *treeNode, p string)
	visit = func(node *treeNode, p string) {
		index[p] = node
		for i := range node.Children {
			visit(&node.Children[i], joinTreePath(p, node.Children[i].Name))
		}
	}
	visit(root, "")
	return index
}

// walkTree lists entries below root breadth-first, fetching one tree per
// depth level so a deep listing costs depth requests rather than one per
// directory.
func walkTree(ctx context.Context, fetch treeFetcher, root string, opts treeOptions) ([]treeEntry, bool, error) {
	entries := []treeEntry{}
	if !opts.includeFiles && !opts.includeDirectories {
		return entries, false, nil
	}
	type pending struct {
		path  string
		depth int
	}
	queue := []pending{{root, 0}}
	queued := map[string]bool{root: true}
	seen := map[string]bool{}
	truncated := false

	for len(queue) > 0 && !truncated {
		depth := queue[0].depth
		var level []string
		for len(queue) > 0 && queue[0].depth == depth {
			level = append(level, queue[0].path)
			queue = queue[1:]
		}
		request := make([]string, 0, len(level))
		for _, p := range level {
			if p != "" {
				request = append(request, p)
			}
		}
		tree, err := fetch(ctx, request)
		if err != nil {
			return nil, false, err
		}
		index := indexTree(&tree)
		for _, dir := range level {
			node, ok := index[dir]
			if depth == 0 && dir != "" {
				if !ok {
					return nil, false, fmt.Errorf("directory %q not found", dir)
				}
				if node.Type != "tree" {
					return nil, false, fmt.Errorf("%q is a file, not a directory; use read_file", dir)
				}
			}
			if !ok || node.Type != "tree" {
				continue
			}
			for _, child := range node.Children {
				if child.Type != "tree" && child.Type != "blob" {
					continue
				}
				childPath := joinTreePath(dir, child.Name)
				childDepth := depth + 1
				if child.Type == "tree" && childDepth < opts.depth && !queued[childPath] {
					queue = append(queue, pending{childPath, childDepth})
					queued[childPath] = true
				}
				if (child.Type == "blob" && !opts.includeFiles) || (child.Type == "tree" && !opts.includeDirectories) {
					continue
				}
				key := child.Type + ":" + childPath
				if seen[key] {
					continue
				}
				seen[key] = true
				if len(entries) >= opts.maxEntries {
					truncated = true
					break
				}
				entries = append(entries, treeEntry{
					Type:       child.Type,
					Path:       childPath,
					Name:       child.Name,
					ParentPath: dir,
					Depth:      childDepth,
				})
			}
			if truncated {
				break
			}
		}
	}
	sortTreeEntries(entries)
	return entries, truncated, nil
}

func sortTreeEntries(entries []treeEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if pa, pb := strings.ToLower(a.ParentPath), strings.ToLower(b.ParentPath); pa != pb {
			return pa < pb
		}
		if a.Type != b.Type {
			return a.Type == "tree"
		}
		if na, nb := strings.ToLower(a.Name), strings.ToLower(b.Name); na != nb {
			return na < nb
		}
		return a.Path < b.Path
	})
}

func listTree(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	repo := r.Str("repo")
	path := r.Str("path")
	ref := r.Str("ref")
	depth := r.OptInt("depth", defaultTreeDepth)
	maxEntries := r.OptInt("max_entries", defaultTreeEntries)
	includeFiles := r.Bool("include_files")
	includeDirectories := r.Bool("include_directories")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("repo", repo); err != nil {
		return mcp.ErrResult(err)
	}
	if !present(args, "include_files") {
		includeFiles = true
	}
	if !present(args, "include_directories") {
		includeDirectories = true
	}
	if ref == "" {
		ref = "HEAD"
	}
	root := normalizeTreePath(path)
	opts := treeOptions{
		depth:              clamp(depth, 1, maxTreeDepth),
		maxEntries:         clamp(maxEntries, 1, maxTreeEntries),
		includeFiles:       includeFiles,
		includeDirectories: includeDirectories,
	}
	fetch := func(ctx context.Context, paths []string) (treeNode, error) {
		var resp struct {
			Tree treeNode `json:"tree"`
		}
		body := map[string]any{"repoName": repo, "revisionName": ref, "paths": paths}
		if err := s.postJSON(ctx, s.client, "/api/tree", body, &resp); err != nil {
			return treeNode{}, err
		}
		return resp.Tree, nil
	}
	entries, truncated, err := walkTree(ctx, fetch, root, opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(treeResult{
		Repo:          repo,
		Ref:           ref,
		Path:          root,
		Entries:       entries,
		TotalReturned: len(entries),
		Truncated:     truncated,
	})
}

// apiCommit accepts both author field spellings: Sourcebot 4.11 returns
// author_name/author_email and 4.17 returns authorName/authorEmail.
type apiCommit struct {
	Hash              string `json:"hash"`
	Date              string `json:"date"`
	Message           string `json:"message"`
	Refs              string `json:"refs"`
	Body              string `json:"body"`
	AuthorName        string `json:"authorName"`
	AuthorEmail       string `json:"authorEmail"`
	LegacyAuthorName  string `json:"author_name"`
	LegacyAuthorEmail string `json:"author_email"`
}

type commit struct {
	Hash        string `json:"hash"`
	Date        string `json:"date"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Message     string `json:"message"`
	Body        string `json:"body,omitempty"`
	Refs        string `json:"refs,omitempty"`
}

func normalizeCommit(c apiCommit) commit {
	out := commit{
		Hash:        c.Hash,
		Date:        c.Date,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Message:     c.Message,
		Body:        strings.TrimSpace(c.Body),
		Refs:        c.Refs,
	}
	if out.AuthorName == "" {
		out.AuthorName = c.LegacyAuthorName
	}
	if out.AuthorEmail == "" {
		out.AuthorEmail = c.LegacyAuthorEmail
	}
	return out
}

func listCommits(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	repo := r.Str("repo")
	query := r.Str("query")
	author := r.Str("author")
	since := r.Str("since")
	until := r.Str("until")
	ref := r.Str("ref")
	path := r.Str("path")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", defaultPerPage)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("repo", repo); err != nil {
		return mcp.ErrResult(err)
	}
	perPage = min(perPage, maxPerPage)
	params := url.Values{
		"repo":    {repo},
		"query":   {query},
		"author":  {author},
		"since":   {since},
		"until":   {until},
		"ref":     {ref},
		"path":    {path},
		"page":    {strconv.Itoa(page)},
		"perPage": {strconv.Itoa(perPage)},
	}
	var raw []apiCommit
	header, err := s.getJSON(ctx, "/api/commits"+queryString(params), &raw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	commits := make([]commit, len(raw))
	for i, c := range raw {
		commits[i] = normalizeCommit(c)
	}
	return mcp.JSONResult(struct {
		Commits []commit `json:"commits"`
		listResult
	}{commits, newListResult(header, page, perPage, len(commits))})
}

type symbolAPIResponse struct {
	Stats struct {
		MatchCount int `json:"matchCount"`
	} `json:"stats"`
	Files []struct {
		FileName     string `json:"fileName"`
		Repository   string `json:"repository"`
		RepositoryID int    `json:"repositoryId"`
		WebURL       string `json:"webUrl"`
		Language     string `json:"language"`
		Matches      []struct {
			LineContent string      `json:"lineContent"`
			Range       sourceRange `json:"range"`
		} `json:"matches"`
	} `json:"files"`
}

type symbolResult struct {
	Symbol     string       `json:"symbol"`
	Language   string       `json:"language"`
	MatchCount int          `json:"matchCount"`
	Files      []symbolFile `json:"files"`
}

type symbolFile struct {
	Repository   string        `json:"repository"`
	RepositoryID int           `json:"repositoryId"`
	FileName     string        `json:"fileName"`
	Language     string        `json:"language,omitempty"`
	WebURL       string        `json:"webUrl"`
	Matches      []symbolMatch `json:"matches"`
}

type symbolMatch struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func toSymbolResult(symbol, language string, resp symbolAPIResponse) symbolResult {
	out := symbolResult{Symbol: symbol, Language: language, MatchCount: resp.Stats.MatchCount, Files: make([]symbolFile, 0, len(resp.Files))}
	for _, f := range resp.Files {
		file := symbolFile{
			Repository:   f.Repository,
			RepositoryID: f.RepositoryID,
			FileName:     f.FileName,
			Language:     f.Language,
			WebURL:       f.WebURL,
			Matches:      make([]symbolMatch, 0, len(f.Matches)),
		}
		seen := map[int]bool{}
		for _, m := range f.Matches {
			line := m.Range.Start.LineNumber
			if seen[line] {
				continue
			}
			seen[line] = true
			file.Matches = append(file.Matches, symbolMatch{Line: line, Content: strings.TrimRight(m.LineContent, "\r\n")})
		}
		sort.Slice(file.Matches, func(i, j int) bool { return file.Matches[i].Line < file.Matches[j].Line })
		out.Files = append(out.Files, file)
	}
	return out
}

func findSymbol(ctx context.Context, s *sourcebot, args map[string]any, path string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	symbol := r.Str("symbol")
	language := r.Str("language")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("symbol", symbol); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("language", language); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"symbolName": strings.TrimSpace(symbol), "language": strings.TrimSpace(language)}
	if repo != "" {
		body["repoName"] = repo
	}
	if ref != "" {
		body["revisionName"] = ref
	}
	var resp symbolAPIResponse
	if err := s.postJSON(ctx, s.client, path, body, &resp); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(toSymbolResult(symbol, language, resp))
}

func findReferences(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	return findSymbol(ctx, s, args, "/api/find_references")
}

func findDefinitions(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	return findSymbol(ctx, s, args, "/api/find_definitions")
}

func listLanguageModels(ctx context.Context, s *sourcebot, _ map[string]any) (*mcp.ToolResult, error) {
	var models []json.RawMessage
	if _, err := s.getJSON(ctx, "/api/models", &models); err != nil {
		return mcp.ErrResult(err)
	}
	if models == nil {
		models = []json.RawMessage{}
	}
	return mcp.JSONResult(map[string]any{"models": models})
}

func askCodebase(ctx context.Context, s *sourcebot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	repos := r.StrSlice("repos")
	provider := r.Str("model_provider")
	model := r.Str("model")
	visibility := r.Str("visibility")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("query", query); err != nil {
		return mcp.ErrResult(err)
	}
	if (provider == "") != (model == "") {
		return mcp.ErrResult(fmt.Errorf("model_provider and model must be set together; see list_language_models"))
	}
	visibility = strings.ToUpper(strings.TrimSpace(visibility))
	if visibility != "" && visibility != "PRIVATE" && visibility != "PUBLIC" {
		return mcp.ErrResult(fmt.Errorf("visibility must be PRIVATE or PUBLIC"))
	}
	body := map[string]any{"query": query}
	if repos = nonEmpty(repos); len(repos) > 0 {
		body["repos"] = repos
	}
	if provider != "" {
		body["languageModel"] = map[string]string{"provider": provider, "model": model}
	}
	if visibility != "" {
		body["visibility"] = visibility
	}
	var resp json.RawMessage
	if err := s.postJSON(ctx, s.askClient, "/api/chat/blocking", body, &resp); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(resp)
}
