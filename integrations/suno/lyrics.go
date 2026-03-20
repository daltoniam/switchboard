package suno

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func generateLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	prompt := r.Str("prompt")
	callbackURL := r.Str("callback_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"prompt": prompt,
	}

	if callbackURL != "" {
		body["callBackUrl"] = callbackURL
	}

	data, err := s.post(ctx, "/api/v1/lyrics", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	taskID := r.Str("task_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/lyrics/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAlignedLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	audioID := r.Str("audio_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"audioId": audioID,
	}

	data, err := s.post(ctx, "/api/v1/lyrics/aligned", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
