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
	if !hasTypeFilter(query) {
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
	if _, ok := args["public"]; ok {
		public = r.Bool("public")
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
	requesterEmail := r.Str("requester_email")
	requesterName := r.Str("requester_name")
	tags := r.StrSlice("tags")
	public := true
	if _, ok := args["public"]; ok {
		public = r.Bool("public")
	}
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
	if id, ok, err := optionalIntID(args, "requester_id"); err != nil {
		return nil, err
	} else if ok {
		ticket["requester_id"] = id
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
	if _, present := args["assignee_id"]; present {
		id, ok, err := optionalIntID(args, "assignee_id")
		if err != nil {
			return nil, err
		}
		if ok {
			ticket["assignee_id"] = id
		} else {
			ticket["assignee_id"] = nil
		}
	}
	if id, ok, err := optionalIntID(args, "group_id"); err != nil {
		return nil, err
	} else if ok {
		ticket["group_id"] = id
	}
	if id, ok, err := optionalIntID(args, "organization_id"); err != nil {
		return nil, err
	} else if ok {
		ticket["organization_id"] = id
	}
	if len(tags) > 0 {
		ticket["tags"] = tags
	}
	if fields, err := parseCustomFields(args["custom_fields"]); err != nil {
		return nil, err
	} else if fields != nil {
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
	tags := r.StrSlice("tags")
	verified := false
	if _, ok := args["verified"]; ok {
		verified = r.Bool("verified")
	}
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
	if id, ok, err := optionalIntID(args, "organization_id"); err != nil {
		return nil, err
	} else if ok {
		user["organization_id"] = id
	}
	if len(tags) > 0 {
		user["tags"] = tags
	}
	if _, ok := args["verified"]; ok {
		user["verified"] = verified
	}
	return user, nil
}

func hasTypeFilter(query string) bool {
	for _, tok := range strings.Fields(query) {
		if strings.HasPrefix(strings.ToLower(tok), "type:") {
			return true
		}
	}
	return false
}

func optionalIntID(args map[string]any, key string) (int64, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false, nil
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("parameter %q: cannot convert string %q to int: %w", key, v, err)
		}
		return n, true, nil
	case float64:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case int64:
		return v, true, nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false, fmt.Errorf("parameter %q: cannot convert json.Number %q to int: %w", key, v.String(), err)
		}
		return n, true, nil
	default:
		n, err := mcp.ArgInt64(args, key)
		if err != nil {
			return 0, false, err
		}
		return n, true, nil
	}
}

func parseCustomFields(raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		var fields any
		if err := json.Unmarshal([]byte(v), &fields); err != nil {
			return nil, fmt.Errorf("invalid custom_fields JSON: %w", err)
		}
		return fields, nil
	case []any, map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("parameter %q: cannot convert %T to custom_fields", "custom_fields", v)
	}
}
