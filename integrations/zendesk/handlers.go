package zendesk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func searchTickets(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	sortBy := r.Str("sort_by")
	sortOrder := r.Str("sort_order")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 25)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if !strings.Contains(query, "type:") {
		query = "type:ticket " + query
	}
	params := map[string]string{
		"query":      query,
		"sort_by":    sortBy,
		"sort_order": sortOrder,
		"page":       strconv.Itoa(page),
		"per_page":   strconv.Itoa(perPage),
	}
	data, err := z.get(ctx, "/search.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTickets(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sortBy := r.Str("sort_by")
	sortOrder := r.Str("sort_order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if sortBy != "" {
		params["sort_by"] = sortBy
	}
	if sortOrder != "" {
		params["sort_order"] = sortOrder
	}
	data, err := z.get(ctx, "/tickets.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTicket(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	include := r.Str("include")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if include != "" {
		params["include"] = include
	}
	data, err := z.get(ctx, "/tickets/%s.json%s", url.PathEscape(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTicket(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	ticket, err := ticketPayload(args, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.post(ctx, "/tickets.json", map[string]any{"ticket": ticket})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTicket(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ticket, err := ticketPayload(args, false)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.put(ctx, "/tickets/"+url.PathEscape(id)+".json", map[string]any{"ticket": ticket})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTicket(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.del(ctx, "/tickets/%s.json", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTicketComments(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	sortOrder := r.Str("sort_order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if sortOrder != "" {
		params["sort_order"] = sortOrder
	}
	data, err := z.get(ctx, "/tickets/%s/comments.json%s", url.PathEscape(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addTicketComment(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	body := r.Str("body")
	public := true
	if v := r.Str("public"); v != "" {
		public = v != "false"
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{
		"ticket": map[string]any{
			"comment": map[string]any{"body": body, "public": public},
		},
	}
	data, err := z.put(ctx, "/tickets/"+url.PathEscape(id)+".json", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTicketAudits(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.get(ctx, "/tickets/%s/audits.json%s", url.PathEscape(id), queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func search(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	sortBy := r.Str("sort_by")
	sortOrder := r.Str("sort_order")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 25)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":      query,
		"sort_by":    sortBy,
		"sort_order": sortOrder,
		"page":       strconv.Itoa(page),
		"per_page":   strconv.Itoa(perPage),
	}
	data, err := z.get(ctx, "/search.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUsers(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if role != "" {
		params["role"] = role
	}
	data, err := z.get(ctx, "/users.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUser(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.get(ctx, "/users/%s.json", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCurrentUser(ctx context.Context, z *zendesk, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := z.get(ctx, "/users/me.json")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createUser(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	user, err := userPayload(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.post(ctx, "/users.json", map[string]any{"user": user})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateUser(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	user, err := userPayload(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.put(ctx, "/users/"+url.PathEscape(id)+".json", map[string]any{"user": user})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrganizations(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	data, err := z.get(ctx, "/organizations.json%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getOrganization(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("organization_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.get(ctx, "/organizations/%s.json", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listGroups(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	data, err := z.get(ctx, "/groups.json%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getGroup(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.get(ctx, "/groups/%s.json", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listViews(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	active := r.Str("active")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if active != "" {
		params["active"] = active
	}
	data, err := z.get(ctx, "/views.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listViewTickets(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("view_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := z.get(ctx, "/views/%s/tickets.json%s", url.PathEscape(id), queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMacros(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	active := r.Str("active")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if active != "" {
		params["active"] = active
	}
	data, err := z.get(ctx, "/macros.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchArticles(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	locale := r.Str("locale")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 25)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":    query,
		"locale":   locale,
		"page":     strconv.Itoa(page),
		"per_page": strconv.Itoa(perPage),
	}
	data, err := z.get(ctx, "/help_center/articles/search.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getArticle(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("article_id")
	locale := r.Str("locale")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := "/help_center/articles/%s.json"
	if locale != "" {
		path = "/help_center/" + url.PathEscape(locale) + "/articles/%s.json"
	}
	data, err := z.get(ctx, path, url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTags(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	data, err := z.get(ctx, "/tags.json%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSatisfactionRatings(ctx context.Context, z *zendesk, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	score := r.Str("score")
	startTime := r.Str("start_time")
	endTime := r.Str("end_time")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	if score != "" {
		params["score"] = score
	}
	if startTime != "" {
		params["start_time"] = startTime
	}
	if endTime != "" {
		params["end_time"] = endTime
	}
	data, err := z.get(ctx, "/satisfaction_ratings.json%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ticketPayload(args map[string]any, requireComment bool) (map[string]any, error) {
	r := mcp.NewArgs(args)
	subject := r.Str("subject")
	comment := r.Str("comment")
	status := r.Str("status")
	priority := r.Str("priority")
	typ := r.Str("type")
	requesterID := r.Str("requester_id")
	requesterEmail := r.Str("requester_email")
	requesterName := r.Str("requester_name")
	assigneeID := r.Str("assignee_id")
	groupID := r.Str("group_id")
	orgID := r.Str("organization_id")
	tags := r.Str("tags")
	customFieldsRaw := r.Str("custom_fields")
	publicStr := r.Str("public")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if requireComment && comment == "" {
		return nil, fmt.Errorf("comment is required")
	}

	ticket := map[string]any{}
	if subject != "" {
		ticket["subject"] = subject
	}
	if comment != "" {
		public := true
		if publicStr != "" {
			public = publicStr != "false"
		}
		ticket["comment"] = map[string]any{"body": comment, "public": public}
	}
	if status != "" {
		ticket["status"] = status
	}
	if priority != "" {
		ticket["priority"] = priority
	}
	if typ != "" {
		ticket["type"] = typ
	}
	if requesterID != "" {
		ticket["requester_id"] = requesterID
	} else if requesterEmail != "" || requesterName != "" {
		req := map[string]any{}
		if requesterEmail != "" {
			req["email"] = requesterEmail
		}
		if requesterName != "" {
			req["name"] = requesterName
		}
		ticket["requester"] = req
	}
	if _, ok := args["assignee_id"]; ok {
		if assigneeID == "" {
			ticket["assignee_id"] = nil
		} else {
			ticket["assignee_id"] = assigneeID
		}
	}
	if groupID != "" {
		ticket["group_id"] = groupID
	}
	if orgID != "" {
		ticket["organization_id"] = orgID
	}
	if tags != "" {
		ticket["tags"] = splitCSV(tags)
	}
	if customFieldsRaw != "" {
		var fields any
		if err := json.Unmarshal([]byte(customFieldsRaw), &fields); err != nil {
			return nil, fmt.Errorf("invalid custom_fields JSON: %w", err)
		}
		ticket["custom_fields"] = fields
	}
	return ticket, nil
}

func userPayload(args map[string]any) (map[string]any, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	email := r.Str("email")
	role := r.Str("role")
	phone := r.Str("phone")
	orgID := r.Str("organization_id")
	tags := r.Str("tags")
	verified := r.Str("verified")
	if err := r.Err(); err != nil {
		return nil, err
	}
	user := map[string]any{}
	if name != "" {
		user["name"] = name
	}
	if email != "" {
		user["email"] = email
	}
	if role != "" {
		user["role"] = role
	}
	if phone != "" {
		user["phone"] = phone
	}
	if orgID != "" {
		user["organization_id"] = orgID
	}
	if tags != "" {
		user["tags"] = splitCSV(tags)
	}
	if verified != "" {
		user["verified"] = verified == "true"
	}
	return user, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
