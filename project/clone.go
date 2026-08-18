package project

import "encoding/json"

func cloneDefinition(def *Definition) *Definition {
	if def == nil {
		return nil
	}
	out := &Definition{
		Schema:      def.Schema,
		Version:     def.Version,
		Name:        def.Name,
		Description: def.Description,
		baseDir:     def.baseDir,
	}
	out.Resources = cloneResources(def.Resources)
	out.Launch = cloneLaunch(def.Launch)
	out.Tools = cloneTools(def.Tools)
	out.Agents = cloneAgents(def.Agents)
	out.Extensions = cloneExtensions(def.Extensions)
	out.Additional = cloneAdditional(def.Additional)
	return out
}

func cloneResources(in map[string]Resource) map[string]Resource {
	if in == nil {
		return nil
	}
	out := make(map[string]Resource, len(in))
	for k, v := range in {
		out[k] = cloneResource(v)
	}
	return out
}

func cloneResource(r Resource) Resource {
	out := r
	if r.Include != nil {
		out.Include = append([]string(nil), r.Include...)
	}
	if r.Exclude != nil {
		out.Exclude = append([]string(nil), r.Exclude...)
	}
	return out
}

func cloneLaunch(in *LaunchConfig) *LaunchConfig {
	if in == nil {
		return nil
	}
	out := &LaunchConfig{
		Prompt:     in.Prompt,
		PromptFile: in.PromptFile,
	}
	if in.Env != nil {
		out.Env = make(map[string]string, len(in.Env))
		for k, v := range in.Env {
			out.Env[k] = v
		}
	}
	return out
}

func cloneTools(in map[string]*ScopeRule) map[string]*ScopeRule {
	if in == nil {
		return nil
	}
	out := make(map[string]*ScopeRule, len(in))
	for k, v := range in {
		out[k] = copyScopeRule(v)
	}
	return out
}

func cloneAgents(in *AgentsConfig) *AgentsConfig {
	if in == nil {
		return nil
	}
	out := &AgentsConfig{MaxConcurrent: in.MaxConcurrent}
	if in.Roles != nil {
		out.Roles = make(map[string]*RoleDefinition, len(in.Roles))
		for k, v := range in.Roles {
			out.Roles[k] = cloneRole(v)
		}
	}
	return out
}

func cloneExtensions(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneAdditional(in map[string]json.RawMessage) map[string]json.RawMessage {
	if in == nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(in))
	for k, v := range in {
		out[k] = append(json.RawMessage(nil), v...)
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
