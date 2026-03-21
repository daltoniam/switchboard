package slack

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func authTest(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"user": resp.User, "user_id": resp.UserID, "team": resp.Team, "team_id": resp.TeamID, "url": resp.URL})
}

func teamInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	info, err := client.GetTeamInfoContext(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": info.ID, "name": info.Name, "domain": info.Domain, "icon": info.Icon})
}

func uploadFile(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	content := r.Str("content")
	filename := r.Str("filename")
	title := r.Str("title")
	initialComment := r.Str("initial_comment")
	filetype := r.Str("filetype")
	threadTS := r.Str("thread_ts")
	channels := r.Str("channels")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := slack.UploadFileParameters{
		Content: content, Filename: filename, Title: title,
		InitialComment: initialComment, SnippetType: filetype, ThreadTimestamp: threadTS,
	}
	if channels != "" {
		params.Channel = strings.Split(channels, ",")[0]
	}
	file, err := client.UploadFileContext(ctx, params)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": file.ID, "title": file.Title})
}

func listFiles(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	userID := r.Str("user_id")
	types := r.Str("types")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := slack.ListFilesParameters{Channel: channelID, User: userID, Types: types, Limit: mcp.OptInt(args, "count", 20)}
	files, _, err := client.ListFilesContext(ctx, params)
	if err != nil {
		return errResult(err)
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
		out = append(out, f{ID: file.ID, Name: file.Name, Title: file.Title, Filetype: file.Filetype, Size: file.Size, User: file.User})
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "files": out})
}

func deleteFile(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	fileID := r.Str("file_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.DeleteFileContext(ctx, fileID); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "deleted", "file_id": fileID})
}

func listEmoji(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	emoji, err := client.GetEmojiContext(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"count": len(emoji), "emoji": emoji})
}

func setStatus(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	statusText := r.Str("status_text")
	statusEmoji := r.Str("status_emoji")
	statusExpiration := r.Int("status_expiration")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.SetUserCustomStatusContext(ctx, statusText, statusEmoji, int64(statusExpiration)); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "set", "status_text": statusText})
}

func listBookmarks(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	bookmarks, err := client.ListBookmarksContext(ctx, channelID)
	if err != nil {
		return errResult(err)
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
		out = append(out, bm{ID: b.ID, Title: b.Title, Link: b.Link, Emoji: b.Emoji, Type: b.Type})
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "bookmarks": out})
}

func addBookmark(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	title := r.Str("title")
	link := r.Str("link")
	emoji := r.Str("emoji")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	bm, err := client.AddBookmarkContext(ctx, channelID, slack.AddBookmarkParameters{Title: title, Type: "link", Link: link, Emoji: emoji})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": bm.ID, "title": bm.Title, "link": bm.Link})
}

func removeBookmark(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	bookmarkID := r.Str("bookmark_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.RemoveBookmarkContext(ctx, channelID, bookmarkID); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "removed", "bookmark_id": bookmarkID})
}

func addReminder(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	user := r.Str("user")
	text := r.Str("text")
	timeStr := r.Str("time")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if user == "" {
		user = "me"
	}
	reminder, err := client.AddUserReminder(user, text, timeStr)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": reminder.ID, "text": reminder.Text, "user": reminder.User, "time": reminder.Time})
}

func listReminders(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	reminders, err := client.ListReminders()
	if err != nil {
		return errResult(err)
	}
	type rem struct {
		ID   string `json:"id"`
		Text string `json:"text"`
		User string `json:"user"`
		Time int    `json:"time"`
	}
	out := make([]rem, 0, len(reminders))
	for _, r := range reminders {
		out = append(out, rem{ID: r.ID, Text: r.Text, User: r.User, Time: r.Time})
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "reminders": out})
}

func deleteReminder(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	reminderID := r.Str("reminder_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.DeleteReminder(reminderID); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "deleted", "reminder_id": reminderID})
}
