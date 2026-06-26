package prompts

import (
	"bytes"
	"embed"
	"strings"
	"text/template"
)

//go:embed dynamic/*.md.tmpl
var dynamicFS embed.FS

//go:embed meta/*.md.tmpl
var metaFS embed.FS

var dynamicTmpl = template.Must(
	template.New("dynamic").Delims("<%", "%>").ParseFS(dynamicFS, "dynamic/*.md.tmpl"),
)

var metaTmpl = template.Must(
	template.New("meta").Delims("<%", "%>").ParseFS(metaFS, "meta/*.md.tmpl"),
)

// Context carries request-scoped data for template authors.
// Empty in v1; add fields only when a concrete template needs them.
type Context struct{}

func render(t *template.Template, name string, data any) string {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		panic(err)
	}
	return strings.TrimRight(buf.String(), "\n")
}
