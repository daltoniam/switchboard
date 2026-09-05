package recoll

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"golang.org/x/net/html"
)

var tools = []mcp.ToolDefinition{
	{
		Name:        "recoll_search",
		Description: "Search indexed documents, files, PDFs, emails, and their full text through Recoll WebUI. Start here to find local documents and search desktop file contents.",
		Parameters: map[string]string{
			"query": "Search query passed to the Recoll WebUI",
		},
		Required: []string{"query"},
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"recoll_search": search,
}

func search(ctx context.Context, r *recoll, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	query := reader.Str("query")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(query) == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}

	data, err := r.get(ctx, "/?"+url.Values{"q": {query}}.Encode())
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("search Recoll documents: %w", err))
	}

	result, err := parseSearchResults(data, query)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("parse Recoll search results: %w", err))
	}
	return mcp.JSONResult(result)
}

type searchResponse struct {
	Query   string         `json:"query"`
	Count   int            `json:"count"`
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Path     string `json:"path"`
	MIMEType string `json:"mime_type,omitempty"`
}

var resultCountPattern = regexp.MustCompile(`(?i)\b([0-9]+)\s+result\(s\)\s+for\b`)

func parseSearchResults(data []byte, query string) (searchResponse, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return searchResponse{}, fmt.Errorf("parse HTML: %w", err)
	}

	response := searchResponse{Query: query}
	forEachNode(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if hasClass(n, "result") {
			response.Results = append(response.Results, searchResult{
				Path:     textForClass(n, "path"),
				MIMEType: decodeMIMETypes(textForClass(n, "meta")),
			})
			return
		}
		if n.Data == "p" && response.Count == 0 {
			match := resultCountPattern.FindStringSubmatch(nodeText(n))
			if len(match) == 2 {
				response.Count, _ = strconv.Atoi(match[1])
			}
		}
	})
	if response.Count == 0 {
		response.Count = len(response.Results)
	}
	return response, nil
}

func forEachNode(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		forEachNode(child, fn)
	}
}

func hasClass(n *html.Node, want string) bool {
	for _, attr := range n.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, class := range strings.Fields(attr.Val) {
			if class == want {
				return true
			}
		}
	}
	return false
}

func textForClass(n *html.Node, class string) string {
	var text string
	forEachNode(n, func(child *html.Node) {
		if text == "" && child.Type == html.ElementNode && hasClass(child, class) {
			text = nodeText(child)
		}
	})
	return text
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	forEachNode(n, func(child *html.Node) {
		if child.Type == html.TextNode {
			b.WriteString(child.Data)
		}
	})
	return strings.TrimSpace(b.String())
}

func decodeMIMETypes(encoded string) string {
	for _, value := range strings.Fields(encoded) {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			continue
		}
		mimeType := string(decoded)
		if strings.Contains(mimeType, "/") {
			return mimeType
		}
	}
	return ""
}
