package slack

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func listUsers(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := []slack.GetUsersOption{slack.GetUsersOptionLimit(mcp.OptInt(args, "limit", 200))}
	if cursor != "" {
		opts = append(opts, slack.GetUsersOptionCursor(cursor))
	}
	users, err := client.GetUsersContext(ctx, opts...)
	if err != nil {
		return errResult(err)
	}
	type u struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		RealName    string `json:"real_name"`
		DisplayName string `json:"display_name,omitempty"`
		Email       string `json:"email,omitempty"`
		IsAdmin     bool   `json:"is_admin,omitempty"`
		IsBot       bool   `json:"is_bot,omitempty"`
		Deleted     bool   `json:"deleted,omitempty"`
		TZ          string `json:"timezone,omitempty"`
	}
	out := make([]u, 0, len(users))
	for _, user := range users {
		out = append(out, u{ID: user.ID, Name: user.Name, RealName: user.RealName, DisplayName: user.Profile.DisplayName, Email: user.Profile.Email, IsAdmin: user.IsAdmin, IsBot: user.IsBot, Deleted: user.Deleted, TZ: user.TZ})
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "users": out})
}

func getUserInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	user, err := client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id": user.ID, "name": user.Name, "real_name": user.RealName, "display_name": user.Profile.DisplayName,
		"email": user.Profile.Email, "title": user.Profile.Title, "status_text": user.Profile.StatusText,
		"status_emoji": user.Profile.StatusEmoji, "timezone": user.TZ, "is_admin": user.IsAdmin,
		"is_owner": user.IsOwner, "is_bot": user.IsBot, "deleted": user.Deleted,
	})
}

func getUserPresence(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	presence, err := client.GetUserPresenceContext(ctx, userID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"user_id": userID, "presence": presence.Presence, "online": presence.Online})
}

func listUserGroups(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	includeUsers := r.Bool("include_users")
	includeDisabled := r.Bool("include_disabled")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := []slack.GetUserGroupsOption{
		slack.GetUserGroupsOptionIncludeUsers(includeUsers),
		slack.GetUserGroupsOptionIncludeDisabled(includeDisabled),
		slack.GetUserGroupsOptionIncludeCount(true),
	}
	groups, err := client.GetUserGroupsContext(ctx, opts...)
	if err != nil {
		return errResult(err)
	}
	type ug struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Handle      string   `json:"handle"`
		Description string   `json:"description"`
		UserCount   int      `json:"user_count"`
		Users       []string `json:"users,omitempty"`
	}
	out := make([]ug, 0, len(groups))
	for _, g := range groups {
		out = append(out, ug{ID: g.ID, Name: g.Name, Handle: g.Handle, Description: g.Description, UserCount: g.UserCount, Users: g.Users})
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "user_groups": out})
}

func getUserGroup(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	usergroupID := r.Str("usergroup_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	members, err := client.GetUserGroupMembersContext(ctx, usergroupID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"usergroup_id": usergroupID, "count": len(members), "members": members})
}
