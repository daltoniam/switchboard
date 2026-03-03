package slack

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func listUsers(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	opts := []slack.GetUsersOption{
		slack.GetUsersOptionLimit(optInt(args, "limit", 200)),
	}
	if c := argStr(args, "cursor"); c != "" {
		opts = append(opts, slack.GetUsersOptionCursor(c))
	}

	users, err := s.getClient().GetUsersContext(ctx, opts...)
	if err != nil {
		return errResult(err), nil
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
		out = append(out, u{
			ID:          user.ID,
			Name:        user.Name,
			RealName:    user.RealName,
			DisplayName: user.Profile.DisplayName,
			Email:       user.Profile.Email,
			IsAdmin:     user.IsAdmin,
			IsBot:       user.IsBot,
			Deleted:     user.Deleted,
			TZ:          user.TZ,
		})
	}
	return jsonResult(map[string]any{"count": len(out), "users": out})
}

func getUserInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	user, err := s.getClient().GetUserInfoContext(ctx, argStr(args, "user_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":           user.ID,
		"name":         user.Name,
		"real_name":    user.RealName,
		"display_name": user.Profile.DisplayName,
		"email":        user.Profile.Email,
		"title":        user.Profile.Title,
		"status_text":  user.Profile.StatusText,
		"status_emoji": user.Profile.StatusEmoji,
		"timezone":     user.TZ,
		"is_admin":     user.IsAdmin,
		"is_owner":     user.IsOwner,
		"is_bot":       user.IsBot,
		"deleted":      user.Deleted,
	})
}

func getUserPresence(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	presence, err := s.getClient().GetUserPresenceContext(ctx, argStr(args, "user_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"user_id":  argStr(args, "user_id"),
		"presence": presence.Presence,
		"online":   presence.Online,
	})
}

func listUserGroups(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	opts := []slack.GetUserGroupsOption{
		slack.GetUserGroupsOptionIncludeUsers(argBool(args, "include_users")),
		slack.GetUserGroupsOptionIncludeDisabled(argBool(args, "include_disabled")),
		slack.GetUserGroupsOptionIncludeCount(true),
	}

	groups, err := s.getClient().GetUserGroupsContext(ctx, opts...)
	if err != nil {
		return errResult(err), nil
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
		out = append(out, ug{
			ID:          g.ID,
			Name:        g.Name,
			Handle:      g.Handle,
			Description: g.Description,
			UserCount:   g.UserCount,
			Users:       g.Users,
		})
	}
	return jsonResult(map[string]any{"count": len(out), "user_groups": out})
}

func getUserGroup(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	members, err := s.getClient().GetUserGroupMembersContext(ctx, argStr(args, "usergroup_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"usergroup_id": argStr(args, "usergroup_id"),
		"count":        len(members),
		"members":      members,
	})
}
