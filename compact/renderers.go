package compact

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// jsonRenderer is the framework default for FormatJSON. Marshals the
// projected Go value with json.Marshal — same shape as today's
// processResult output.
func jsonRenderer(projected any) ([]byte, error) {
	return json.Marshal(projected)
}

// markdownRenderer is the framework default for FormatMarkdown. Renders any
// projected Go value as readable markdown:
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
func markdownRenderer(projected any) ([]byte, error) {
	var sb strings.Builder
	renderMarkdownValue(&sb, projected, 0)
	out := strings.TrimRight(sb.String(), "\n") + "\n"
	return []byte(out), nil
}

func renderMarkdownValue(sb *strings.Builder, v any, depth int) {
	switch val := v.(type) {
	case nil:
		sb.WriteString("_(none)_\n")
	case map[string]any:
		renderMarkdownMap(sb, val, depth)
	case []any:
		renderMarkdownSlice(sb, val, depth)
	default:
		fmt.Fprintf(sb, "%v\n", val)
	}
}

func renderMarkdownMap(sb *strings.Builder, m map[string]any, depth int) {
	keys := sortedKeys(m)
	for _, k := range keys {
		v := m[k]
		switch vv := v.(type) {
		case nil:
			// skip nil values
		case map[string]any, []any:
			level := min(depth+2, 6)
			fmt.Fprintf(sb, "%s %s\n\n", strings.Repeat("#", level), k)
			renderMarkdownValue(sb, vv, depth+1)
			sb.WriteString("\n")
		default:
			fmt.Fprintf(sb, "- **%s**: %v\n", k, vv)
		}
	}
}

func renderMarkdownSlice(sb *strings.Builder, s []any, depth int) {
	if len(s) == 0 {
		sb.WriteString("_(empty)_\n")
		return
	}
	if first, ok := s[0].(map[string]any); ok && allMapsHomogeneous(s, first) {
		renderMarkdownTable(sb, s, first)
		return
	}
	for _, item := range s {
		switch vv := item.(type) {
		case map[string]any:
			sb.WriteString("- \n")
			renderMarkdownMap(sb, vv, depth+1)
		default:
			fmt.Fprintf(sb, "- %v\n", vv)
		}
	}
}

func renderMarkdownTable(sb *strings.Builder, s []any, first map[string]any) {
	keys := sortedKeys(first)
	sb.WriteString("| " + strings.Join(keys, " | ") + " |\n")
	sb.WriteString("|")
	for range keys {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")
	for _, item := range s {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = fmt.Sprintf("%v", m[k])
		}
		sb.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	sb.WriteString("\n")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func allMapsHomogeneous(s []any, first map[string]any) bool {
	firstKeys := sortedKeys(first)
	for _, item := range s {
		m, ok := item.(map[string]any)
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
