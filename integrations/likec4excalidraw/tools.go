package likec4excalidraw

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "likec4excalidraw_get_architecture",
		Description: "Get the current LikeC4 architecture model, including elements, relationships, views, and diagnostics. Start here to inspect or navigate an architecture diagram before editing it.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "likec4excalidraw_get_scene",
		Description: "Get the persisted Excalidraw scene, including canvas elements, files, application state, and the active LikeC4 view. Use when raw canvas geometry or presentation metadata is needed.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "likec4excalidraw_get_diagnostics",
		Description: "Get validation diagnostics for the current Excalidraw canvas and LikeC4 architecture model. Use to find architecture errors and warnings before or after edits.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "likec4excalidraw_validate",
		Description: "Generate and validate LikeC4 DSL from the current canvas without changing the project. Use before architecture mutations or persistence to inspect diagnostics and generated source.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "likec4excalidraw_get_canvas_screenshot",
		Description: "Get a PNG screenshot of the whole LikeC4 Excalidraw architecture diagram or a scene-coordinate area. Use for visual inspection after get_architecture or an edit.",
		Parameters: map[string]string{
			"scope": "Screenshot scope: whole or area. Defaults to whole.",
			"area":  "Scene-coordinate object with x, y, width, and height. Required when scope is area; width and height must be positive and at most 4096.",
			"scale": "Render scale from 0.25 through 2. Defaults to 1.",
		},
	},
	{
		Name:        "likec4excalidraw_create_element",
		Description: "Create a managed actor, system, or service on the LikeC4 Excalidraw canvas and persist validated architecture DSL. Use after get_architecture to choose a unique semantic FQN and existing parent or view.",
		Parameters: map[string]string{
			"fqn":    "Unique semantic fully qualified name for the architecture element.",
			"kind":   "Element kind: actor, system, or service.",
			"title":  "Non-empty visible and semantic title.",
			"parent": "Optional semantic parent FQN.",
			"viewId": "Target LikeC4 view ID. Defaults to index.",
			"x":      "Canvas X coordinate. Defaults to 80.",
			"y":      "Canvas Y coordinate. Defaults to 80.",
			"width":  "Positive element width. Defaults to 180.",
			"height": "Positive element height. Defaults to 100.",
		},
		Required: []string{"fqn", "kind", "title"},
	},
	{
		Name:        "likec4excalidraw_update_element",
		Description: "Update semantic metadata for every canvas presentation of a managed LikeC4 element and persist validated architecture DSL. Use after get_architecture with an existing element FQN.",
		Parameters: map[string]string{
			"fqn":         "Existing semantic fully qualified name to update.",
			"title":       "Optional non-empty title.",
			"kind":        "Optional element kind: actor, system, or service.",
			"technology":  "Optional technology label.",
			"description": "Optional architecture element description.",
			"tags":        "Optional array of LikeC4 tags.",
		},
		Required: []string{"fqn"},
	},
	{
		Name:        "likec4excalidraw_delete_element",
		Description: "Delete every canvas presentation of a managed LikeC4 element and persist validated architecture DSL. Use after get_architecture and verify the semantic FQN before this destructive action.",
		Parameters:  map[string]string{"fqn": "Existing semantic fully qualified name to delete."},
		Required:    []string{"fqn"},
	},
	{
		Name:        "likec4excalidraw_create_relationship",
		Description: "Create a named relationship arrow between two managed LikeC4 elements in a view and persist validated architecture DSL. Use after get_architecture; both endpoint presentations must exist in the requested view.",
		Parameters: map[string]string{
			"source":     "Source element FQN.",
			"target":     "Target element FQN.",
			"title":      "Non-empty relationship title.",
			"relationId": "Optional unique relationship ID; generated when omitted.",
			"viewId":     "View containing both element presentations. Defaults to index.",
		},
		Required: []string{"source", "target", "title"},
	},
	{
		Name:        "likec4excalidraw_create_view",
		Description: "Create a managed Excalidraw frame exported as a LikeC4 architecture view and persist validated DSL. Use after get_architecture to choose a unique view ID.",
		Parameters: map[string]string{
			"viewId": "Unique LikeC4 view ID.",
			"title":  "Non-empty view title.",
			"x":      "Frame X coordinate. Defaults to 0.",
			"y":      "Frame Y coordinate. Defaults to 0.",
			"width":  "Positive frame width. Defaults to 800.",
			"height": "Positive frame height. Defaults to 600.",
		},
		Required: []string{"viewId", "title"},
	},
}

var supportedTools = func() map[mcp.ToolName]struct{} {
	names := make(map[mcp.ToolName]struct{}, len(tools))
	for _, tool := range tools {
		names[tool.Name] = struct{}{}
	}
	return names
}()
