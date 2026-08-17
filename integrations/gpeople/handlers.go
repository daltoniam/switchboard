package gpeople

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// normalizeResourceName accepts either a full People API resource name
// (people/c12345), a bare ID (c12345), or "me" / "people/me" and returns
// the canonical "people/{id}" form. This lets callers pass either shape.
func normalizeResourceName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "people/") {
		return s
	}
	return "people/" + s
}

// parsePerson accepts the "person" argument as either a JSON-encoded
// string (the most common shape from LLM tool calls) or a pre-decoded
// map[string]any (when scripts construct the object directly). Returns
// (nil, nil) for absent/empty input; an error on invalid JSON.
func parsePerson(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return nil, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(x), &m); err != nil {
			return nil, fmt.Errorf("person: invalid JSON: %w", err)
		}
		return m, nil
	case map[string]any:
		if len(x) == 0 {
			return nil, nil
		}
		return x, nil
	default:
		return nil, fmt.Errorf("person: expected JSON object or string, got %T", v)
	}
}

// ── Contacts ────────────────────────────────────────────────────────

func listContacts(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	personFields := r.Str("person_fields")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	sortOrder := r.Str("sort_order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if personFields == "" {
		personFields = defaultPersonFields
	}

	params := url.Values{}
	params.Set("personFields", personFields)
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if sortOrder != "" {
		params.Set("sortOrder", sortOrder)
	}

	data, err := g.get(ctx, "/people/me/connections?%s", params.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchContacts(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	readMask := r.Str("read_mask")
	pageSize := r.OptInt("page_size", 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_search_contacts: query is required"))
	}
	if readMask == "" {
		readMask = "names,emailAddresses,phoneNumbers,organizations,addresses"
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("readMask", readMask)
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}

	data, err := g.get(ctx, "/people:searchContacts?%s", params.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPerson(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	resourceName := r.Str("resource_name")
	personFields := r.Str("person_fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if resourceName == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_get_person: resource_name is required"))
	}
	if personFields == "" {
		personFields = defaultPersonFields
	}

	rn := normalizeResourceName(resourceName)
	// rn is of form "people/{id}"; escape only the id segment.
	id := strings.TrimPrefix(rn, "people/")
	path := "/people/" + url.PathEscape(id) + "?personFields=" + url.QueryEscape(personFields)

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createContact(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	personFields := r.Str("person_fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["person"]; !ok {
		return mcp.ErrResult(fmt.Errorf("gpeople_create_contact: person is required"))
	}
	body, err := parsePerson(args["person"])
	if err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(fmt.Errorf("gpeople_create_contact: person is required"))
	}
	if personFields == "" {
		personFields = "names,emailAddresses,phoneNumbers,organizations,addresses,metadata"
	}

	path := "/people:createContact?personFields=" + url.QueryEscape(personFields)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateContact(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	resourceName := r.Str("resource_name")
	updatePersonFields := r.Str("update_person_fields")
	personFields := r.Str("person_fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if resourceName == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_update_contact: resource_name is required"))
	}
	if updatePersonFields == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_update_contact: update_person_fields is required"))
	}
	if _, ok := args["person"]; !ok {
		return mcp.ErrResult(fmt.Errorf("gpeople_update_contact: person is required"))
	}
	body, err := parsePerson(args["person"])
	if err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(fmt.Errorf("gpeople_update_contact: person is required"))
	}

	rn := normalizeResourceName(resourceName)
	id := strings.TrimPrefix(rn, "people/")
	params := url.Values{}
	params.Set("updatePersonFields", updatePersonFields)
	if personFields != "" {
		params.Set("personFields", personFields)
	}
	path := "/people/" + url.PathEscape(id) + ":updateContact?" + params.Encode()

	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteContact(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	resourceName := r.Str("resource_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if resourceName == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_delete_contact: resource_name is required"))
	}

	rn := normalizeResourceName(resourceName)
	id := strings.TrimPrefix(rn, "people/")
	path := "/people/" + url.PathEscape(id) + ":deleteContact"

	data, err := g.delete(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Directory ───────────────────────────────────────────────────────

func listDirectoryPeople(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	readMask := r.Str("read_mask")
	sources := r.Str("sources")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if readMask == "" {
		readMask = "names,emailAddresses,phoneNumbers,organizations,locations,metadata"
	}
	if sources == "" {
		sources = "DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT,DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE"
	}

	params := url.Values{}
	params.Set("readMask", readMask)
	for _, s := range strings.Split(sources, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			params.Add("sources", s)
		}
	}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	data, err := g.get(ctx, "/people:listDirectoryPeople?%s", params.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchDirectoryPeople(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	readMask := r.Str("read_mask")
	sources := r.Str("sources")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("gpeople_search_directory_people: query is required"))
	}
	if readMask == "" {
		readMask = "names,emailAddresses,phoneNumbers,organizations,locations,metadata"
	}
	if sources == "" {
		sources = "DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT,DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE"
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("readMask", readMask)
	for _, s := range strings.Split(sources, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			params.Add("sources", s)
		}
	}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	data, err := g.get(ctx, "/people:searchDirectoryPeople?%s", params.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Other contacts ──────────────────────────────────────────────────

func listOtherContacts(ctx context.Context, g *gpeople, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	readMask := r.Str("read_mask")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if readMask == "" {
		readMask = "names,emailAddresses,phoneNumbers,metadata"
	}

	params := url.Values{}
	params.Set("readMask", readMask)
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	data, err := g.get(ctx, "/otherContacts?%s", params.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
