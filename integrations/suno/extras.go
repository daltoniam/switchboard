package suno

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func separateStems(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
	}

	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/vocal-removal/generate", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getStemSeparation(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	taskID := argStr(args, "task_id")
	data, err := s.get(ctx, "/api/v1/vocal-removal/record-info?taskId=%s", taskID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func convertWav(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
	}

	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/convert/wav", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getWavConversion(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	taskID := argStr(args, "task_id")
	data, err := s.get(ctx, "/api/v1/convert/wav/record-info?taskId=%s", taskID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func generateVideo(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
	}

	if v := argStr(args, "author"); v != "" {
		body["author"] = v
	}
	if v := argStr(args, "domain_name"); v != "" {
		body["domainName"] = v
	}
	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/video/generate", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getVideo(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	taskID := argStr(args, "task_id")
	data, err := s.get(ctx, "/api/v1/video/record-info?taskId=%s", taskID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func generateMidi(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
	}

	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/midi/generate", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
