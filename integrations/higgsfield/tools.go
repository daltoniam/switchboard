package higgsfield

import mcp "github.com/daltoniam/switchboard"

// dispatch maps each exposed tool to its upstream hosted MCP tool name. Only
// these tools are forwarded; every other upstream tool is refused locally.
var dispatch = map[mcp.ToolName]string{
	// Account and credits
	"higgsfield_balance":                "balance",
	"higgsfield_transactions":           "transactions",
	"higgsfield_show_plans_and_credits": "show_plans_and_credits",
	"higgsfield_list_workspaces":        "list_workspaces",
	"higgsfield_get_preferences":        "get_preferences",
	// Catalog discovery
	"higgsfield_models_explore": "models_explore",
	"higgsfield_get_presets":    "get_presets",
	"higgsfield_list_voices":    "list_voices",
	// History and jobs
	"higgsfield_show_generations": "show_generations",
	"higgsfield_jobs_wait":        "jobs_wait",
	"higgsfield_job_display":      "job_display",
	// Projects
	"higgsfield_list_projects":       "list_projects",
	"higgsfield_get_project":         "get_project",
	"higgsfield_list_project_assets": "list_project_assets",
	"higgsfield_create_project":      "create_project",
	// Media inputs
	"higgsfield_media_import_url": "media_import_url",
	"higgsfield_media_upload":     "media_upload",
	"higgsfield_media_confirm":    "media_confirm",
	// Paid generation and edits
	"higgsfield_generate_image":    "generate_image",
	"higgsfield_generate_video":    "generate_video",
	"higgsfield_generate_audio":    "generate_audio",
	"higgsfield_upscale_image":     "upscale_image",
	"higgsfield_remove_background": "remove_background",
}

const (
	workspaceIDParam = "Workspace UUID from higgsfield_list_workspaces (the private workspace or a team workspace)."
	cursorParam      = "Pagination cursor: next_cursor from the previous response."
	contextParam     = "Optional one-sentence note on the user's goal (analytics only; no names, credentials, or URLs)."
	costHint         = " Costs credits: first call with params.get_cost=true to get the price without submitting, and check higgsfield_balance. On a timeout do not resubmit; poll returned job IDs with higgsfield_jobs_wait."
	generationParams = "Object (required). Keys: model (required, ID from higgsfield_models_explore); prompt; aspect_ratio (e.g. 1:1, 16:9, 9:16); count (1-4 variants of the same prompt); medias (array of {value, role}: value is a media_id from higgsfield_media_import_url/higgsfield_media_confirm or a prior job_id, never a URL; role from the model's medias[].roles); folder_id (project default_folder_id); get_cost (true = price preflight, submits nothing); use_unlim (true only when the user asks to spend free-trial unlimited generations); plus model-specific parameters listed by higgsfield_models_explore."
)

var tools = []mcp.ToolDefinition{
	// ── Account and credits ──────────────────────────────────────────
	{
		Name:        "higgsfield_balance",
		Description: "Get Higgsfield credit balance and subscription plan for AI image, video, and audio generation. Start here: check remaining credits before any paid generation, then use higgsfield_models_explore to pick a model.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},
	{
		Name:        "higgsfield_transactions",
		Description: "List Higgsfield credit transactions (spend, refund, grant, deduct), newest first, to see what generations cost. Use after higgsfield_balance.",
		Parameters: map[string]string{
			"cursor": cursorParam,
			"size":   "Transactions per page, 1-100 (default 10).",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "higgsfield_show_plans_and_credits",
		Description: "Show Higgsfield subscription plans, pricing, and credit top-up packs with checkout links. Read-only; it never purchases anything.",
		Parameters: map[string]string{
			"intent": "Purchase path to prioritize: general (default), upgrade, topup, auto_refill, or trial.",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "higgsfield_list_workspaces",
		Description: "List Higgsfield workspaces (private and team) with plan, credits, and which one is selected. Use to get the workspace_id that project tools require.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},
	{
		Name:        "higgsfield_get_preferences",
		Description: "Get the saved Higgsfield project preference (auto_create_project). Read before the first generation to decide whether to create a project destination.",
		Parameters:  map[string]string{},
		Access:      mcp.AccessRead,
	},

	// ── Catalog discovery ────────────────────────────────────────────
	{
		Name:        "higgsfield_models_explore",
		Description: "Find Higgsfield generation models for images, videos, audio, and 3D: list, search, recommend for a goal, or get one model's parameters, aspect ratios, durations, and media roles. Use before higgsfield_generate_image, higgsfield_generate_video, or higgsfield_generate_audio.",
		Parameters: map[string]string{
			"action":   "Required: list, search, get, or recommend.",
			"query":    "Model or use-case query for search/recommend; include input context (text-only, reference image, image-to-video).",
			"model_id": "Model ID (required for get).",
			"type":     "Filter by output type: image, video, audio, or 3d.",
			"input":    "Filter by input: text (text-only) or image (accepts reference media).",
			"limit":    "Max results, 1-100 (default 20; 5 for recommend).",
			"after":    "Pagination cursor: next_page_token from a previous list/search.",
			"unlim":    "true = only models that accept free-trial unlimited generations.",
		},
		Required: []string{"action"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "higgsfield_get_presets",
		Description: "Browse or search Higgsfield preset and template galleries (Viral effects, Marketing Studio, Genjutsu motion, Soul styles, Katana). Browsing never starts a generation.",
		Parameters: map[string]string{
			"source":    "Gallery: viral, marketing_studio, genjutsu, soul, recraft, katana, what_if, or shorts_studio. Omit for the combined feed.",
			"category":  "Category slug returned by this tool (e.g. product-shot, motion, genjutsu-trending).",
			"query":     "Search words for preset names and descriptions (not supported for katana).",
			"preset_id": "Open one published preset's details by ID or slug.",
			"limit":     "Presets per page, 1-50 (default 12).",
			"cursor":    cursorParam,
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "higgsfield_list_voices",
		Description: "List Higgsfield speech voices (built-in presets and custom voices) with voice_id, voice_type, and preview audio URL. Use before higgsfield_generate_audio for text-to-speech.",
		Parameters: map[string]string{
			"cursor": cursorParam,
			"size":   "Voices per page, 1-100 (default 20).",
		},
		Access: mcp.AccessRead,
	},

	// ── History and jobs ─────────────────────────────────────────────
	{
		Name:        "higgsfield_show_generations",
		Description: "List past Higgsfield generations (images, videos, audio) with job id, status, model, prompt, and result URL. Pass a job id as a medias value to reuse a result.",
		Parameters: map[string]string{
			"type":           "Filter by media type: image, video, audio, or 3d.",
			"only_completed": "Default true. false includes queued and in-progress jobs.",
			"size":           "Results per page, 1-100 (default 24).",
			"cursor":         cursorParam,
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "higgsfield_jobs_wait",
		Description: "Wait for 1-12 Higgsfield generation jobs to finish and return their status and result URLs. Use after a generate tool returns job IDs.",
		Parameters: map[string]string{
			"jobs":            "Array of {index, job_id} from the generation response (1-12 items).",
			"timeout_seconds": "Long-poll budget 0-15 seconds (default 15; 0 = immediate snapshot).",
		},
		Required: []string{"jobs"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "higgsfield_job_display",
		Description: "Get one previous Higgsfield generation job by ID, including its result. Use after higgsfield_show_generations or higgsfield_list_project_assets.",
		Parameters: map[string]string{
			"id":        "Job ID (item_id from higgsfield_list_project_assets).",
			"folder_id": "Folder context from higgsfield_list_project_assets for jobs in shared folders.",
		},
		Required: []string{"id"},
		Access:   mcp.AccessRead,
	},

	// ── Projects ─────────────────────────────────────────────────────
	{
		Name:        "higgsfield_list_projects",
		Description: "List or search Higgsfield media projects with project_id and default_folder_id (the destination for generations).",
		Parameters: map[string]string{
			"workspace_id": workspaceIDParam,
			"search":       "Case-insensitive project name search.",
			"size":         "Page size, 1-50 (default 20).",
			"cursor":       cursorParam,
		},
		Required: []string{"workspace_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "higgsfield_get_project",
		Description: "Get one Higgsfield media project's metadata and capabilities. Use after higgsfield_list_projects.",
		Parameters: map[string]string{
			"workspace_id": workspaceIDParam,
			"project_id":   "Project UUID from higgsfield_list_projects.",
		},
		Required: []string{"workspace_id", "project_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "higgsfield_list_project_assets",
		Description: "List generations and uploaded media files in a Higgsfield project (metadata only). Use after higgsfield_list_projects; open a job with higgsfield_job_display.",
		Parameters: map[string]string{
			"workspace_id": workspaceIDParam,
			"project_id":   "Project UUID from higgsfield_list_projects.",
			"folder_id":    "Limit to one folder (without subfolders).",
			"output_kinds": "Array filter: image, video, audio, file, unknown.",
			"size":         "Page size, 1-50 (default 20).",
			"cursor":       cursorParam,
		},
		Required: []string{"workspace_id", "project_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "higgsfield_create_project",
		Description: "Create a private Higgsfield media project and return its default_folder_id to pass as folder_id on generations. Only on explicit request or when higgsfield_get_preferences says auto_create_project.",
		Parameters: map[string]string{
			"workspace_id":    workspaceIDParam,
			"name":            "Project name (1-100 characters).",
			"idempotency_key": "Unique key chosen once for this creation; reuse it on retries.",
		},
		Required: []string{"workspace_id", "name", "idempotency_key"},
		Access:   mcp.AccessWrite,
	},

	// ── Media inputs ─────────────────────────────────────────────────
	{
		Name:        "higgsfield_media_import_url",
		Description: "Import an HTTPS image, video, or audio URL into Higgsfield storage and return a media_id for use as a reference input in generations (max 50 MB).",
		Parameters: map[string]string{
			"url":  "HTTPS URL of the media file.",
			"type": "Expected media type: auto (default), image, video, or audio.",
		},
		Required: []string{"url"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_media_upload",
		Description: "Get presigned upload URLs for local image, video, or audio files. PUT the bytes to each upload_url, then call higgsfield_media_confirm to get usable media IDs.",
		Parameters: map[string]string{
			"filename":     "Single filename; the media type is inferred from the extension.",
			"content_type": "MIME type for a single file (e.g. image/png).",
			"files":        "Batch: array of 1-20 {filename, content_type}.",
		},
		Access: mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_media_confirm",
		Description: "Confirm Higgsfield media uploads after every PUT to the presigned URL returned HTTP 200. Use after higgsfield_media_upload.",
		Parameters: map[string]string{
			"type":      "Required: image, video, audio, or file.",
			"media_id":  "Single media ID to confirm.",
			"media_ids": "Array of 1-20 media IDs to confirm.",
		},
		Required: []string{"type"},
		Access:   mcp.AccessWrite,
	},

	// ── Paid generation and edits ────────────────────────────────────
	{
		Name:        "higgsfield_generate_image",
		Description: "Generate AI images from a text prompt and optional reference images with Higgsfield models (default gpt_image_2_6; soul_2 for portraits and fashion; native GIF/SVG via output_format)." + costHint,
		Parameters: map[string]string{
			"params":  generationParams,
			"context": contextParam,
		},
		Required: []string{"params"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_generate_video",
		Description: "Generate AI videos (text-to-video, image-to-video, motion transfer) with Higgsfield models such as seedance_2_5 and kling3_0. params also takes duration in seconds." + costHint,
		Parameters: map[string]string{
			"params":  generationParams + " Video adds duration (seconds).",
			"context": contextParam,
		},
		Required: []string{"params"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_generate_audio",
		Description: "Generate AI audio: text-to-speech voiceover (seed_audio with a voice from higgsfield_list_voices), music (suno_music), or sound effects (mirelo_text_to_audio)." + costHint,
		Parameters: map[string]string{
			"params":  "Object (required). Keys: model (required); prompt (required); voice_type (preset or element) and voice_id from higgsfield_list_voices for speech; duration in seconds (Suno 15-360, required for music and sound effects); medias; folder_id; get_cost (true = price preflight); use_unlim.",
			"context": contextParam,
		},
		Required: []string{"params"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_upscale_image",
		Description: "Upscale and enhance an existing Higgsfield image to 2K or 4K resolution." + costHint,
		Parameters: map[string]string{
			"params": "Object (required). Keys: image_id (required; media_id or completed image job_id); width and height (required; source pixels); resolution (2k or 4k, default 4k); get_cost (true = price preflight).",
		},
		Required: []string{"params"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "higgsfield_remove_background",
		Description: "Remove the background from an existing image or video to get a transparent cutout of the subject. Costs credits and has no price preflight (no get_cost), so confirm with the user and check higgsfield_balance first.",
		Parameters: map[string]string{
			"params": "Object (required). Keys: media_id (required; media_id or completed job_id); media_type (required; image or video).",
		},
		Required: []string{"params"},
		Access:   mcp.AccessWrite,
	},
}
