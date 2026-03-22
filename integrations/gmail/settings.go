package gmail

import (
	"context"
	"fmt"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// ── Vacation ────────────────────────────────────────────────────────

func getVacation(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/vacation", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateVacation(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if _, ok := args["enable_auto_reply"]; ok {
		body["enableAutoReply"] = r.Bool("enable_auto_reply")
	}
	if v := r.Str("response_subject"); v != "" {
		body["responseSubject"] = v
	}
	if v := r.Str("response_body_plain_text"); v != "" {
		body["responseBodyPlainText"] = v
	}
	if v := r.Str("response_body_html"); v != "" {
		body["responseBodyHtml"] = v
	}
	if _, ok := args["restrict_to_contacts"]; ok {
		body["restrictToContacts"] = r.Bool("restrict_to_contacts")
	}
	if _, ok := args["restrict_to_domain"]; ok {
		body["restrictToDomain"] = r.Bool("restrict_to_domain")
	}
	if v := r.Str("start_time"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			body["startTime"] = n
		}
	}
	if v := r.Str("end_time"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			body["endTime"] = n
		}
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/vacation", u)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Auto-Forwarding ─────────────────────────────────────────────────

func getAutoForwarding(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/autoForwarding", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAutoForwarding(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if _, ok := args["enabled"]; ok {
		body["enabled"] = r.Bool("enabled")
	}
	if v := r.Str("email_address"); v != "" {
		body["emailAddress"] = v
	}
	if v := r.Str("disposition"); v != "" {
		body["disposition"] = v
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/autoForwarding", u)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── IMAP ────────────────────────────────────────────────────────────

func getImap(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/imap", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateImap(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if _, ok := args["enabled"]; ok {
		body["enabled"] = r.Bool("enabled")
	}
	if _, ok := args["auto_expunge"]; ok {
		body["autoExpunge"] = r.Bool("auto_expunge")
	}
	if v := r.Str("expunge_behavior"); v != "" {
		body["expungeBehavior"] = v
	}
	if v := r.Int("max_folder_size"); v != 0 {
		body["maxFolderSize"] = v
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/imap", u)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── POP ─────────────────────────────────────────────────────────────

func getPop(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/pop", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePop(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if v := r.Str("access_window"); v != "" {
		body["accessWindow"] = v
	}
	if v := r.Str("disposition"); v != "" {
		body["disposition"] = v
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/pop", u)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Language ────────────────────────────────────────────────────────

func getLanguage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/language", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateLanguage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"displayLanguage": r.Str("display_language"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/language", u)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Filters ─────────────────────────────────────────────────────────

func listFilters(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/filters", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	filterID := r.Str("filter_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/filters/%s", u, filterID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if criteria, err := parseJSON(args, "criteria"); err != nil {
		return mcp.ErrResult(err)
	} else if criteria != nil {
		body["criteria"] = criteria
	}
	if action, err := parseJSON(args, "action"); err != nil {
		return mcp.ErrResult(err)
	} else if action != nil {
		body["action"] = action
	}
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/filters", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	filterID := r.Str("filter_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/filters/%s", u, filterID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Forwarding Addresses ────────────────────────────────────────────

func listForwardingAddresses(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	fwdEmail := r.Str("forwarding_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses/%s",
		u, fwdEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"forwardingEmail": r.Str("forwarding_email"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/forwardingAddresses", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	fwdEmail := r.Str("forwarding_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses/%s",
		u, fwdEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Send As ─────────────────────────────────────────────────────────

func listSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/sendAs", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	sendAsEmail := r.Str("send_as_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/sendAs/%s",
		u, sendAsEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"sendAsEmail": r.Str("send_as_email"),
	}
	if v := r.Str("display_name"); v != "" {
		body["displayName"] = v
	}
	if v := r.Str("reply_to_address"); v != "" {
		body["replyToAddress"] = v
	}
	if _, ok := args["is_default"]; ok {
		body["isDefault"] = r.Bool("is_default")
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"sendAsEmail": r.Str("send_as_email"),
	}
	if v := r.Str("display_name"); v != "" {
		body["displayName"] = v
	}
	if v := r.Str("reply_to_address"); v != "" {
		body["replyToAddress"] = v
	}
	if _, ok := args["is_default"]; ok {
		body["isDefault"] = r.Bool("is_default")
	}
	u := user(r)
	sendAsEmail := r.Str("send_as_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs/%s", u, sendAsEmail)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	sendAsEmail := r.Str("send_as_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/sendAs/%s",
		u, sendAsEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func verifySendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	sendAsEmail := r.Str("send_as_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs/%s/verify",
		u, sendAsEmail)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Delegates ───────────────────────────────────────────────────────

func listDelegates(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/delegates", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	delegateEmail := r.Str("delegate_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/delegates/%s",
		u, delegateEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"delegateEmail": r.Str("delegate_email"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/delegates", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	delegateEmail := r.Str("delegate_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/delegates/%s",
		u, delegateEmail)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
