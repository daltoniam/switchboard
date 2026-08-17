package project

import "encoding/json"

func cloneDefinition(def *Definition) *Definition {
	if def == nil {
		return nil
	}
	out := &Definition{
		Schema:  def.Schema,
		Version: def.Version,
		Name:    def.Name,
		Repo:    def.Repo,
		Branch:  def.Branch,
	}
	if def.Launch != nil {
		out.Launch = &LaunchConfig{
			Prompt:     def.Launch.Prompt,
			PromptFile: def.Launch.PromptFile,
		}
		if def.Launch.Env != nil {
			out.Launch.Env = make(map[string]string, len(def.Launch.Env))
			for k, v := range def.Launch.Env {
				out.Launch.Env[k] = v
			}
		}
	}
	if def.Tools != nil {
		out.Tools = make(map[string]*ScopeRule, len(def.Tools))
		for k, v := range def.Tools {
			out.Tools[k] = copyScopeRule(v)
		}
	}
	if def.Context != nil {
		out.Context = &ContextConfig{
			MaxBytes: def.Context.MaxBytes,
		}
		out.Context.Files = append([]string(nil), def.Context.Files...)
		out.Context.RepoIncludes = append([]string(nil), def.Context.RepoIncludes...)
	}
	if def.Agents != nil {
		out.Agents = &AgentsConfig{MaxConcurrent: def.Agents.MaxConcurrent}
		if def.Agents.Roles != nil {
			out.Agents.Roles = make(map[string]*RoleDefinition, len(def.Agents.Roles))
			for k, v := range def.Agents.Roles {
				out.Agents.Roles[k] = cloneRole(v)
			}
		}
	}
	if def.Extensions != nil {
		out.Extensions = make(map[string]any, len(def.Extensions))
		for k, v := range def.Extensions {
			out.Extensions[k] = v
		}
	}
	if def.Additional != nil {
		out.Additional = make(map[string]json.RawMessage, len(def.Additional))
		for k, v := range def.Additional {
			out.Additional[k] = append(json.RawMessage(nil), v...)
		}
	}
	return out
}

func cloneRole(r *RoleDefinition) *RoleDefinition {
	if r == nil {
		return nil
	}
	out := &RoleDefinition{Description: r.Description}
	if r.ToolOverrides != nil {
		out.ToolOverrides = make(map[string]*ScopeRule, len(r.ToolOverrides))
		for k, v := range r.ToolOverrides {
			out.ToolOverrides[k] = copyScopeRule(v)
		}
	}
	if r.ContextOverrides != nil {
		out.ContextOverrides = &ContextConfig{MaxBytes: r.ContextOverrides.MaxBytes}
		out.ContextOverrides.Files = append([]string(nil), r.ContextOverrides.Files...)
		out.ContextOverrides.RepoIncludes = append([]string(nil), r.ContextOverrides.RepoIncludes...)
	}
	return out
}

func cloneSnapshot(in Snapshot) Snapshot {
	out := in
	out.Definition = *cloneDefinition(&in.Definition)
	out.Sources = append([]Source(nil), in.Sources...)
	out.Diagnostics = append([]Diagnostic(nil), in.Diagnostics...)
	out.UserBytes = append([]byte(nil), in.UserBytes...)
	return out
}

func cloneRevisionSnapshot(in RevisionSnapshot) RevisionSnapshot {
	out := in
	out.Definition = *cloneDefinition(&in.Definition)
	return out
}

func cloneDiagnostics(in []Diagnostic) []Diagnostic {
	if in == nil {
		return []Diagnostic{}
	}
	return append([]Diagnostic(nil), in...)
}
