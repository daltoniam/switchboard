package suno

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Music Generation ───────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_generate_music"),
		Description: "Generate a music track with Suno AI. Each request produces 2 songs. Start here for music creation workflows. Stream URL available in ~30s, download URL in ~2-3 min. Use suno_get_generation to poll status",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("prompt"), Description: "Text description or lyrics for the song (500 chars in non-custom mode, up to 5000 in custom mode)", Required: true}, {Name: mcp.ParamName("style"), Description: "Music genre/style (e.g. pop, rock, jazz, folk, synthwave). Required in custom mode"}, {Name: mcp.ParamName("title"), Description: "Song title (max 80-100 chars depending on model). Required in custom mode"}, {Name: mcp.ParamName("model"), Description: "Model version: V5, V4_5PLUS, V4_5ALL, V4_5, V4 (default: V4_5ALL)"}, {Name: mcp.ParamName("custom_mode"), Description: "Enable custom mode for fine control over style/title/lyrics (true/false, default: true)"}, {Name: mcp.ParamName("instrumental"), Description: "Generate instrumental only, no vocals (true/false, default: false)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}, {Name: mcp.ParamName("persona_id"), Description: "Persona ID for personalized style"}, {Name: mcp.ParamName("negative_tags"), Description: "Styles to avoid (e.g. 'Heavy Metal, Upbeat Drums')"}, {Name: mcp.ParamName("vocal_gender"), Description: "Vocal gender: m or f"}, {Name: mcp.ParamName("style_weight"), Description: "Style influence weight 0-1 (default: 0.65)"}},
	},
	{
		Name:        mcp.ToolName("suno_get_generation"),
		Description: "Check the status of a music generation task. Returns track URLs when complete. Status values: PENDING, TEXT_SUCCESS, FIRST_SUCCESS, SUCCESS, FAILED",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID returned from suno_generate_music or other generation tools", Required: true}},
	},
	{
		Name:        mcp.ToolName("suno_extend_music"),
		Description: "Extend an existing audio track with additional content. Creates a continuation from a specific timestamp",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track to extend", Required: true}, {Name: mcp.ParamName("prompt"), Description: "Description or lyrics for the extension"}, {Name: mcp.ParamName("style"), Description: "Music style for the extension"}, {Name: mcp.ParamName("title"), Description: "Title for the extended track"}, {Name: mcp.ParamName("continue_at"), Description: "Time in seconds to start extension from"}, {Name: mcp.ParamName("model"), Description: "Model version: V5, V4_5PLUS, V4_5ALL, V4_5, V4 (default: V4_5ALL)"}, {Name: mcp.ParamName("use_default_params"), Description: "Use original track parameters instead of custom (true/false, default: false)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_get_credits"),
		Description: "Get the number of remaining generation credits for the authenticated account",
	},

	// ── Lyrics ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_generate_lyrics"),
		Description: "Generate song lyrics from a text prompt. Max 200 characters. Use suno_get_lyrics to poll for results",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("prompt"), Description: "Description of desired lyrics (max 200 chars)", Required: true}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_get_lyrics"),
		Description: "Get the status and result of a lyrics generation task",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID from suno_generate_lyrics", Required: true}},
	},
	{
		Name:        mcp.ToolName("suno_get_aligned_lyrics"),
		Description: "Get timestamped/word-level aligned lyrics for an audio track. Useful for karaoke or synced display",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track", Required: true}},
	},

	// ── Audio Processing ───────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_separate_stems"),
		Description: "Separate an audio track into vocal and instrumental stems. Use suno_get_stem_separation to poll status",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track to separate", Required: true}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_get_stem_separation"),
		Description: "Get the status and URLs of a stem separation task. Returns vocal and instrumental track URLs when complete",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID from suno_separate_stems", Required: true}},
	},
	{
		Name:        mcp.ToolName("suno_convert_wav"),
		Description: "Convert a generated audio track to WAV format. Use suno_get_wav_conversion to poll status",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track to convert", Required: true}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_get_wav_conversion"),
		Description: "Get the status and download URL of a WAV conversion task",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID from suno_convert_wav", Required: true}},
	},

	// ── Advanced Generation ────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_cover_audio"),
		Description: "Create a cover version of uploaded audio with a new style and arrangement",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("upload_url"), Description: "URL of the source audio file", Required: true}, {Name: mcp.ParamName("style"), Description: "Target music style for the cover"}, {Name: mcp.ParamName("title"), Description: "Title for the cover version"}, {Name: mcp.ParamName("prompt"), Description: "Description or lyrics for the cover"}, {Name: mcp.ParamName("custom_mode"), Description: "Enable custom mode (true/false, default: true)"}, {Name: mcp.ParamName("model"), Description: "Model version (default: V4_5ALL)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_upload_extend"),
		Description: "Upload audio and extend it with AI-generated continuation",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("upload_url"), Description: "URL of the audio file to extend", Required: true}, {Name: mcp.ParamName("prompt"), Description: "Description or lyrics for the extension"}, {Name: mcp.ParamName("style"), Description: "Music style for the extension"}, {Name: mcp.ParamName("title"), Description: "Title for the extended track"}, {Name: mcp.ParamName("model"), Description: "Model version (default: V4_5ALL)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_add_vocals"),
		Description: "Generate vocal tracks for instrumental music",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the instrumental audio track", Required: true}, {Name: mcp.ParamName("prompt"), Description: "Lyrics or vocal description"}, {Name: mcp.ParamName("style"), Description: "Vocal style"}, {Name: mcp.ParamName("model"), Description: "Model version (default: V4_5ALL)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_add_instrumental"),
		Description: "Generate instrumental accompaniment for a vocal track",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the vocal audio track", Required: true}, {Name: mcp.ParamName("prompt"), Description: "Instrumental description"}, {Name: mcp.ParamName("style"), Description: "Instrumental style"}, {Name: mcp.ParamName("model"), Description: "Model version (default: V4_5ALL)"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_generate_mashup"),
		Description: "Generate a mashup combining elements from multiple tracks",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_ids"), Description: "Comma-separated list of audio IDs to mashup", Required: true}, {Name: mcp.ParamName("style"), Description: "Target style for the mashup"}, {Name: mcp.ParamName("prompt"), Description: "Description of the desired mashup"}, {Name: mcp.ParamName("model"), Description: "Model version (default: V4_5ALL)"}, {Name:

		// ── Persona ────────────────────────────────────────────────────
		mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},

	{
		Name:        mcp.ToolName("suno_generate_persona"),
		Description: "Create a personalized music persona based on generated tracks. Returns a persona_id for use in suno_generate_music",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_ids"), Description: "Comma-separated list of audio IDs to base the persona on", Required: true}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},

	// ── Video ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_generate_video"),
		Description: "Generate a music video from an audio track. Use suno_get_video to poll status",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track", Required: true}, {Name: mcp.ParamName("author"), Description: "Author/artist name for the video"}, {Name: mcp.ParamName("domain_name"), Description: "Brand domain name for the video"}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
	{
		Name:        mcp.ToolName("suno_get_video"),
		Description: "Get the status and URL of a video generation task",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID from suno_generate_video", Required: true}},
	},

	// ── MIDI ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("suno_generate_midi"),
		Description: "Generate a MIDI file from an audio track",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("audio_id"), Description: "ID of the audio track", Required: true}, {Name: mcp.ParamName("callback_url"), Description: "Webhook URL for completion notification"}},
	},
}
