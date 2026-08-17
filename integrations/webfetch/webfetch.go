package webfetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var _ mcp.Integration = (*webfetch)(nil)

const (
	maxTimeout  = 30
	defTimeout  = 10
	maxRedirect = 5
	maxBody     = 40_000
	userAgent   = "switchboard-mcp/1.0 (web_fetch; +https://github.com/daltoniam/switchboard)"
)

type webfetch struct {
	client           *http.Client
	allowPrivateAddr bool // testing only: bypass private address check
}

func New() mcp.Integration {
	return &webfetch{
		client: &http.Client{
			Timeout: time.Duration(defTimeout) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirect {
					return fmt.Errorf("too many redirects (max %d)", maxRedirect)
				}
				return nil
			},
		},
	}
}

func (w *webfetch) Name() string { return "web" }

func (w *webfetch) Configure(_ context.Context, _ mcp.Credentials) error {
	return nil
}

func (w *webfetch) Tools() []mcp.ToolDefinition { return tools }

func (w *webfetch) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: "unknown tool: " + string(toolName), IsError: true}, nil
	}
	return fn(ctx, w, args)
}

func (w *webfetch) Healthy(_ context.Context) bool { return true }

type handlerFunc func(ctx context.Context, w *webfetch, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"web_fetch": fetchURL,
}

// fetchURL fetches a URL and returns the content as readable text.
func fetchURL(ctx context.Context, w *webfetch, args map[string]any) (*mcp.ToolResult, error) {
	rawURL, err := mcp.ArgStr(args, "url")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if rawURL == "" {
		return &mcp.ToolResult{Data: "error: url parameter is required", IsError: true}, nil
	}

	timeout, _ := mcp.ArgInt(args, "timeout")
	if timeout <= 0 {
		timeout = defTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return &mcp.ToolResult{Data: "error: invalid URL: " + err.Error(), IsError: true}, nil
	}
	if parsed.Scheme != "https" {
		return &mcp.ToolResult{Data: "error: only https:// URLs are supported", IsError: true}, nil
	}

	if !w.allowPrivateAddr && isPrivateHost(parsed.Hostname()) {
		return &mcp.ToolResult{Data: "error: requests to private/local addresses are not allowed", IsError: true}, nil
	}

	client := *w.client
	client.Timeout = time.Duration(timeout) * time.Second

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return &mcp.ToolResult{Data: "error: " + err.Error(), IsError: true}, nil
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html, text/plain, application/json, text/markdown, */*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return &mcp.ToolResult{Data: "error: " + err.Error(), IsError: true}, nil
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return &mcp.ToolResult{
			Data:    fmt.Sprintf("error: HTTP %d — %s at %s", resp.StatusCode, http.StatusText(resp.StatusCode), rawURL),
			IsError: true,
		}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return &mcp.ToolResult{Data: "error: reading response: " + err.Error(), IsError: true}, nil
	}

	content := string(body)
	truncated := false
	if len(body) > maxBody {
		content = content[:maxBody]
		truncated = true
	}

	finalURL := resp.Request.URL.String()
	ct := resp.Header.Get("Content-Type")

	if isPlainText(ct, finalURL) {
		content = strings.TrimSpace(content)
	} else {
		content = extractReadableText(content)
	}

	var sb strings.Builder
	sb.WriteString("Source: ")
	sb.WriteString(finalURL)
	sb.WriteString("\nFetched: ")
	sb.WriteString(time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("\n\n")
	sb.WriteString(content)
	if truncated {
		sb.WriteString("\n\n[truncated — content exceeded limit]")
	}

	return &mcp.ToolResult{Data: sb.String()}, nil
}

// isPrivateHost checks if the hostname is a private/local address.
func isPrivateHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// isPlainText returns true if the content type or URL suggests plain text.
func isPlainText(ct, rawURL string) bool {
	ct = strings.ToLower(ct)
	if strings.Contains(ct, "text/plain") || strings.Contains(ct, "text/markdown") || strings.Contains(ct, "application/json") {
		return true
	}
	lower := strings.ToLower(rawURL)
	return strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".csv") ||
		strings.Contains(lower, "raw.githubusercontent.com")
}

var (
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// noiseBlockTags lists HTML tags whose entire content is noise.
var noiseBlockTags = []string{"script", "style", "nav", "footer", "header", "aside"}

// stripTagBlock removes all occurrences of <tag ...>...</tag> (case-insensitive).
func stripTagBlock(s, tag string) string {
	lower := strings.ToLower(s)
	for {
		open := strings.Index(lower, "<"+tag)
		if open == -1 {
			break
		}
		gt := strings.Index(lower[open:], ">")
		if gt == -1 {
			break
		}
		close := strings.Index(lower[open:], "</"+tag+">")
		if close == -1 {
			close = strings.Index(lower[open:], "</"+tag+" ")
		}
		if close == -1 {
			s = s[:open] + s[open+gt+1:]
			lower = strings.ToLower(s)
			continue
		}
		end := close + len("</"+tag+">")
		s = s[:open] + s[open+end:]
		lower = strings.ToLower(s)
	}
	return s
}

// extractCodeBlocks pulls content from <pre> and <code> tags, replacing them with placeholders.
func extractCodeBlocks(s string) (string, []string) {
	var blocks []string
	for _, tag := range []string{"pre", "code"} {
		lower := strings.ToLower(s)
		for {
			open := strings.Index(lower, "<"+tag)
			if open == -1 {
				break
			}
			gt := strings.Index(lower[open:], ">")
			if gt == -1 {
				break
			}
			closeTag := "</" + tag + ">"
			close := strings.Index(lower[open+gt:], closeTag)
			if close == -1 {
				break
			}
			contentStart := open + gt + 1
			contentEnd := open + gt + close
			inner := s[contentStart:contentEnd]
			inner = reTag.ReplaceAllString(inner, "")
			inner = strings.TrimSpace(inner)
			ph := fmt.Sprintf("\x00CODEBLOCK_%d\x00", len(blocks))
			blocks = append(blocks, inner)
			s = s[:open] + ph + s[contentEnd+len(closeTag):]
			lower = strings.ToLower(s)
		}
	}
	return s, blocks
}

// extractReadableText strips HTML, preserving code blocks.
func extractReadableText(html string) string {
	for _, tag := range noiseBlockTags {
		html = stripTagBlock(html, tag)
	}

	cleaned, codeBlocks := extractCodeBlocks(html)

	cleaned = reTag.ReplaceAllString(cleaned, "")

	for i, block := range codeBlocks {
		cleaned = strings.ReplaceAll(cleaned, fmt.Sprintf("\x00CODEBLOCK_%d\x00", i), "\n\n"+block+"\n\n")
	}

	cleaned = reBlankLines.ReplaceAllString(cleaned, "\n\n")

	return strings.TrimSpace(cleaned)
}
