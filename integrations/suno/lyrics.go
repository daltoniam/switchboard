package suno

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func generateLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"prompt": argStr(args, "prompt"),
	}

	if v := argStr(args, "callback_url"); v != "" {
		body["callBackUrl"] = v
	}

	data, err := s.post(ctx, "/api/v1/lyrics", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	taskID := argStr(args, "task_id")
	data, err := s.get(ctx, "/api/v1/lyrics/record-info?taskId=%s", taskID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAlignedLyrics(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"audioId": argStr(args, "audio_id"),
	}

	data, err := s.post(ctx, "/api/v1/lyrics/aligned", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
