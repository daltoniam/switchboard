package server

import (
	"cmp"
	"math"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// stopWords are common English function words (articles, prepositions,
// conjunctions) that appear in tool descriptions but carry no semantic
// signal for search. Filtered from both IDF indexing and query scoring.
//
// This is a closed linguistic class — English doesn't gain new
// prepositions. The list is write-once, not a maintenance burden.
var stopWords = map[string]bool{
	// Articles
	"a": true, "an": true, "the": true,
	// Prepositions
	"to": true, "for": true, "of": true, "in": true, "on": true,
	"at": true, "by": true, "from": true, "with": true, "as": true,
	"into": true, "about": true, "between": true, "through": true,
	// Conjunctions
	"and": true, "or": true, "but": true, "not": true,
	// Pronouns / determiners
	"is": true, "it": true, "its": true, "this": true, "that": true,
	"what": true, "which": true, "who": true, "how": true,
	// Common verbs too generic for search
	"be": true, "are": true, "was": true, "were": true, "been": true,
	"has": true, "have": true, "had": true, "do": true, "does": true, "did": true,
	"can": true, "will": true, "should": true,
}

// tokenize splits text into lowercase words on whitespace, underscores,
// and punctuation, filtering stop words. Tool names like
// "github_list_issues" become ["github", "list", "issues"].
//
// Pipeline order matters: ToLower must run before stop-word filtering
// because stopWords keys are lowercase. Do not reorder.
func tokenize(s string) []string {
	raw := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	result := raw[:0]
	for _, w := range raw {
		if !stopWords[w] {
			result = append(result, w)
		}
	}
	return result
}

// toolWithIntegration pairs a tool definition with its integration name
// for scoring. The tokens field is pre-computed at index time to avoid
// per-query tokenization (~900 map allocations eliminated per search).
type toolWithIntegration struct {
	Integration string
	Tool        mcp.ToolDefinition
	tokens      map[string]bool // pre-computed by computeIDF
}

// scoredResult is the output of scoring — a tool with its relevance score.
type scoredResult struct {
	toolWithIntegration
	Score float64
}

// SearchIndex holds the pre-computed search state built once at startup.
// Shared between Server and ProjectRouter as a single value.
type SearchIndex struct {
	IDF      map[string]float64
	SynMap   map[string][]string
	AllTools []toolWithIntegration
}

// toToolInfo converts a scored result to a search response entry.
// Copies parameters to avoid mutating the original tool definition.
func toToolInfo(r scoredResult) searchToolInfo {
	params := make(map[string]string, len(r.Tool.Parameters))
	for k, v := range r.Tool.Parameters {
		params[k] = v
	}
	return searchToolInfo{
		Integration: r.Integration,
		Name:        r.Tool.Name,
		Description: r.Tool.Description,
		Parameters:  params,
		Required:    r.Tool.Required,
	}
}

// toolDefToInfo converts a raw tool definition to a search response entry.
func toolDefToInfo(integration string, tool mcp.ToolDefinition) searchToolInfo {
	params := make(map[string]string, len(tool.Parameters))
	for k, v := range tool.Parameters {
		params[k] = v
	}
	return searchToolInfo{
		Integration: integration,
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  params,
		Required:    tool.Required,
	}
}

// synonymGroups defines equivalence sets of words that should match
// interchangeably in search. Each group is expanded bidirectionally
// at init time. Adding a synonym means appending to one slice.
//
// Plurals: the tokenizer does exact matching with NO stemming, so
// "errors" ≠ "error". Flatten plurals into the existing group that
// contains the singular form. Only create a standalone pair like
// {"metric", "metrics"} when the word isn't in any other group.
// Never put the same word in two groups — groups must be disjoint.
var synonymGroups = [][]string{
	// Nouns — domain concepts (plurals flattened into groups)
	{"ticket", "issue", "issues", "task", "bug"},
	{"repo", "repository"},
	{"pr", "pull_request", "pull", "merge_request"},
	{"message", "notification", "chat"},
	{"log", "logs", "event", "trace", "record"},
	{"error", "errors", "exception", "incident", "incidents", "failure"},
	{"deploy", "deploys", "deployment", "deployments", "release", "releases", "rollout", "ship"},
	{"alert", "alerts", "monitor", "monitors", "alarm", "warning"},
	{"workspace", "organization", "org", "team"},
	{"channel", "conversation", "room"},
	{"state", "status"},
	{"cycle", "sprint", "iteration"},
	{"dashboard", "dashboards", "board"},
	{"credential", "secret", "key", "token"},
	{"member", "user", "participant"},
	{"email", "mail", "gmail"},
	{"break", "fail", "crash", "down"},
	{"metric", "metrics"},
	{"table", "tables"},
	{"label", "labels", "tag", "tags"},
	{"diff", "patch", "changes"},
	{"column", "columns", "field", "fields"},
	{"schema", "schemas"},
	{"database", "databases", "db"},
	{"branch", "branches"},
	{"comment", "comments"},

	// Verbs — action synonyms
	{"create", "add", "new", "make", "generate"},
	{"draft", "compose", "write"},
	{"update", "edit", "modify", "change"},
	{"delete", "remove", "destroy"},
	{"find", "search", "query", "queries", "lookup", "discover", "filter"},
	{"send", "post", "publish"},
	{"execute", "run", "invoke", "trigger"},
	{"get", "retrieve", "fetch", "read", "show", "view", "describe"},
	{"list", "ls", "enumerate"},
}

// buildSynonymMap expands synonym groups into a self-inclusive lookup map.
// Each word maps to itself AND all other words in its group, so callers
// don't need to prepend the original word when expanding.
//
// Precondition: synonym groups must be disjoint (no word in multiple groups).
func buildSynonymMap(groups [][]string) map[string][]string {
	m := make(map[string][]string)
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		for _, word := range group {
			m[word] = append([]string(nil), group...)
		}
	}
	return m
}

// computeIDF builds the inverse document frequency map from the tool corpus
// and pre-computes each tool's token set (stored on toolWithIntegration.tokens).
// Only words appearing in at least one tool are indexed.
// Formula: IDF(word) = log(totalTools / toolsContainingWord)
// Missing keys at score time are treated as 0.0 (word contributes nothing).
func computeIDF(tools []toolWithIntegration) map[string]float64 {
	total := len(tools)
	if total == 0 {
		return nil
	}

	// Count how many tools contain each word. Reuse seen map across
	// iterations to avoid ~900 map allocations.
	docFreq := make(map[string]int)
	seen := make(map[string]bool)
	for i := range tools {
		clear(seen)
		searchable := tools[i].Tool.Name + " " + tools[i].Tool.Description + " " + tools[i].Integration
		words := tokenize(searchable)
		tokenSet := make(map[string]bool, len(words))
		for _, word := range words {
			tokenSet[word] = true
			if !seen[word] {
				docFreq[word]++
				seen[word] = true
			}
		}
		tools[i].tokens = tokenSet
	}

	idf := make(map[string]float64, len(docFreq))
	for word, count := range docFreq {
		idf[word] = math.Log(float64(total) / float64(count))
	}
	return idf
}

// scoreTool computes the TF-IDF relevance score for a single tool against
// pre-tokenized query words. The tool's token set and the synonym map are
// both pre-computed — this function does zero allocations.
//
// For each query word, the synonym map provides the full expansion
// (including self). The MAX score across variants is taken per original
// query word, then summed.
func scoreTool(queryWords []string, tool toolWithIntegration, idf map[string]float64, synMap map[string][]string) float64 {
	total := 0.0

	for _, qw := range queryWords {
		// synMap is self-inclusive: synMap["ticket"] = ["ticket","issue","task","bug"].
		// For words not in any group, the map returns nil so we check the word directly.
		variants := synMap[qw]
		if len(variants) == 0 {
			variants = []string{qw}
		}
		bestScore := 0.0

		for _, variant := range variants {
			if tool.tokens[variant] {
				if score := idf[variant]; score > bestScore {
					bestScore = score
				}
			}
		}

		total += bestScore
	}

	return total
}

// scoreTools scores all tools against a query, filters out zero-score
// results, and sorts by (score desc, integration asc, tool name asc).
func scoreTools(query string, tools []toolWithIntegration, idf map[string]float64, synMap map[string][]string) []scoredResult {
	queryWords := tokenize(query)
	if len(queryWords) == 0 {
		return nil
	}

	var results []scoredResult

	for _, ti := range tools {
		s := scoreTool(queryWords, ti, idf, synMap)
		if s > 0 {
			results = append(results, scoredResult{
				toolWithIntegration: ti,
				Score:               s,
			})
		}
	}

	slices.SortFunc(results, func(a, b scoredResult) int {
		// Score descending.
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		// Tiebreaker: integration ascending, then tool name ascending.
		if c := cmp.Compare(a.Integration, b.Integration); c != 0 {
			return c
		}
		return cmp.Compare(a.Tool.Name, b.Tool.Name)
	})

	return results
}
