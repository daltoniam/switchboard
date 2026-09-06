package hubspot

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("hubspot", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type hubspot struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var objectTypePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	_ mcp.Integration                = (*hubspot)(nil)
	_ mcp.FieldCompactionIntegration = (*hubspot)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*hubspot)(nil)
	_ mcp.PlainTextCredentials       = (*hubspot)(nil)
	_ mcp.PlaceholderHints           = (*hubspot)(nil)
	_ mcp.OptionalCredentials        = (*hubspot)(nil)
)

func (h *hubspot) PlainTextKeys() []string { return []string{"base_url"} }

func (h *hubspot) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "HubSpot private app access token",
		"base_url":     "https://api.hubapi.com (default)",
	}
}

func (h *hubspot) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &hubspot{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.hubapi.com",
	}
}

func (h *hubspot) Name() string { return "hubspot" }

func (h *hubspot) Configure(_ context.Context, creds mcp.Credentials) error {
	h.accessToken = creds["access_token"]
	if h.accessToken == "" {
		return fmt.Errorf("hubspot: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		h.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (h *hubspot) Healthy(ctx context.Context) bool {
	_, err := h.get(ctx, "/crm/v3/objects/contacts?limit=1")
	return err == nil
}

func (h *hubspot) Tools() []mcp.ToolDefinition {
	return tools
}

func (h *hubspot) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (h *hubspot) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (h *hubspot) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, h, args)
}

func (h *hubspot) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("hubspot API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hubspot API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (h *hubspot) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return h.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (h *hubspot) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return h.doRequest(ctx, http.MethodPost, path, body)
}

func (h *hubspot) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return h.doRequest(ctx, http.MethodPatch, path, body)
}

func (h *hubspot) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return h.doRequest(ctx, http.MethodPut, path, body)
}

func (h *hubspot) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return h.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, h *hubspot, args map[string]any) (*mcp.ToolResult, error)

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

func validObjectType(objectType string) error {
	if !objectTypePattern.MatchString(objectType) {
		return fmt.Errorf("invalid object_type %q: must be alphanumeric, hyphen, or underscore", objectType)
	}
	return nil
}

func clampLimit(n int) int {
	if n > 100 {
		return 100
	}
	return n
}

func objectProperties(objectType, extra string) string {
	def := defaultProperties[objectType]
	extra = strings.TrimSpace(extra)
	switch {
	case extra == "":
		return def
	case def == "":
		return extra
	default:
		return def + "," + extra
	}
}

var defaultProperties = map[string]string{
	"contacts":  "email,firstname,lastname,phone,jobtitle,company,lifecyclestage,hs_lead_status,hubspot_owner_id",
	"companies": "name,domain,industry,phone,city,state,country,numberofemployees,lifecyclestage,hubspot_owner_id",
	"deals":     "dealname,amount,dealstage,pipeline,closedate,hubspot_owner_id,hs_is_closed,hs_is_closed_won",
	"tickets":   "subject,content,hs_pipeline,hs_pipeline_stage,hs_ticket_priority,hubspot_owner_id,hs_ticket_category",
	"notes":     "hs_note_body,hs_timestamp,hubspot_owner_id",
	"tasks":     "hs_task_subject,hs_task_body,hs_task_status,hs_task_priority,hs_timestamp,hubspot_owner_id",
	"meetings":  "hs_meeting_title,hs_meeting_body,hs_meeting_start_time,hs_meeting_end_time,hs_meeting_outcome,hubspot_owner_id",
	"calls":     "hs_call_title,hs_call_body,hs_call_direction,hs_call_status,hs_timestamp,hubspot_owner_id",
	"emails":    "hs_email_subject,hs_email_text,hs_email_direction,hs_timestamp,hubspot_owner_id",
}

var searchProperties = map[string][]string{
	"contacts":  {"email", "firstname", "lastname", "phone", "company"},
	"companies": {"name", "domain", "website", "phone"},
	"deals":     {"dealname", "description"},
	"tickets":   {"subject", "content"},
	"notes":     {"hs_note_body"},
	"tasks":     {"hs_task_subject", "hs_task_body"},
	"meetings":  {"hs_meeting_title", "hs_meeting_body"},
	"calls":     {"hs_call_title", "hs_call_body"},
	"emails":    {"hs_email_subject", "hs_email_text"},
}

func listQuery(args map[string]any, objectType string) (map[string]string, error) {
	r := mcp.NewArgs(args)
	after := r.Str("after")
	properties := r.Str("properties")
	archived := r.Str("archived")
	associations := r.Str("associations")
	idProperty := r.Str("id_property")
	if err := r.Err(); err != nil {
		return nil, err
	}
	params := map[string]string{
		"limit":        strconv.Itoa(clampLimit(r.OptInt("limit", 10))),
		"after":        after,
		"properties":   objectProperties(objectType, properties),
		"associations": associations,
		"idProperty":   idProperty,
		"archived":     archived,
	}
	return params, nil
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("hubspot_search_contacts"):    searchContacts,
	mcp.ToolName("hubspot_list_contacts"):      listContacts,
	mcp.ToolName("hubspot_get_contact"):        getContact,
	mcp.ToolName("hubspot_create_contact"):     createContact,
	mcp.ToolName("hubspot_update_contact"):     updateContact,
	mcp.ToolName("hubspot_search_companies"):   searchCompanies,
	mcp.ToolName("hubspot_list_companies"):     listCompanies,
	mcp.ToolName("hubspot_get_company"):        getCompany,
	mcp.ToolName("hubspot_create_company"):     createCompany,
	mcp.ToolName("hubspot_update_company"):     updateCompany,
	mcp.ToolName("hubspot_search_deals"):       searchDeals,
	mcp.ToolName("hubspot_list_deals"):         listDeals,
	mcp.ToolName("hubspot_get_deal"):           getDeal,
	mcp.ToolName("hubspot_create_deal"):        createDeal,
	mcp.ToolName("hubspot_update_deal"):        updateDeal,
	mcp.ToolName("hubspot_search_tickets"):     searchTickets,
	mcp.ToolName("hubspot_list_tickets"):       listTickets,
	mcp.ToolName("hubspot_get_ticket"):         getTicket,
	mcp.ToolName("hubspot_create_ticket"):      createTicket,
	mcp.ToolName("hubspot_update_ticket"):      updateTicket,
	mcp.ToolName("hubspot_search_objects"):     searchObjects,
	mcp.ToolName("hubspot_list_objects"):       listObjects,
	mcp.ToolName("hubspot_get_object"):         getObject,
	mcp.ToolName("hubspot_create_object"):      createObject,
	mcp.ToolName("hubspot_update_object"):      updateObject,
	mcp.ToolName("hubspot_delete_object"):      deleteObject,
	mcp.ToolName("hubspot_list_associations"):  listAssociations,
	mcp.ToolName("hubspot_create_association"): createAssociation,
	mcp.ToolName("hubspot_list_owners"):        listOwners,
	mcp.ToolName("hubspot_list_pipelines"):     listPipelines,
	mcp.ToolName("hubspot_list_properties"):    listProperties,
}
