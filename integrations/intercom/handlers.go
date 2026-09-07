package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func parseJSONObject(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	return out, nil
}

func parseJSONArray(raw string) ([]any, error) {
	if raw == "" {
		return nil, nil
	}
	var out []any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON array: %w", err)
	}
	return out, nil
}

func searchBody(query any, perPage int, startingAfter, sortField, sortOrder string) map[string]any {
	body := map[string]any{"query": query}
	pagination := map[string]any{"per_page": perPage}
	if startingAfter != "" {
		pagination["starting_after"] = startingAfter
	}
	body["pagination"] = pagination
	if sortField != "" {
		sort := map[string]any{"field": sortField}
		if sortOrder != "" {
			sort["order"] = sortOrder
		}
		body["sort"] = sort
	}
	return body
}

func equalityQuery(field string, value any) map[string]any {
	return map[string]any{
		"field":    field,
		"operator": "=",
		"value":    value,
	}
}

func listQuery(args map[string]any, extra map[string]string) map[string]string {
	r := mcp.NewArgs(args)
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	_ = r.Err()
	params := map[string]string{
		"per_page":       strconv.Itoa(perPage),
		"starting_after": startingAfter,
	}
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func searchConversations(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryRaw := r.Str("query")
	state := r.Str("state")
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	sortField := r.Str("sort_field")
	sortOrder := r.Str("sort_order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var query any
	if queryRaw != "" {
		q, err := parseJSONObject(queryRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		query = q
	} else if state != "" {
		query = equalityQuery("state", state)
	} else {
		query = equalityQuery("open", true)
	}
	data, err := c.post(ctx, "/conversations/search", searchBody(query, perPage, startingAfter, sortField, sortOrder))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listConversations(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	params := listQuery(args, nil)
	data, err := c.get(ctx, "/conversations%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	displayAs := r.Str("display_as")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if displayAs == "" {
		displayAs = "plaintext"
	}
	params := map[string]string{"display_as": displayAs}
	data, err := c.get(ctx, "/conversations/%s%s", url.PathEscape(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func replyConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	body := r.Str("body")
	adminID := r.Str("admin_id")
	messageType := r.Str("message_type")
	replyType := r.Str("type")
	userID := r.Str("intercom_user_id")
	attachments := r.StrSlice("attachment_urls")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if messageType == "" {
		messageType = "comment"
	}
	if replyType == "" {
		replyType = "admin"
	}
	payload := map[string]any{
		"message_type": messageType,
		"type":         replyType,
		"body":         body,
	}
	if err := applyReplyActor(payload, replyType, adminID, userID); err != nil {
		return mcp.ErrResult(err)
	}
	if len(attachments) > 0 {
		payload["attachment_urls"] = attachments
	}
	data, err := c.post(ctx, "/conversations/"+url.PathEscape(id)+"/reply", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func closeConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	return manageConversation(ctx, c, args, "close")
}

func reopenConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	return manageConversation(ctx, c, args, "open")
}

func manageConversation(ctx context.Context, c *intercom, args map[string]any, messageType string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	adminID := r.Str("admin_id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{
		"message_type": messageType,
		"type":         "admin",
		"admin_id":     adminID,
	}
	if body != "" {
		payload["body"] = body
	}
	data, err := c.post(ctx, "/conversations/"+url.PathEscape(id)+"/parts", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func snoozeConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	adminID := r.Str("admin_id")
	until := r.Int64("snoozed_until")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{
		"message_type":  "snoozed",
		"type":          "admin",
		"admin_id":      adminID,
		"snoozed_until": until,
	}
	data, err := c.post(ctx, "/conversations/"+url.PathEscape(id)+"/parts", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func assignConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	adminID := r.Str("admin_id")
	assigneeID := r.Str("assignee_id")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{
		"message_type": "assignment",
		"type":         "admin",
		"admin_id":     adminID,
		"assignee_id":  assigneeID,
	}
	if body != "" {
		payload["body"] = body
	}
	data, err := c.post(ctx, "/conversations/"+url.PathEscape(id)+"/parts", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func tagConversation(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("conversation_id")
	tagID := r.Str("tag_id")
	adminID := r.Str("admin_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{
		"id":       tagID,
		"admin_id": adminID,
	}
	data, err := c.post(ctx, "/conversations/"+url.PathEscape(id)+"/tags", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchContacts(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryRaw := r.Str("query")
	email := r.Str("email")
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var query any
	if queryRaw != "" {
		q, err := parseJSONObject(queryRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		query = q
	} else if email != "" {
		query = equalityQuery("email", email)
	} else {
		return mcp.ErrResult(fmt.Errorf("provide email or query"))
	}
	data, err := c.post(ctx, "/contacts/search", searchBody(query, perPage, startingAfter, "", ""))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listContacts(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	params := listQuery(args, nil)
	data, err := c.get(ctx, "/contacts%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getContact(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("contact_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/contacts/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func contactPayload(r *mcp.Args) (map[string]any, error) {
	email := r.Str("email")
	externalID := r.Str("external_id")
	name := r.Str("name")
	phone := r.Str("phone")
	role := r.Str("role")
	customRaw := r.Str("custom_attributes")
	if err := r.Err(); err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if email != "" {
		payload["email"] = email
	}
	if externalID != "" {
		payload["external_id"] = externalID
	}
	if name != "" {
		payload["name"] = name
	}
	if phone != "" {
		payload["phone"] = phone
	}
	if role != "" {
		payload["role"] = role
	}
	if customRaw != "" {
		attrs, err := parseJSONObject(customRaw)
		if err != nil {
			return nil, err
		}
		payload["custom_attributes"] = attrs
	}
	return payload, nil
}

func createContact(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payload, err := contactPayload(r)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if payload["email"] == nil && payload["external_id"] == nil {
		return mcp.ErrResult(fmt.Errorf("provide email or external_id"))
	}
	data, err := c.post(ctx, "/contacts", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateContact(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("contact_id")
	payload, err := contactPayload(r)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.put(ctx, "/contacts/"+url.PathEscape(id), payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCompanies(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	companyID := r.Str("company_id")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"name":           name,
		"company_id":     companyID,
		"page":           strconv.Itoa(page),
		"per_page":       strconv.Itoa(perPage),
		"starting_after": startingAfter,
	}
	data, err := c.get(ctx, "/companies%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCompany(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/companies/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchTickets(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryRaw := r.Str("query")
	state := r.Str("state")
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var query any
	if queryRaw != "" {
		q, err := parseJSONObject(queryRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		query = q
	} else if state != "" {
		query = equalityQuery("state", state)
	} else {
		query = equalityQuery("open", true)
	}
	data, err := c.post(ctx, "/tickets/search", searchBody(query, perPage, startingAfter, "", ""))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTicket(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/tickets/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTicket(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	ticketTypeID := r.Str("ticket_type_id")
	contactID := r.Str("contact_id")
	contactsRaw := r.Str("contacts")
	title := r.Str("title")
	description := r.Str("description")
	adminAssignee := r.Str("admin_assignee_id")
	teamAssignee := r.Str("team_assignee_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{"ticket_type_id": ticketTypeID}
	if contactsRaw != "" {
		contacts, err := parseJSONArray(contactsRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		payload["contacts"] = contacts
	} else if contactID != "" {
		payload["contacts"] = []any{map[string]any{"id": contactID}}
	}
	if _, ok := payload["contacts"]; !ok {
		return mcp.ErrResult(fmt.Errorf("provide contact_id or contacts"))
	}
	if title != "" {
		payload["ticket_attributes"] = map[string]any{"_default_title_": title, "_default_description_": description}
	} else if description != "" {
		payload["ticket_attributes"] = map[string]any{"_default_description_": description}
	}
	assignment := map[string]any{}
	if adminAssignee != "" {
		assignment["admin_assignee_id"] = adminAssignee
	}
	if teamAssignee != "" {
		assignment["team_assignee_id"] = teamAssignee
	}
	if len(assignment) > 0 {
		payload["assignment"] = assignment
	}
	data, err := c.post(ctx, "/tickets", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTicket(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	assigneeID := r.Str("assignee_id")
	adminAssignee := r.Str("admin_assignee_id")
	teamAssignee := r.Str("team_assignee_id")
	attrsRaw := r.Str("ticket_attributes")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	payload := map[string]any{}
	if _, ok := args["open"]; ok {
		open, err := mcp.ArgBool(args, "open")
		if err != nil {
			return mcp.ErrResult(err)
		}
		payload["open"] = open
	}
	if assigneeID == "" {
		if adminAssignee != "" {
			assigneeID = adminAssignee
		} else {
			assigneeID = teamAssignee
		}
	}
	if assigneeID != "" {
		payload["assignee_id"] = assigneeID
	}
	if attrsRaw != "" {
		attrs, err := parseJSONObject(attrsRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		payload["ticket_attributes"] = attrs
	}
	data, err := c.put(ctx, "/tickets/"+url.PathEscape(id), payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func replyTicket(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	body := r.Str("body")
	adminID := r.Str("admin_id")
	messageType := r.Str("message_type")
	replyType := r.Str("type")
	userID := r.Str("intercom_user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if messageType == "" {
		messageType = "comment"
	}
	if replyType == "" {
		replyType = "admin"
	}
	payload := map[string]any{
		"message_type": messageType,
		"type":         replyType,
		"body":         body,
	}
	if err := applyReplyActor(payload, replyType, adminID, userID); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, "/tickets/"+url.PathEscape(id)+"/reply", payload)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchArticles(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	phrase := r.Str("phrase")
	perPage := r.OptInt("per_page", 20)
	startingAfter := r.Str("starting_after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"phrase":         phrase,
		"per_page":       strconv.Itoa(perPage),
		"starting_after": startingAfter,
	}
	data, err := c.get(ctx, "/articles/search%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listArticles(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	params := listQuery(args, nil)
	data, err := c.get(ctx, "/articles%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getArticle(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("article_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/articles/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAdmins(ctx context.Context, c *intercom, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/admins")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMe(ctx context.Context, c *intercom, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/me")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTeams(ctx context.Context, c *intercom, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/teams")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTeam(ctx context.Context, c *intercom, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/teams/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTags(ctx context.Context, c *intercom, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/tags")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTicketTypes(ctx context.Context, c *intercom, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/ticket_types")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func applyReplyActor(payload map[string]any, replyType, adminID, userID string) error {
	switch replyType {
	case "admin":
		if adminID == "" {
			return fmt.Errorf("admin_id is required when type is admin")
		}
		payload["admin_id"] = adminID
		return nil
	case "user":
		if userID == "" {
			return fmt.Errorf("intercom_user_id is required when type is user")
		}
		payload["intercom_user_id"] = userID
		return nil
	default:
		return fmt.Errorf("type must be admin or user, got %q", replyType)
	}
}
