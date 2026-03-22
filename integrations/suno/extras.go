package suno

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func separateStems(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
	}

	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/vocal-removal/generate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getStemSeparation(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/vocal-removal/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func convertWav(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
	}

	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/convert/wav", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getWavConversion(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/convert/wav/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateVideo(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	author := r.Str("author")
	domainName := r.Str("domain_name")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
	}

	if author != "" {
		body["author"] = author
	}
	if domainName != "" {
		body["domainName"] = domainName
	}
	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/video/generate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVideo(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/video/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateMidi(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
	}

	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/midi/generate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
