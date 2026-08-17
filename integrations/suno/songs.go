package suno

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func generateMusic(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	prompt := r.Str("prompt")
	model := r.Str("model")
	style := r.Str("style")
	title := r.Str("title")
	callbackURL := r.Str("callback_url")
	personaID := r.Str("persona_id")
	negativeTags := r.Str("negative_tags")
	vocalGender := r.Str("vocal_gender")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"prompt":     prompt,
		"model":      "V4_5ALL",
		"customMode": true,
	}

	if model != "" {
		body["model"] = model
	}
	if style != "" {
		body["style"] = style
	}
	if title != "" {
		body["title"] = title
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}
	if personaID != "" {
		body["personaId"] = personaID
	}
	if negativeTags != "" {
		body["negativeTags"] = negativeTags
	}
	if vocalGender != "" {
		body["vocalGender"] = vocalGender
	}
	if args["custom_mode"] != nil {
		v, err := mcp.ArgBool(args, "custom_mode")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["customMode"] = v
	}
	if args["instrumental"] != nil {
		v, err := mcp.ArgBool(args, "instrumental")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["instrumental"] = v
	}
	if args["style_weight"] != nil {
		v, err := mcp.ArgFloat64(args, "style_weight")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["styleWeight"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getGeneration(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/generate/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func extendMusic(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	useDefaultParams := r.Bool("use_default_params")
	model := r.Str("model")
	prompt := r.Str("prompt")
	style := r.Str("style")
	title := r.Str("title")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId":          audioID,
		"defaultParamFlag": useDefaultParams,
		"model":            "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if style != "" {
		body["style"] = style
	}
	if title != "" {
		body["title"] = title
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}
	if args["continue_at"] != nil {
		v, err := mcp.ArgInt(args, "continue_at")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["continueAt"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/extend", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCredits(ctx context.Context, s *suno, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/generate/credit")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func coverAudio(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	uploadURL := r.Str("upload_url")
	model := r.Str("model")
	style := r.Str("style")
	title := r.Str("title")
	prompt := r.Str("prompt")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"uploadUrl":  uploadURL,
		"customMode": true,
		"model":      "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if style != "" {
		body["style"] = style
	}
	if title != "" {
		body["title"] = title
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}
	if args["custom_mode"] != nil {
		v, err := mcp.ArgBool(args, "custom_mode")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["customMode"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/cover", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func uploadExtend(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	uploadURL := r.Str("upload_url")
	model := r.Str("model")
	prompt := r.Str("prompt")
	style := r.Str("style")
	title := r.Str("title")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"uploadUrl": uploadURL,
		"model":     "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if style != "" {
		body["style"] = style
	}
	if title != "" {
		body["title"] = title
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/generate/upload-extend", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addVocals(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	model := r.Str("model")
	prompt := r.Str("prompt")
	style := r.Str("style")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
		"model":   "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if style != "" {
		body["style"] = style
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/generate/add-vocals", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addInstrumental(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	model := r.Str("model")
	prompt := r.Str("prompt")
	style := r.Str("style")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
		"model":   "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if style != "" {
		body["style"] = style
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/generate/add-instrumental", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateMashup(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioIDsStr := r.Str("audio_ids")
	model := r.Str("model")
	style := r.Str("style")
	prompt := r.Str("prompt")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if audioIDsStr == "" {
		return mcp.ErrResult(fmt.Errorf("audio_ids is required"))
	}
	audioIDs := strings.Split(audioIDsStr, ",")
	for i := range audioIDs {
		audioIDs[i] = strings.TrimSpace(audioIDs[i])
	}

	body := map[string]any{
		"audioIds": audioIDs,
		"model":    "V4_5ALL",
	}

	if model != "" {
		body["model"] = model
	}
	if style != "" {
		body["style"] = style
	}
	if prompt != "" {
		body["prompt"] = prompt
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/generate/mashup", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generatePersona(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioIDsStr := r.Str("audio_ids")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if audioIDsStr == "" {
		return mcp.ErrResult(fmt.Errorf("audio_ids is required"))
	}
	audioIDs := strings.Split(audioIDsStr, ",")
	for i := range audioIDs {
		audioIDs[i] = strings.TrimSpace(audioIDs[i])
	}

	body := map[string]any{
		"audioIds": audioIDs,
	}

	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/generate/persona", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
