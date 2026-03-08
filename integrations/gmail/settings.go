package gmail

import (
	"context"
	"fmt"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// ── Vacation ────────────────────────────────────────────────────────

func getVacation(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/vacation", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateVacation(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if _, ok := args["enable_auto_reply"]; ok {
		body["enableAutoReply"] = argBool(args, "enable_auto_reply")
	}
	if v := argStr(args, "response_subject"); v != "" {
		body["responseSubject"] = v
	}
	if v := argStr(args, "response_body_plain_text"); v != "" {
		body["responseBodyPlainText"] = v
	}
	if v := argStr(args, "response_body_html"); v != "" {
		body["responseBodyHtml"] = v
	}
	if _, ok := args["restrict_to_contacts"]; ok {
		body["restrictToContacts"] = argBool(args, "restrict_to_contacts")
	}
	if _, ok := args["restrict_to_domain"]; ok {
		body["restrictToDomain"] = argBool(args, "restrict_to_domain")
	}
	if v := argStr(args, "start_time"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			body["startTime"] = n
		}
	}
	if v := argStr(args, "end_time"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			body["endTime"] = n
		}
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/vacation", user(args))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Auto-Forwarding ─────────────────────────────────────────────────

func getAutoForwarding(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/autoForwarding", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateAutoForwarding(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if _, ok := args["enabled"]; ok {
		body["enabled"] = argBool(args, "enabled")
	}
	if v := argStr(args, "email_address"); v != "" {
		body["emailAddress"] = v
	}
	if v := argStr(args, "disposition"); v != "" {
		body["disposition"] = v
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/autoForwarding", user(args))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── IMAP ────────────────────────────────────────────────────────────

func getImap(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/imap", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateImap(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if _, ok := args["enabled"]; ok {
		body["enabled"] = argBool(args, "enabled")
	}
	if _, ok := args["auto_expunge"]; ok {
		body["autoExpunge"] = argBool(args, "auto_expunge")
	}
	if v := argStr(args, "expunge_behavior"); v != "" {
		body["expungeBehavior"] = v
	}
	if v := argInt(args, "max_folder_size"); v != 0 {
		body["maxFolderSize"] = v
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/imap", user(args))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── POP ─────────────────────────────────────────────────────────────

func getPop(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/pop", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updatePop(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "access_window"); v != "" {
		body["accessWindow"] = v
	}
	if v := argStr(args, "disposition"); v != "" {
		body["disposition"] = v
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/pop", user(args))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Language ────────────────────────────────────────────────────────

func getLanguage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/language", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateLanguage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"displayLanguage": argStr(args, "display_language"),
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/language", user(args))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Filters ─────────────────────────────────────────────────────────

func listFilters(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/filters", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/filters/%s", user(args), argStr(args, "filter_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if criteria, err := parseJSON(args, "criteria"); err != nil {
		return errResult(err)
	} else if criteria != nil {
		body["criteria"] = criteria
	}
	if action, err := parseJSON(args, "action"); err != nil {
		return errResult(err)
	} else if action != nil {
		body["action"] = action
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/filters", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteFilter(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/filters/%s", user(args), argStr(args, "filter_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Forwarding Addresses ────────────────────────────────────────────

func listForwardingAddresses(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses/%s",
		user(args), argStr(args, "forwarding_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"forwardingEmail": argStr(args, "forwarding_email"),
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/forwardingAddresses", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteForwardingAddress(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/forwardingAddresses/%s",
		user(args), argStr(args, "forwarding_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Send As ─────────────────────────────────────────────────────────

func listSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/sendAs", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/sendAs/%s",
		user(args), argStr(args, "send_as_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"sendAsEmail": argStr(args, "send_as_email"),
	}
	if v := argStr(args, "display_name"); v != "" {
		body["displayName"] = v
	}
	if v := argStr(args, "reply_to_address"); v != "" {
		body["replyToAddress"] = v
	}
	if _, ok := args["is_default"]; ok {
		body["isDefault"] = argBool(args, "is_default")
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"sendAsEmail": argStr(args, "send_as_email"),
	}
	if v := argStr(args, "display_name"); v != "" {
		body["displayName"] = v
	}
	if v := argStr(args, "reply_to_address"); v != "" {
		body["replyToAddress"] = v
	}
	if _, ok := args["is_default"]; ok {
		body["isDefault"] = argBool(args, "is_default")
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs/%s", user(args), argStr(args, "send_as_email"))
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteSendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/sendAs/%s",
		user(args), argStr(args, "send_as_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func verifySendAs(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/sendAs/%s/verify",
		user(args), argStr(args, "send_as_email"))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Delegates ───────────────────────────────────────────────────────

func listDelegates(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/delegates", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/settings/delegates/%s",
		user(args), argStr(args, "delegate_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"delegateEmail": argStr(args, "delegate_email"),
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/settings/delegates", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteDelegate(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/settings/delegates/%s",
		user(args), argStr(args, "delegate_email"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
