package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func listUsers(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	filter := r.Str("filter")
	search := r.Str("search")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/users%s", queryEncode(map[string]string{
		"q": q, "filter": filter, "search": search, "after": after, "limit": limit,
	}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireID("user_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.get(ctx, "/users/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	profileRaw := r.Str("profile")
	activate := r.Str("activate")
	groupIDsRaw := r.Str("group_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireID("profile", profileRaw); err != nil {
		return mcp.ErrResult(err)
	}
	profile, err := parseJSONObject(profileRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"profile": profile}
	if groupIDsRaw != "" {
		var ids []any
		if err := json.Unmarshal([]byte(groupIDsRaw), &ids); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON array for group_ids: %w", err))
		}
		body["groupIds"] = ids
	}
	if activate == "" {
		activate = "true"
	}
	data, err := o.post(ctx, "/users"+queryEncode(map[string]string{"activate": activate}), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	profileRaw := r.Str("profile")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	profile, err := parseJSONObject(profileRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.post(ctx, "/users/"+url.PathEscape(id), map[string]any{"profile": profile})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func activateUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	return userLifecycle(ctx, o, args, "activate", true)
}

func deactivateUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	return userLifecycle(ctx, o, args, "deactivate", false)
}

func suspendUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	return userLifecycle(ctx, o, args, "suspend", false)
}

func unsuspendUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	return userLifecycle(ctx, o, args, "unsuspend", false)
}

func unlockUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	return userLifecycle(ctx, o, args, "unlock", false)
}

func userLifecycle(ctx context.Context, o *okta, args map[string]any, action string, defaultSendEmail bool) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	sendEmail := r.Str("send_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireID("user_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if sendEmail != "" {
		params["sendEmail"] = sendEmail
	} else if action == "activate" || action == "deactivate" {
		params["sendEmail"] = strconv.FormatBool(defaultSendEmail)
	}
	data, err := o.post(ctx, fmt.Sprintf("/users/%s/lifecycle/%s%s", url.PathEscape(id), action, queryEncode(params)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func resetPassword(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	sendEmail := r.Str("send_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sendEmail == "" {
		sendEmail = "true"
	}
	data, err := o.post(ctx, fmt.Sprintf("/users/%s/lifecycle/reset_password%s", url.PathEscape(id), queryEncode(map[string]string{"sendEmail": sendEmail})), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func expirePassword(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	temp := r.Str("temp_password")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	action := "expire_password"
	if temp == "true" {
		action = "expire_password_with_temp_password"
	}
	data, err := o.post(ctx, fmt.Sprintf("/users/%s/lifecycle/%s", url.PathEscape(id), action), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUserGroups(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/users/%s/groups%s", url.PathEscape(id), queryEncode(map[string]string{"after": after, "limit": limit}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUserFactors(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/users/%s/factors", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listGroups(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	filter := r.Str("filter")
	search := r.Str("search")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/groups%s", queryEncode(map[string]string{
		"q": q, "filter": filter, "search": search, "after": after, "limit": limit,
	}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.get(ctx, "/groups/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"profile": map[string]any{"name": name, "description": description}}
	data, err := o.post(ctx, "/groups", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("group_id")
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"profile": map[string]any{"name": name, "description": description}}
	data, err := o.put(ctx, "/groups/"+url.PathEscape(id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.del(ctx, "/groups/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listGroupMembers(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("group_id")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/groups/%s/users%s", url.PathEscape(id), queryEncode(map[string]string{"after": after, "limit": limit}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addGroupMember(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	groupID := r.Str("group_id")
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.put(ctx, fmt.Sprintf("/groups/%s/users/%s", url.PathEscape(groupID), url.PathEscape(userID)), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func removeGroupMember(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	groupID := r.Str("group_id")
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.del(ctx, "/groups/%s/users/%s", url.PathEscape(groupID), url.PathEscape(userID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listApps(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	filter := r.Str("filter")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/apps%s", queryEncode(map[string]string{
		"q": q, "filter": filter, "after": after, "limit": limit,
	}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getApp(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.get(ctx, "/apps/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAppUsers(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("app_id")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/apps/%s/users%s", url.PathEscape(id), queryEncode(map[string]string{"after": after, "limit": limit}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignAppUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	userID := r.Str("user_id")
	profileRaw := r.Str("profile")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"id": userID, "scope": "USER"}
	if profileRaw != "" {
		profile, err := parseJSONObject(profileRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["profile"] = profile
	}
	data, err := o.post(ctx, "/apps/"+url.PathEscape(appID)+"/users", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unassignAppUser(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.del(ctx, "/apps/%s/users/%s", url.PathEscape(appID), url.PathEscape(userID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAppGroups(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("app_id")
	after := r.Str("after")
	limit := limitParam(r, 20, 200)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/apps/%s/groups%s", url.PathEscape(id), queryEncode(map[string]string{"after": after, "limit": limit}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignAppGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	groupID := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.put(ctx, fmt.Sprintf("/apps/%s/groups/%s", url.PathEscape(appID), url.PathEscape(groupID)), map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unassignAppGroup(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	groupID := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.del(ctx, "/apps/%s/groups/%s", url.PathEscape(appID), url.PathEscape(groupID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPolicies(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	policyType := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/policies%s", queryEncode(map[string]string{"type": policyType}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPolicy(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.get(ctx, "/policies/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPolicyRules(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("policy_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/policies/%s/rules", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLogs(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	since := r.Str("since")
	until := r.Str("until")
	filter := r.Str("filter")
	q := r.Str("q")
	after := r.Str("after")
	limit := limitParam(r, 20, 1000)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := o.getList(ctx, "/logs%s", queryEncode(map[string]string{
		"since": since, "until": until, "filter": filter, "q": q, "after": after, "limit": limit,
	}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getOrg(ctx context.Context, o *okta, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := o.get(ctx, "/org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
