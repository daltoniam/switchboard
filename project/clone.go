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
	}
	out.Tools = cloneTools(def.Tools)
	out.Agents = cloneAgents(def.Agents)
	out.Additional = cloneAdditional(def.Additional)
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

func cloneAgents(a *AgentsConfig) *AgentsConfig {
	if a == nil {
		return nil
	}
	out := &AgentsConfig{MaxConcurrent: a.MaxConcurrent}
	if a.Roles != nil {
		out.Roles = make(map[string]*RoleDefinition, len(a.Roles))
		for k, v := range a.Roles {
			out.Roles[k] = cloneRole(v)
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
		out.ContextOverrides = &ContextConfig{
			MaxBytes:     r.ContextOverrides.MaxBytes,
			Files:        append([]string(nil), r.ContextOverrides.Files...),
			RepoIncludes: append([]string(nil), r.ContextOverrides.RepoIncludes...),
		}
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
