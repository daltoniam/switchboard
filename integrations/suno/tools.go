package suno

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Music Generation ───────────────────────────────────────────
	{
		Name:        "suno_generate_music",
		Description: "Generate a music track with Suno AI. Each request produces 2 songs. Start here for music creation workflows. Stream URL available in ~30s, download URL in ~2-3 min. Use suno_get_generation to poll status",
		Parameters: map[string]string{
			"prompt":        "Text description or lyrics for the song (500 chars non-custom, up to 5000 in custom mode)",
			"style":         "Music genre/style (e.g. pop, rock, jazz, folk, synthwave). Required in custom mode",
			"title":         "Song title (max 80-100 chars depending on model). Required in custom mode",
			"model":         "Model version: V5, V4_5PLUS, V4_5ALL, V4_5, V4 (default: V4_5ALL)",
			"custom_mode":   "Enable custom mode for fine control over style/title/lyrics (true/false, default: true)",
			"instrumental":  "Generate instrumental only, no vocals (true/false, default: false)",
			"callback_url":  "Webhook URL for completion notification",
			"persona_id":    "Persona ID for personalized style",
			"negative_tags": "Styles to avoid (e.g. 'Heavy Metal, Upbeat Drums')",
			"vocal_gender":  "Vocal gender: m or f",
			"style_weight":  "Style influence weight 0-1 (default: 0.65)",
		},
		Required: []string{"prompt"},
	},
	{
		Name:        "suno_get_generation",
		Description: "Check the status of a music generation task. Returns track URLs when complete. Status values: PENDING, TEXT_SUCCESS, FIRST_SUCCESS, SUCCESS, FAILED",
		Parameters:  map[string]string{"task_id": "Task ID returned from suno_generate_music or other generation tools"},
		Required:    []string{"task_id"},
	},
	{
		Name:        "suno_extend_music",
		Description: "Extend an existing audio track with additional content. Creates a continuation from a specific timestamp",
		Parameters: map[string]string{
			"audio_id":           "ID of the audio track to extend",
			"prompt":             "Description or lyrics for the extension",
			"style":              "Music style for the extension",
			"title":              "Title for the extended track",
			"continue_at":        "Time in seconds to start extension from",
			"model":              "Model version: V5, V4_5PLUS, V4_5ALL, V4_5, V4 (default: V4_5ALL)",
			"use_default_params": "Use original track parameters instead of custom (true/false, default: false)",
			"callback_url":       "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_get_credits",
		Description: "Get the number of remaining generation credits for the authenticated account",
	},

	// ── Lyrics ─────────────────────────────────────────────────────
	{
		Name:        "suno_generate_lyrics",
		Description: "Generate song lyrics from a text prompt. Max 200 characters. Use suno_get_lyrics to poll for results",
		Parameters: map[string]string{
			"prompt":       "Description of desired lyrics (max 200 chars)",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"prompt"},
	},
	{
		Name:        "suno_get_lyrics",
		Description: "Get the status and result of a lyrics generation task",
		Parameters:  map[string]string{"task_id": "Task ID from suno_generate_lyrics"},
		Required:    []string{"task_id"},
	},
	{
		Name:        "suno_get_aligned_lyrics",
		Description: "Get timestamped/word-level aligned lyrics for an audio track. Useful for karaoke or synced display",
		Parameters:  map[string]string{"audio_id": "ID of the audio track"},
		Required:    []string{"audio_id"},
	},

	// ── Audio Processing ───────────────────────────────────────────
	{
		Name:        "suno_separate_stems",
		Description: "Separate an audio track into vocal and instrumental stems. Use suno_get_stem_separation to poll status",
		Parameters: map[string]string{
			"audio_id":     "ID of the audio track to separate",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_get_stem_separation",
		Description: "Get the status and URLs of a stem separation task. Returns vocal and instrumental track URLs when complete",
		Parameters:  map[string]string{"task_id": "Task ID from suno_separate_stems"},
		Required:    []string{"task_id"},
	},
	{
		Name:        "suno_convert_wav",
		Description: "Convert a generated audio track to WAV format. Use suno_get_wav_conversion to poll status",
		Parameters: map[string]string{
			"audio_id":     "ID of the audio track to convert",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_get_wav_conversion",
		Description: "Get the status and download URL of a WAV conversion task",
		Parameters:  map[string]string{"task_id": "Task ID from suno_convert_wav"},
		Required:    []string{"task_id"},
	},

	// ── Advanced Generation ────────────────────────────────────────
	{
		Name:        "suno_cover_audio",
		Description: "Create a cover version of uploaded audio with a new style and arrangement",
		Parameters: map[string]string{
			"upload_url":    "URL of the source audio file",
			"style":         "Target music style for the cover",
			"title":         "Title for the cover version",
			"prompt":        "Description or lyrics for the cover",
			"custom_mode":   "Enable custom mode (true/false, default: true)",
			"model":         "Model version (default: V4_5ALL)",
			"callback_url":  "Webhook URL for completion notification",
		},
		Required: []string{"upload_url"},
	},
	{
		Name:        "suno_upload_extend",
		Description: "Upload audio and extend it with AI-generated continuation",
		Parameters: map[string]string{
			"upload_url":   "URL of the audio file to extend",
			"prompt":       "Description or lyrics for the extension",
			"style":        "Music style for the extension",
			"title":        "Title for the extended track",
			"model":        "Model version (default: V4_5ALL)",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"upload_url"},
	},
	{
		Name:        "suno_add_vocals",
		Description: "Generate vocal tracks for instrumental music",
		Parameters: map[string]string{
			"audio_id":     "ID of the instrumental audio track",
			"prompt":       "Lyrics or vocal description",
			"style":        "Vocal style",
			"model":        "Model version (default: V4_5ALL)",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_add_instrumental",
		Description: "Generate instrumental accompaniment for a vocal track",
		Parameters: map[string]string{
			"audio_id":     "ID of the vocal audio track",
			"prompt":       "Instrumental description",
			"style":        "Instrumental style",
			"model":        "Model version (default: V4_5ALL)",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_generate_mashup",
		Description: "Generate a mashup combining elements from multiple tracks",
		Parameters: map[string]string{
			"audio_ids":    "Comma-separated list of audio IDs to mashup",
			"style":        "Target style for the mashup",
			"prompt":       "Description of the desired mashup",
			"model":        "Model version (default: V4_5ALL)",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_ids"},
	},

	// ── Persona ────────────────────────────────────────────────────
	{
		Name:        "suno_generate_persona",
		Description: "Create a personalized music persona based on generated tracks. Returns a persona_id for use in suno_generate_music",
		Parameters: map[string]string{
			"audio_ids":    "Comma-separated list of audio IDs to base the persona on",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_ids"},
	},

	// ── Video ──────────────────────────────────────────────────────
	{
		Name:        "suno_generate_video",
		Description: "Generate a music video from an audio track. Use suno_get_video to poll status",
		Parameters: map[string]string{
			"audio_id":     "ID of the audio track",
			"author":       "Author/artist name for the video",
			"domain_name":  "Brand domain name for the video",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
	{
		Name:        "suno_get_video",
		Description: "Get the status and URL of a video generation task",
		Parameters:  map[string]string{"task_id": "Task ID from suno_generate_video"},
		Required:    []string{"task_id"},
	},

	// ── MIDI ───────────────────────────────────────────────────────
	{
		Name:        "suno_generate_midi",
		Description: "Generate a MIDI file from an audio track",
		Parameters: map[string]string{
			"audio_id":     "ID of the audio track",
			"callback_url": "Webhook URL for completion notification",
		},
		Required: []string{"audio_id"},
	},
}
