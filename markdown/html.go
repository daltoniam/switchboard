package markdown

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// headingLevel maps heading atoms to their Markdown prefix.
var headingLevel = map[atom.Atom]string{
	atom.H1: "# ",
	atom.H2: "## ",
	atom.H3: "### ",
	atom.H4: "#### ",
	atom.H5: "##### ",
	atom.H6: "###### ",
}

// HTMLToMarkdown converts an HTML string to Markdown.
// Handles standard HTML elements plus Confluence-specific ac: namespace macros
// (code blocks, info/warning/note/tip panels).
func FromHTML(s string) Markdown {
	if s == "" {
		return ""
	}

	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Body,
		Data:     "body",
	})
	if err != nil {
		return Markdown(s)
	}

	var sb strings.Builder
	for _, n := range nodes {
		renderNode(&sb, n, 0, false)
	}
	return Markdown(sb.String())
}

// renderNode recursively converts an HTML node to Markdown.
// depth tracks list nesting for indentation. inPre suppresses block-level
// formatting inside <pre> elements.
func renderNode(sb *strings.Builder, n *html.Node, depth int, inPre bool) {
	switch n.Type {
	case html.TextNode:
		text := n.Data
		if !inPre {
			text = collapseWhitespace(text)
		}
		sb.WriteString(text)
		return
	case html.ElementNode:
		// handled below
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNode(sb, c, depth, inPre)
		}
		return
	}

	// Confluence ac: namespace macros.
	if isConfluenceMacro(n) {
		renderMacro(sb, n)
		return
	}

	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		prefix := headingLevel[n.DataAtom]
		sb.WriteString(prefix)
		renderChildren(sb, n, depth, false)
		sb.WriteString("\n\n")

	case atom.P:
		renderChildren(sb, n, depth, inPre)
		sb.WriteString("\n\n")

	case atom.Strong, atom.B:
		sb.WriteString("**")
		renderChildren(sb, n, depth, inPre)
		sb.WriteString("**")

	case atom.Em, atom.I:
		sb.WriteString("*")
		renderChildren(sb, n, depth, inPre)
		sb.WriteString("*")

	case atom.Del, atom.S:
		sb.WriteString("~~")
		renderChildren(sb, n, depth, inPre)
		sb.WriteString("~~")

	case atom.Code:
		if !inPre {
			sb.WriteString("`")
			renderChildren(sb, n, depth, inPre)
			sb.WriteString("`")
		} else {
			renderChildren(sb, n, depth, true)
		}

	case atom.A:
		href := getAttr(n, "href")
		sb.WriteString("[")
		renderChildren(sb, n, depth, inPre)
		fmt.Fprintf(sb, "](%s)", href)

	case atom.Ul:
		renderList(sb, n, depth, false)
		if depth == 0 {
			sb.WriteString("\n")
		}

	case atom.Ol:
		renderList(sb, n, depth, true)
		if depth == 0 {
			sb.WriteString("\n")
		}

	case atom.Li:
		renderChildren(sb, n, depth, inPre)

	case atom.Pre:
		sb.WriteString("```\n")
		renderChildren(sb, n, depth, true)
		sb.WriteString("\n```\n\n")

	case atom.Br:
		sb.WriteString("\n")

	case atom.Hr:
		sb.WriteString("---\n\n")

	case atom.Table:
		renderTable(sb, n)
		sb.WriteString("\n")

	default:
		renderChildren(sb, n, depth, inPre)
	}
}

func renderChildren(sb *strings.Builder, n *html.Node, depth int, inPre bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNode(sb, c, depth, inPre)
	}
}

func renderList(sb *strings.Builder, n *html.Node, depth int, ordered bool) {
	idx := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Li {
			continue
		}
		idx++

		indent := strings.Repeat("  ", depth)
		if ordered {
			fmt.Fprintf(sb, "%s%d. ", indent, idx)
		} else {
			sb.WriteString(indent + "- ")
		}

		endsWithNestedList := false
		for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
			if gc.Type == html.ElementNode && (gc.DataAtom == atom.Ul || gc.DataAtom == atom.Ol) {
				sb.WriteString("\n")
				renderNode(sb, gc, depth+1, false)
				endsWithNestedList = gc.NextSibling == nil
			} else {
				renderNode(sb, gc, depth, false)
			}
		}
		if !endsWithNestedList {
			sb.WriteString("\n")
		}
	}
}

func renderTable(sb *strings.Builder, n *html.Node) {
	var rows [][]string
	forEachElement(n, atom.Tr, func(tr *html.Node) {
		var cells []string
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.DataAtom == atom.Th || c.DataAtom == atom.Td {
				var cell strings.Builder
				renderChildren(&cell, c, 0, false)
				cells = append(cells, strings.TrimSpace(cell.String()))
			}
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	})

	WriteTable(sb, rows)
}

func forEachElement(n *html.Node, a atom.Atom, fn func(*html.Node)) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == a {
			fn(c)
		} else {
			forEachElement(c, a, fn)
		}
	}
}

func isConfluenceMacro(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "ac:structured-macro"
}

func renderMacro(sb *strings.Builder, n *html.Node) {
	macroName := getAttr(n, "ac:name")

	switch macroName {
	case "code":
		lang := getMacroParam(n, "language")
		body := getMacroPlainTextBody(n)
		fmt.Fprintf(sb, "```%s\n%s\n```\n\n", strings.ToLower(lang), body)

	case "info", "warning", "note", "tip":
		label := strings.ToUpper(macroName[:1]) + macroName[1:]
		body := getMacroRichTextBody(n)
		body = strings.TrimRight(body, "\n")
		fmt.Fprintf(sb, "> **%s:** %s\n\n", label, body)

	default:
		renderChildren(sb, n, 0, false)
	}
}

func getMacroParam(n *html.Node, name string) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "ac:parameter" && getAttr(c, "ac:name") == name {
			return textContent(c)
		}
	}
	return ""
}

func getMacroPlainTextBody(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "ac:plain-text-body" {
			return textContent(c)
		}
	}
	return ""
}

func getMacroRichTextBody(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "ac:rich-text-body" {
			var sb strings.Builder
			renderChildren(&sb, c, 0, false)
			return sb.String()
		}
	}
	return ""
}

// textContent returns concatenated text content of a node and its descendants.
// Handles CDATA sections which the HTML parser converts to CommentNodes.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			sb.WriteString(n.Data)
		case html.CommentNode:
			if strings.HasPrefix(n.Data, "[CDATA[") && strings.HasSuffix(n.Data, "]]") {
				sb.WriteString(n.Data[len("[CDATA[") : len(n.Data)-len("]]")])
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// collapseWhitespace replaces runs of whitespace with a single space.
func collapseWhitespace(s string) string {
	if s == "" {
		return ""
	}
	var sb strings.Builder
	inSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inSpace {
				sb.WriteRune(' ')
			}
			inSpace = true
			continue
		}
		inSpace = false
		sb.WriteRune(r)
	}
	return sb.String()
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		attrKey := a.Key
		if a.Namespace != "" {
			attrKey = a.Namespace + ":" + a.Key
		}
		if attrKey == key {
			return a.Val
		}
	}
	return ""
}
