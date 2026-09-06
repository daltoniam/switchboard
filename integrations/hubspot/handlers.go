package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func parseJSONValue(raw, name string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", name, err)
	}
	return out, nil
}

func objectWriteBody(args map[string]any) (map[string]any, error) {
	r := mcp.NewArgs(args)
	propsRaw := r.Str("properties")
	assocRaw := r.Str("associations")
	if err := r.Err(); err != nil {
		return nil, err
	}
	props, err := parseJSONValue(propsRaw, "properties")
	if err != nil {
		return nil, err
	}
	if props == nil {
		return nil, fmt.Errorf("properties is required")
	}
	body := map[string]any{"properties": props}
	if assocRaw != "" {
		assoc, err := parseJSONValue(assocRaw, "associations")
		if err != nil {
			return nil, err
		}
		body["associations"] = assoc
	}
	return body, nil
}

func buildSearchBody(args map[string]any, objectType string) (map[string]any, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	filtersRaw := r.Str("filters")
	filterGroupsRaw := r.Str("filter_groups")
	properties := r.Str("properties")
	sortsRaw := r.Str("sorts")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"limit":      clampLimit(r.OptInt("limit", 10)),
		"properties": splitCSV(objectProperties(objectType, properties)),
	}
	if after != "" {
		body["after"] = after
	}
	if sortsRaw != "" {
		sorts, err := parseJSONValue(sortsRaw, "sorts")
		if err != nil {
			return nil, err
		}
		body["sorts"] = sorts
	}
	filterGroups, err := parseJSONValue(filterGroupsRaw, "filter_groups")
	if err != nil {
		return nil, err
	}
	if filterGroups != nil {
		body["filterGroups"] = filterGroups
		return body, nil
	}
	filters, err := parseJSONValue(filtersRaw, "filters")
	if err != nil {
		return nil, err
	}
	if filters != nil {
		body["filterGroups"] = []any{map[string]any{"filters": filters}}
		return body, nil
	}
	if query != "" {
		groups, err := queryFilterGroups(objectType, query)
		if err != nil {
			return nil, err
		}
		body["filterGroups"] = groups
	}
	return body, nil
}

func queryFilterGroups(objectType, query string) ([]any, error) {
	props := searchProperties[objectType]
	if len(props) == 0 {
		return nil, fmt.Errorf("query is not supported for object_type %q; use filters or filter_groups", objectType)
	}
	groups := make([]any, 0, len(props))
	for _, prop := range props {
		groups = append(groups, map[string]any{
			"filters": []any{
				map[string]any{
					"propertyName": prop,
					"operator":     "CONTAINS_TOKEN",
					"value":        query,
				},
			},
		})
	}
	return groups, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
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

func listTyped(ctx context.Context, h *hubspot, objectType string, args map[string]any) (*mcp.ToolResult, error) {
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	params, err := listQuery(args, objectType)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := h.get(ctx, "/crm/v3/objects/%s%s", url.PathEscape(objectType), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTyped(ctx context.Context, h *hubspot, objectType, id string, args map[string]any) (*mcp.ToolResult, error) {
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	properties := r.Str("properties")
	associations := r.Str("associations")
	idProperty := r.Str("id_property")
	archived := r.Str("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"properties":   objectProperties(objectType, properties),
		"associations": associations,
		"idProperty":   idProperty,
		"archived":     archived,
	}
	data, err := h.get(ctx, "/crm/v3/objects/%s/%s%s", url.PathEscape(objectType), url.PathEscape(id), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTyped(ctx context.Context, h *hubspot, objectType string, args map[string]any) (*mcp.ToolResult, error) {
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := objectWriteBody(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := h.post(ctx, "/crm/v3/objects/"+url.PathEscape(objectType), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTyped(ctx context.Context, h *hubspot, objectType, id string, args map[string]any) (*mcp.ToolResult, error) {
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := objectWriteBody(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	delete(body, "associations")
	data, err := h.patch(ctx, fmt.Sprintf("/crm/v3/objects/%s/%s", url.PathEscape(objectType), url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchTyped(ctx context.Context, h *hubspot, objectType string, args map[string]any) (*mcp.ToolResult, error) {
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := buildSearchBody(args, objectType)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := h.post(ctx, "/crm/v3/objects/"+url.PathEscape(objectType)+"/search", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchContacts(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return searchTyped(ctx, h, "contacts", args)
}

func listContacts(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, h, "contacts", args)
}

func getContact(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("contact_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, h, "contacts", id, args)
}

func createContact(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return createTyped(ctx, h, "contacts", args)
}

func updateContact(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("contact_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return updateTyped(ctx, h, "contacts", id, args)
}

func searchCompanies(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return searchTyped(ctx, h, "companies", args)
}

func listCompanies(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, h, "companies", args)
}

func getCompany(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("company_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, h, "companies", id, args)
}

func createCompany(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return createTyped(ctx, h, "companies", args)
}

func updateCompany(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("company_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return updateTyped(ctx, h, "companies", id, args)
}

func searchDeals(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return searchTyped(ctx, h, "deals", args)
}

func listDeals(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, h, "deals", args)
}

func getDeal(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("deal_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, h, "deals", id, args)
}

func createDeal(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return createTyped(ctx, h, "deals", args)
}

func updateDeal(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("deal_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return updateTyped(ctx, h, "deals", id, args)
}

func searchTickets(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return searchTyped(ctx, h, "tickets", args)
}

func listTickets(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, h, "tickets", args)
}

func getTicket(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, h, "tickets", id, args)
}

func createTicket(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	return createTyped(ctx, h, "tickets", args)
}

func updateTicket(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("ticket_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return updateTyped(ctx, h, "tickets", id, args)
}

func searchObjects(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return searchTyped(ctx, h, objectType, args)
}

func listObjects(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return listTyped(ctx, h, objectType, args)
}

func getObject(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, h, objectType, id, args)
}

func createObject(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return createTyped(ctx, h, objectType, args)
}

func updateObject(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return updateTyped(ctx, h, objectType, id, args)
}

func deleteObject(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := h.del(ctx, "/crm/v3/objects/%s/%s", url.PathEscape(objectType), url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAssociations(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fromType := r.Str("from_object_type")
	fromID := r.Str("from_object_id")
	toType := r.Str("to_object_type")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(fromType); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(toType); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"limit": strconv.Itoa(clampLimit(r.OptInt("limit", 10))),
		"after": after,
	}
	data, err := h.get(ctx, "/crm/v3/objects/%s/%s/associations/%s%s",
		url.PathEscape(fromType), url.PathEscape(fromID), url.PathEscape(toType), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAssociation(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fromType := r.Str("from_object_type")
	fromID := r.Str("from_object_id")
	toType := r.Str("to_object_type")
	toID := r.Str("to_object_id")
	typeID := r.Str("association_type_id")
	category := r.Str("association_category")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(fromType); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(toType); err != nil {
		return mcp.ErrResult(err)
	}
	if typeID == "" {
		path := fmt.Sprintf("/crm/v4/objects/%s/%s/associations/default/%s/%s",
			url.PathEscape(fromType), url.PathEscape(fromID), url.PathEscape(toType), url.PathEscape(toID))
		data, err := h.put(ctx, path, nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
	if category == "" {
		category = "HUBSPOT_DEFINED"
	}
	typeNum, err := strconv.Atoi(typeID)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("association_type_id must be an integer"))
	}
	path := fmt.Sprintf("/crm/v4/objects/%s/%s/associations/%s/%s",
		url.PathEscape(fromType), url.PathEscape(fromID), url.PathEscape(toType), url.PathEscape(toID))
	body := []any{map[string]any{
		"associationCategory": category,
		"associationTypeId":   typeNum,
	}}
	data, err := h.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOwners(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	email := r.Str("email")
	archived := r.Str("archived")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"limit":    strconv.Itoa(clampLimit(r.OptInt("limit", 10))),
		"after":    after,
		"email":    email,
		"archived": archived,
	}
	data, err := h.get(ctx, "/crm/v3/owners%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPipelines(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := h.get(ctx, "/crm/v3/pipelines/%s", url.PathEscape(objectType))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProperties(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	objectType := r.Str("object_type")
	archived := r.Str("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validObjectType(objectType); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"archived": archived}
	data, err := h.get(ctx, "/crm/v3/properties/%s%s", url.PathEscape(objectType), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
