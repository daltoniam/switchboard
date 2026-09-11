package front

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func resourcePath(id string) string {
	return strings.ReplaceAll(id, "/", "%2F")
}

func pageParamsFromNext(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return map[string]string{"page_token": raw}
	}
	out := map[string]string{}
	for k, vs := range u.Query() {
		if len(vs) > 0 && vs[0] != "" {
			out[k] = vs[0]
		}
	}
	return out
}

func clampLimit(limit int) int {
	if limit > 100 {
		return 100
	}
	if limit < 1 {
		return 25
	}
	return limit
}

func listQuery(args map[string]any, extra map[string]string) map[string]string {
	r := mcp.NewArgs(args)
	limit := r.OptInt("limit", 0)
	pageToken := r.Str("page_token")
	_ = r.Err()
	params := pageParamsFromNext(pageToken)
	if params == nil {
		params = map[string]string{}
	}
	if _, ok := args["limit"]; ok || params["limit"] == "" {
		if limit == 0 {
			limit = 25
		}
		params["limit"] = strconv.Itoa(clampLimit(limit))
	}
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func csvSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func searchConversations(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listQuery(args, nil)
	data, err := f.get(ctx, "/conversations/search/%s%s", url.PathEscape(query), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listConversations(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	statuses := r.Str("statuses")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listQuery(args, nil)
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	for _, status := range csvSlice(statuses) {
		vals.Add("q[statuses]", status)
	}
	qs := ""
	if encoded := vals.Encode(); encoded != "" {
		qs = "?" + encoded
	}
	data, err := f.get(ctx, "/conversations%s", qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getConversation(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/conversations/%s", resourcePath(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listConversationMessages(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listQuery(args, nil)
	data, err := f.get(ctx, "/conversations/%s/messages%s", resourcePath(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listConversationComments(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listQuery(args, nil)
	data, err := f.get(ctx, "/conversations/%s/comments%s", resourcePath(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addComment(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	body := r.Str("body")
	authorID := r.Str("author_id")
	pinned := r.Bool("is_pinned")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{"body": body}
	if authorID != "" {
		payload["author_id"] = authorID
	}
	if _, ok := args["is_pinned"]; ok {
		payload["is_pinned"] = pinned
	}
	data, err := f.post(ctx, "/conversations/"+resourcePath(id)+"/comments", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateConversation(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	status := r.Str("status")
	statusID := r.Str("status_id")
	assigneeID := r.Str("assignee_id")
	inboxID := r.Str("inbox_id")
	tagIDs := r.Str("tag_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, assigneeSet := args["assignee_id"]
	payload := map[string]any{}
	if status != "" {
		payload["status"] = status
	}
	if statusID != "" {
		payload["status_id"] = statusID
	}
	if assigneeSet {
		if assigneeID == "" {
			payload["assignee_id"] = nil
		} else {
			payload["assignee_id"] = assigneeID
		}
	}
	if inboxID != "" {
		payload["inbox_id"] = inboxID
	}
	if tags := csvSlice(tagIDs); tags != nil {
		payload["tag_ids"] = tags
	}
	if len(payload) == 0 {
		return mcp.ErrResult(fmt.Errorf("provide at least one of status, status_id, assignee_id, inbox_id, or tag_ids"))
	}
	data, err := f.patch(ctx, "/conversations/"+resourcePath(id), payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignConversation(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	assigneeID := r.Str("assignee_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{}
	if assigneeID == "" {
		payload["assignee_id"] = nil
	} else {
		payload["assignee_id"] = assigneeID
	}
	data, err := f.patch(ctx, "/conversations/"+resourcePath(id), payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listInboxes(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/inboxes")
}

func listTeammates(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/teammates")
}

func listTags(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/tags")
}

func listChannels(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/channels")
}

func listContacts(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/contacts")
}

func listAccounts(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	return listPath(ctx, f, args, "/accounts")
}

func listPath(ctx context.Context, f *front, args map[string]any, path string) (*mcp.ToolResult, error) {
	params := listQuery(args, nil)
	data, err := f.get(ctx, "%s%s", path, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchContacts(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	email := r.Str("email")
	contactID := r.Str("contact_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	id := contactID
	if id == "" && email != "" {
		id = "alt:email:" + email
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("provide email or contact_id"))
	}
	data, err := f.get(ctx, "/contacts/%s", resourcePath(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getContact(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("contact_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/contacts/%s", resourcePath(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAccount(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("account_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/accounts/%s", resourcePath(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func outboundPayload(r *mcp.Args, args map[string]any, defaultArchive bool) (map[string]any, error) {
	to := r.Str("to")
	cc := r.Str("cc")
	bcc := r.Str("bcc")
	subject := r.Str("subject")
	body := r.Str("body")
	authorID := r.Str("author_id")
	senderName := r.Str("sender_name")
	archive := r.Bool("archive")
	tagIDs := r.Str("tag_ids")
	if err := r.Err(); err != nil {
		return nil, err
	}
	payload := map[string]any{"body": body}
	if recips := csvSlice(to); recips != nil {
		payload["to"] = recips
	}
	if recips := csvSlice(cc); recips != nil {
		payload["cc"] = recips
	}
	if recips := csvSlice(bcc); recips != nil {
		payload["bcc"] = recips
	}
	if subject != "" {
		payload["subject"] = subject
	}
	if authorID != "" {
		payload["author_id"] = authorID
	}
	if senderName != "" {
		payload["sender_name"] = senderName
	}
	options := map[string]any{}
	if _, ok := args["archive"]; ok {
		options["archive"] = archive
	} else {
		options["archive"] = defaultArchive
	}
	if tags := csvSlice(tagIDs); tags != nil {
		options["tag_ids"] = tags
	}
	payload["options"] = options
	return payload, nil
}

func createMessage(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload, err := outboundPayload(r, args, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := payload["to"]; !ok {
		if _, ok := payload["cc"]; !ok {
			if _, ok := payload["bcc"]; !ok {
				return mcp.ErrResult(fmt.Errorf("provide to, cc, or bcc"))
			}
		}
	}
	data, err := f.post(ctx, "/channels/"+resourcePath(channelID)+"/messages", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func replyConversation(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload, err := outboundPayload(r, args, false)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if channelID != "" {
		payload["channel_id"] = channelID
	}
	data, err := f.post(ctx, "/conversations/"+resourcePath(id)+"/messages", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDraft(ctx context.Context, f *front, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	to := r.Str("to")
	cc := r.Str("cc")
	bcc := r.Str("bcc")
	subject := r.Str("subject")
	body := r.Str("body")
	authorID := r.Str("author_id")
	mode := r.Str("mode")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{"body": body}
	if recips := csvSlice(to); recips != nil {
		payload["to"] = recips
	}
	if recips := csvSlice(cc); recips != nil {
		payload["cc"] = recips
	}
	if recips := csvSlice(bcc); recips != nil {
		payload["bcc"] = recips
	}
	if subject != "" {
		payload["subject"] = subject
	}
	if authorID != "" {
		payload["author_id"] = authorID
	}
	if mode != "" {
		payload["mode"] = mode
	}
	data, err := f.post(ctx, "/channels/"+resourcePath(channelID)+"/drafts", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
