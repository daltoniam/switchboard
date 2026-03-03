package slack

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func authTest(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	resp, err := s.getClient().AuthTestContext(ctx)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"user":    resp.User,
		"user_id": resp.UserID,
		"team":    resp.Team,
		"team_id": resp.TeamID,
		"url":     resp.URL,
	})
}

func teamInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	info, err := s.getClient().GetTeamInfoContext(ctx)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":     info.ID,
		"name":   info.Name,
		"domain": info.Domain,
		"icon":   info.Icon,
	})
}

func uploadFile(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	params := slack.UploadFileParameters{
		Content:         argStr(args, "content"),
		Filename:        argStr(args, "filename"),
		Title:           argStr(args, "title"),
		InitialComment:  argStr(args, "initial_comment"),
		SnippetType:     argStr(args, "filetype"),
		ThreadTimestamp: argStr(args, "thread_ts"),
	}
	channels := argStr(args, "channels")
	if channels != "" {
		params.Channel = strings.Split(channels, ",")[0]
	}

	file, err := s.getClient().UploadFileContext(ctx, params)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":    file.ID,
		"title": file.Title,
	})
}

func listFiles(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	params := slack.ListFilesParameters{
		Channel: argStr(args, "channel_id"),
		User:    argStr(args, "user_id"),
		Types:   argStr(args, "types"),
		Limit:   optInt(args, "count", 20),
	}

	files, _, err := s.getClient().ListFilesContext(ctx, params)
	if err != nil {
		return errResult(err), nil
	}

	type f struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Title    string `json:"title"`
		Filetype string `json:"filetype"`
		Size     int    `json:"size"`
		User     string `json:"user"`
	}
	out := make([]f, 0, len(files))
	for _, file := range files {
		out = append(out, f{
			ID:       file.ID,
			Name:     file.Name,
			Title:    file.Title,
			Filetype: file.Filetype,
			Size:     file.Size,
			User:     file.User,
		})
	}
	return jsonResult(map[string]any{"count": len(out), "files": out})
}

func deleteFile(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	err := s.getClient().DeleteFileContext(ctx, argStr(args, "file_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "deleted", "file_id": argStr(args, "file_id")})
}

func listEmoji(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	emoji, err := s.getClient().GetEmojiContext(ctx)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"count": len(emoji), "emoji": emoji})
}

func setStatus(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	expiration := int64(argInt(args, "status_expiration"))
	err := s.getClient().SetUserCustomStatusContext(ctx, argStr(args, "status_text"), argStr(args, "status_emoji"), expiration)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"status":      "set",
		"status_text": argStr(args, "status_text"),
	})
}

func listBookmarks(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	bookmarks, err := s.getClient().ListBookmarksContext(ctx, argStr(args, "channel_id"))
	if err != nil {
		return errResult(err), nil
	}

	type bm struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Link  string `json:"link"`
		Emoji string `json:"emoji,omitempty"`
		Type  string `json:"type"`
	}
	out := make([]bm, 0, len(bookmarks))
	for _, b := range bookmarks {
		out = append(out, bm{
			ID:    b.ID,
			Title: b.Title,
			Link:  b.Link,
			Emoji: b.Emoji,
			Type:  b.Type,
		})
	}
	return jsonResult(map[string]any{"count": len(out), "bookmarks": out})
}

func addBookmark(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	params := slack.AddBookmarkParameters{
		Title: argStr(args, "title"),
		Type:  "link",
		Link:  argStr(args, "link"),
		Emoji: argStr(args, "emoji"),
	}
	bm, err := s.getClient().AddBookmarkContext(ctx, argStr(args, "channel_id"), params)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":    bm.ID,
		"title": bm.Title,
		"link":  bm.Link,
	})
}

func removeBookmark(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	err := s.getClient().RemoveBookmarkContext(ctx, argStr(args, "channel_id"), argStr(args, "bookmark_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "removed", "bookmark_id": argStr(args, "bookmark_id")})
}

func addReminder(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	user := argStr(args, "user")
	if user == "" {
		user = "me"
	}
	reminder, err := s.getClient().AddUserReminder(user, argStr(args, "text"), argStr(args, "time"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":   reminder.ID,
		"text": reminder.Text,
		"user": reminder.User,
		"time": reminder.Time,
	})
}

func listReminders(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	reminders, err := s.getClient().ListReminders()
	if err != nil {
		return errResult(err), nil
	}
	type rem struct {
		ID   string `json:"id"`
		Text string `json:"text"`
		User string `json:"user"`
		Time int    `json:"time"`
	}
	out := make([]rem, 0, len(reminders))
	for _, r := range reminders {
		out = append(out, rem{
			ID:   r.ID,
			Text: r.Text,
			User: r.User,
			Time: r.Time,
		})
	}
	return jsonResult(map[string]any{"count": len(out), "reminders": out})
}

func deleteReminder(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	err := s.getClient().DeleteReminder(argStr(args, "reminder_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "deleted", "reminder_id": argStr(args, "reminder_id")})
}
