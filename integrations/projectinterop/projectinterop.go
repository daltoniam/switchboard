package projectinterop

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
)

type projectInterop struct {
	store *project.Store
}

type handlerFunc func(context.Context, *projectInterop, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	"projectinterop_list_projects":  listProjects,
	"projectinterop_get_project":    getProject,
	"projectinterop_create_project": createProject,
	"projectinterop_update_project": updateProject,
	"projectinterop_delete_project": deleteProject,
	"projectinterop_get_context":    getContext,
}

var _ mcp.FieldCompactionIntegration = (*projectInterop)(nil)
var _ mcp.PlainTextCredentials = (*projectInterop)(nil)

func New() mcp.Integration {
	return &projectInterop{}
}

func (p *projectInterop) Name() string { return "projectinterop" }

func (p *projectInterop) Configure(creds mcp.Credentials) error {
	root := strings.TrimSpace(creds["config_root"])
	if root == "" {
		root = project.DefaultConfigDir()
	}
	p.store = project.NewStore(project.ExpandHome(root))
	if err := p.store.Load(); err != nil {
		return fmt.Errorf("projectinterop: load config root: %w", err)
	}
	return nil
}

func (p *projectInterop) Healthy(context.Context) bool {
	return p.store != nil
}

func (p *projectInterop) Tools() []mcp.ToolDefinition { return tools }

func (p *projectInterop) PlainTextKeys() []string { return []string{"config_root"} }

func (p *projectInterop) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (p *projectInterop) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[toolName]
	if !ok {
		return errResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	if p.store == nil {
		return errResult(mcp.ErrNotConfigured)
	}
	return handler(ctx, p, args)
}

func listProjects(_ context.Context, p *projectInterop, _ map[string]any) (*mcp.ToolResult, error) {
	type summary struct {
		Name   string `json:"name"`
		Repo   string `json:"repo,omitempty"`
		Branch string `json:"branch,omitempty"`
	}

	all := p.store.All()
	projects := make([]summary, 0, len(all))
	for _, definition := range all {
		projects = append(projects, summary{
			Name:   definition.Name,
			Repo:   definition.Repo,
			Branch: definition.Branch,
		})
	}
	slices.SortFunc(projects, func(a, b summary) int {
		return strings.Compare(a.Name, b.Name)
	})
	return jsonResult(projects)
}

func getProject(_ context.Context, p *projectInterop, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredString(args, "name")
	if err != nil {
		return errResult(err)
	}
	definition, ok := p.store.Get(name)
	if !ok {
		return errResult(fmt.Errorf("project %q not found", name))
	}
	return jsonResult(definition)
}

func createProject(_ context.Context, p *projectInterop, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredString(args, "name")
	if err != nil {
		return errResult(err)
	}
	definition := &project.Definition{
		Version: "1",
		Name:    name,
		Repo:    argString(args, "repo"),
		Branch:  argString(args, "branch"),
	}
	if err := p.store.Create(definition); err != nil {
		return errResult(err)
	}
	return jsonResult(definition)
}

func updateProject(_ context.Context, p *projectInterop, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredString(args, "name")
	if err != nil {
		return errResult(err)
	}
	patch, ok := args["patch"]
	if !ok || patch == nil {
		return errResult(fmt.Errorf("patch is required"))
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return errResult(fmt.Errorf("invalid patch: %w", err))
	}
	updated, err := p.store.Update(name, data)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(updated)
}

func deleteProject(_ context.Context, p *projectInterop, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredString(args, "name")
	if err != nil {
		return errResult(err)
	}
	if err := p.store.Delete(name); err != nil {
		return errResult(err)
	}
	return rawResult(fmt.Sprintf("project %q deleted", name))
}

func getContext(_ context.Context, p *projectInterop, args map[string]any) (*mcp.ToolResult, error) {
	name, err := requiredString(args, "name")
	if err != nil {
		return errResult(err)
	}
	definition, ok := p.store.Get(name)
	if !ok {
		return errResult(fmt.Errorf("project %q not found", name))
	}
	if path := argString(args, "path"); path != "" {
		content, err := project.ReadContextFile(definition, p.store.ConfigDir(), path)
		if err != nil {
			return errResult(err)
		}
		return rawResult(content)
	}

	entries := project.AssembleManifestWithRole(definition, p.store.ConfigDir(), argString(args, "role"))
	if query := strings.ToLower(argString(args, "query")); query != "" {
		entries = slices.DeleteFunc(entries, func(entry project.ContextEntry) bool {
			return !strings.Contains(strings.ToLower(entry.Path), query)
		})
	}
	return jsonResult(entries)
}

func requiredString(args map[string]any, key string) (string, error) {
	value := argString(args, key)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func argString(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func jsonResult(value any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return errResult(err)
	}
	return rawResult(string(data))
}

func rawResult(data string) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: data}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}
