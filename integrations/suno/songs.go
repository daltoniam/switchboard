package suno

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func generateMusic(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"prompt":     argStr(args, "prompt"),
		"model":      "V4_5ALL",
		"customMode": true,
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "title"); v != "" {
		body["title"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}
	if v := argStr(args, "persona_id"); v != "" {
		body["personaId"] = v
	}
	if v := argStr(args, "negative_tags"); v != "" {
		body["negativeTags"] = v
	}
	if v := argStr(args, "vocal_gender"); v != "" {
		body["vocalGender"] = v
	}
	if args["custom_mode"] != nil {
		body["customMode"] = argBool(args, "custom_mode")
	}
	if args["instrumental"] != nil {
		body["instrumental"] = argBool(args, "instrumental")
	}
	if args["style_weight"] != nil {
		body["styleWeight"] = argFloat(args, "style_weight")
	}

	data, err := s.post(ctx, "/api/v1/generate", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getGeneration(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	taskID := argStr(args, "task_id")
	data, err := s.get(ctx, "/api/v1/generate/record-info?taskId=%s", taskID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func extendMusic(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId":          argStr(args, "audio_id"),
		"defaultParamFlag": argBool(args, "use_default_params"),
		"model":            "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "title"); v != "" {
		body["title"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}
	if args["continue_at"] != nil {
		body["continueAt"] = argInt(args, "continue_at")
	}

	data, err := s.post(ctx, "/api/v1/generate/extend", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getCredits(ctx context.Context, s *suno, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/generate/credit")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func coverAudio(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"uploadUrl":  argStr(args, "upload_url"),
		"customMode": true,
		"model":      "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "title"); v != "" {
		body["title"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}
	if args["custom_mode"] != nil {
		body["customMode"] = argBool(args, "custom_mode")
	}

	data, err := s.post(ctx, "/api/v1/generate/cover", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func uploadExtend(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"uploadUrl": argStr(args, "upload_url"),
		"model":     "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "title"); v != "" {
		body["title"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/upload-extend", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func addVocals(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
		"model":   "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/add-vocals", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func addInstrumental(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
		"model":   "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/add-instrumental", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func generateMashup(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	audioIDsStr := argStr(args, "audio_ids")
	if audioIDsStr == "" {
		return errResult(fmt.Errorf("audio_ids is required"))
	}
	audioIDs := strings.Split(audioIDsStr, ",")
	for i := range audioIDs {
		audioIDs[i] = strings.TrimSpace(audioIDs[i])
	}

	body := map[string]any{
		"audioIds": audioIDs,
		"model":    "V4_5ALL",
	}

	if v := argStr(args, "model"); v != "" {
		body["model"] = v
	}
	if v := argStr(args, "style"); v != "" {
		body["style"] = v
	}
	if v := argStr(args, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/mashup", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func generatePersona(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	audioIDsStr := argStr(args, "audio_ids")
	if audioIDsStr == "" {
		return errResult(fmt.Errorf("audio_ids is required"))
	}
	audioIDs := strings.Split(audioIDsStr, ",")
	for i := range audioIDs {
		audioIDs[i] = strings.TrimSpace(audioIDs[i])
	}

	body := map[string]any{
		"audioIds": audioIDs,
	}

	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/generate/persona", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
