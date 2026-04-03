package stitch

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Projects ──────────────────────────────────────────────────
	{
		Name: "stitch_list_projects", Description: "List all Stitch UI design projects. Start here to find project IDs needed by other tools. Filter by owned or shared",
		Parameters: map[string]string{"filter": "Optional filter: 'view=owned' (default) or 'view=shared'"},
	},
	{
		Name: "stitch_create_project", Description: "Create a new Stitch project to hold UI screens and design systems",
		Parameters: map[string]string{"title": "Title for the new project"},
	},
	{
		Name: "stitch_get_project", Description: "Get details of a Stitch project including screen instances and design system info",
		Parameters: map[string]string{"name": "Resource name: projects/{project_id} (e.g. 'projects/4044680601076201931')"},
		Required:   []string{"name"},
	},

	// ── Screens ──────────────────────────────────────────────────
	{
		Name: "stitch_list_screens", Description: "List all screens in a Stitch project. Returns screen IDs needed for editing and variant generation",
		Parameters: map[string]string{"project_id": "Project ID (e.g. '4044680601076201931'), without 'projects/' prefix"},
		Required:   []string{"project_id"},
	},
	{
		Name: "stitch_get_screen", Description: "Get details of a specific screen including its design and metadata",
		Parameters: map[string]string{
			"name": "Resource name: projects/{project}/screens/{screen} (e.g. 'projects/123/screens/abc')",
		},
		Required: []string{"name"},
	},
	{
		Name: "stitch_generate_screen_from_text", Description: "Generate a new UI screen from a text prompt using AI. Takes a few minutes to complete",
		Parameters: map[string]string{
			"project_id":  "Project ID, without 'projects/' prefix",
			"prompt":      "Text description of the screen to generate",
			"device_type": "Target device: MOBILE, DESKTOP, TABLET, or AGNOSTIC",
			"model_id":    "AI model: GEMINI_3_FLASH or GEMINI_3_1_PRO",
		},
		Required: []string{"project_id", "prompt"},
	},
	{
		Name: "stitch_edit_screens", Description: "Edit existing screens using a text prompt. Modifies selected screens based on the instruction. Takes a few minutes",
		Parameters: map[string]string{
			"project_id":          "Project ID, without 'projects/' prefix",
			"selected_screen_ids": "JSON array of screen IDs to edit (e.g. ['id1','id2'])",
			"prompt":              "Edit instruction describing what to change",
			"device_type":         "Target device: MOBILE, DESKTOP, TABLET, or AGNOSTIC",
			"model_id":            "AI model: GEMINI_3_FLASH or GEMINI_3_1_PRO",
		},
		Required: []string{"project_id", "selected_screen_ids", "prompt"},
	},
	{
		Name: "stitch_generate_variants", Description: "Generate design variants of existing screens. Control creativity level and which aspects to vary",
		Parameters: map[string]string{
			"project_id":          "Project ID, without 'projects/' prefix",
			"selected_screen_ids": "JSON array of screen IDs to generate variants for",
			"prompt":              "Text prompt guiding the variant generation",
			"variant_count":       "Number of variants to generate (1-5, default 3)",
			"creative_range":      "Creativity level: REFINE, EXPLORE (default), or REIMAGINE",
			"aspects":             "Comma-separated aspects to vary: LAYOUT, COLOR_SCHEME, IMAGES, TEXT_FONT, TEXT_CONTENT",
			"device_type":         "Target device: MOBILE, DESKTOP, TABLET, or AGNOSTIC",
			"model_id":            "AI model: GEMINI_3_FLASH or GEMINI_3_1_PRO",
		},
		Required: []string{"project_id", "selected_screen_ids", "prompt"},
	},

	// ── Design Systems ───────────────────────────────────────────
	{
		Name: "stitch_list_design_systems", Description: "List design systems for a project, or list all global design systems if no project specified",
		Parameters: map[string]string{"project_id": "Project ID to scope the listing, or omit for global systems"},
	},
	{
		Name: "stitch_create_design_system", Description: "Create a new design system defining colors, fonts, roundness, and appearance for a project",
		Parameters: map[string]string{
			"project_id":    "Project ID to associate the design system with, or omit for global",
			"display_name":  "Display name for the design system",
			"color_mode":    "Appearance mode: LIGHT or DARK",
			"headline_font": "Headline font (e.g. INTER, ROBOTO, MANROPE, DM_SANS)",
			"body_font":     "Body font (e.g. INTER, ROBOTO, MANROPE, DM_SANS)",
			"roundness":     "Corner roundness: ROUND_FOUR, ROUND_EIGHT, ROUND_TWELVE, or ROUND_FULL",
			"custom_color":  "Primary/seed color in hex (e.g. '#6750A4')",
			"color_variant": "Color variant: MONOCHROME, NEUTRAL, TONAL_SPOT, VIBRANT, EXPRESSIVE",
			"design_md":     "Optional markdown design instructions",
		},
		Required: []string{"display_name", "color_mode", "headline_font", "body_font", "roundness", "custom_color"},
	},
	{
		Name: "stitch_update_design_system", Description: "Update an existing design system's theme, fonts, colors, or roundness",
		Parameters: map[string]string{
			"name":          "Resource name: assets/{asset_id} (e.g. 'assets/15996705518239280238')",
			"project_id":    "Project ID the design system belongs to",
			"display_name":  "Display name for the design system",
			"color_mode":    "Appearance mode: LIGHT or DARK",
			"headline_font": "Headline font (e.g. INTER, ROBOTO, MANROPE, DM_SANS)",
			"body_font":     "Body font (e.g. INTER, ROBOTO, MANROPE, DM_SANS)",
			"roundness":     "Corner roundness: ROUND_FOUR, ROUND_EIGHT, ROUND_TWELVE, or ROUND_FULL",
			"custom_color":  "Primary/seed color in hex (e.g. '#6750A4')",
			"color_variant": "Color variant: MONOCHROME, NEUTRAL, TONAL_SPOT, VIBRANT, EXPRESSIVE",
			"design_md":     "Optional markdown design instructions",
		},
		Required: []string{"name", "project_id", "display_name", "color_mode", "headline_font", "body_font", "roundness", "custom_color"},
	},
	{
		Name: "stitch_apply_design_system", Description: "Apply a design system's tokens (colors, fonts, shapes) to selected screen instances",
		Parameters: map[string]string{
			"project_id":                "Project ID of the screens",
			"asset_id":                  "Design system asset ID from list_design_systems, without 'assets/' prefix",
			"selected_screen_instances": "JSON array of objects with 'id' (screen instance ID) and 'source_screen' (resource name projects/{p}/screens/{s})",
		},
		Required: []string{"project_id", "asset_id", "selected_screen_instances"},
	},
}
