package compact

import (
	"fmt"
	mcp "github.com/daltoniam/switchboard"
	"github.com/ohler55/ojg/gen"
	"sort"
	"strings"
)

// jsonRenderer is the framework default for FormatJSON. Marshals the
// projected JSON value with the shared codec — same shape as today's
// processResult output.
func jsonRenderer(projected gen.Node) ([]byte, error) {
	return mcp.MarshalJSONValue(projected)
}

// markdownRenderer is the framework default for FormatMarkdown. Renders any
// projected JSON value as readable markdown:
//
//   - flat map → definition list (- **key**: value)
//   - nested map → heading + recurse
//   - array of homogeneous maps → table
//   - array of scalars (or heterogeneous) → bullet list
//   - scalar → inline value
//
// Adapters with rich domain-specific markdown (Notion pages, Gmail threads)
// register a custom renderer via Options.Renderers instead of using this
// default. This is the "good enough" path for tools that don't need
// hand-tuned markdown.
func markdownRenderer(projected gen.Node) ([]byte, error) {
	var sb strings.Builder
	renderMarkdownValue(&sb, projected, 0)
	out := strings.TrimRight(sb.String(), "\n") + "\n"
	return []byte(out), nil
}

func renderMarkdownValue(sb *strings.Builder, v gen.Node, depth int) {
	switch val := v.(type) {
	case nil:
		sb.WriteString("_(none)_\n")
	case gen.Object:
		renderMarkdownMap(sb, val, depth)
	case gen.Array:
		renderMarkdownSlice(sb, val, depth)
	default:
		fmt.Fprintf(sb, "%s\n", displayValue(val))
	}
}

func renderMarkdownMap(sb *strings.Builder, m gen.Object, depth int) {
	keys := sortedKeys(m)
	for _, k := range keys {
		v := m[k]
		switch vv := v.(type) {
		case nil:
			// skip nil values
		case gen.Object, gen.Array:
			level := min(depth+2, 6)
			fmt.Fprintf(sb, "%s %s\n\n", strings.Repeat("#", level), k)
			renderMarkdownValue(sb, vv, depth+1)
			sb.WriteString("\n")
		default:
			fmt.Fprintf(sb, "- **%s**: %s\n", k, displayValue(vv))
		}
	}
}

func renderMarkdownSlice(sb *strings.Builder, s gen.Array, depth int) {
	if len(s) == 0 {
		sb.WriteString("_(empty)_\n")
		return
	}
	if first, ok := s[0].(gen.Object); ok && allMapsHomogeneous(s, first) {
		renderMarkdownTable(sb, s, first)
		return
	}
	for _, item := range s {
		switch vv := item.(type) {
		case gen.Object:
			sb.WriteString("- \n")
			renderMarkdownMap(sb, vv, depth+1)
		default:
			fmt.Fprintf(sb, "- %s\n", displayValue(vv))
		}
	}
}

func renderMarkdownTable(sb *strings.Builder, s gen.Array, first gen.Object) {
	keys := sortedKeys(first)
	sb.WriteString("| " + strings.Join(keys, " | ") + " |\n")
	sb.WriteString("|")
	for range keys {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")
	for _, item := range s {
		m, ok := item.(gen.Object)
		if !ok {
			continue
		}
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = displayValue(m[k])
		}
		sb.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	sb.WriteString("\n")
}

func displayValue(node gen.Node) string {
	switch value := node.(type) {
	case nil:
		return "<nil>"
	case gen.String:
		return string(value)
	case gen.Big:
		return string(value)
	case gen.Object:
		return displayObject(value)
	case gen.Array:
		return displayArray(value)
	default:
		return fmt.Sprint(value)
	}
}

func displayObject(object gen.Object) string {
	parts := make([]string, 0, len(object))
	for _, key := range sortedKeys(object) {
		parts = append(parts, key+":"+displayValue(object[key]))
	}
	return fmt.Sprintf("map[%s]", strings.Join(parts, " "))
}

func displayArray(array gen.Array) string {
	parts := make([]string, len(array))
	for i, value := range array {
		parts[i] = displayValue(value)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, " "))
}

func sortedKeys(m gen.Object) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func allMapsHomogeneous(s gen.Array, first gen.Object) bool {
	firstKeys := sortedKeys(first)
	for _, item := range s {
		m, ok := item.(gen.Object)
		if !ok {
			return false
		}
		if len(m) != len(first) {
			return false
		}
		for _, k := range firstKeys {
			if _, present := m[k]; !present {
				return false
			}
		}
	}
	return true
}

// resolveRenderer picks the renderer for one (tool, view, format) combo at
// load time. Adapter-provided renderers (Options.Renderers) override the
// framework defaults; an undeclared format with no adapter renderer
// returns an error.
func resolveRenderer(key RenderKey, opts Options) (Renderer, error) {
	if r, ok := opts.Renderers[key]; ok {
		return r, nil
	}
	switch key.Format {
	case FormatJSON:
		return jsonRenderer, nil
	case FormatMarkdown:
		return markdownRenderer, nil
	case FormatText:
		return nil, fmt.Errorf("format %q has no framework default; provide a custom renderer via Options.Renderers", key.Format)
	default:
		return nil, fmt.Errorf("unknown format %q (known: %s, %s, %s)",
			key.Format, FormatJSON, FormatMarkdown, FormatText)
	}
}
