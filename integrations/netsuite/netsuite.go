package netsuite

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("netsuite", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type netsuite struct {
	accountID      string
	consumerKey    string
	consumerSecret string
	tokenID        string
	tokenSecret    string
	accessToken    string
	baseURL        string
	client         *http.Client
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*netsuite)(nil)
	_ mcp.FieldCompactionIntegration = (*netsuite)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*netsuite)(nil)
	_ mcp.PlainTextCredentials       = (*netsuite)(nil)
	_ mcp.PlaceholderHints           = (*netsuite)(nil)
	_ mcp.OptionalCredentials        = (*netsuite)(nil)
)

func (n *netsuite) PlainTextKeys() []string {
	return []string{"account_id", "base_url"}
}

func (n *netsuite) Placeholders() map[string]string {
	return map[string]string{
		"account_id":      "NetSuite account ID (e.g. 1234567 or 1234567_SB1)",
		"consumer_key":    "Integration consumer key (TBA)",
		"consumer_secret": "Integration consumer secret (TBA)",
		"token_id":        "Access token ID (TBA)",
		"token_secret":    "Access token secret (TBA)",
		"access_token":    "OAuth 2.0 bearer token (alternative to TBA)",
		"base_url":        "Optional override (default https://{account}.suitetalk.api.netsuite.com)",
	}
}

func (n *netsuite) OptionalKeys() []string {
	return []string{"consumer_key", "consumer_secret", "token_id", "token_secret", "access_token", "base_url"}
}

func New() mcp.Integration {
	return &netsuite{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (n *netsuite) Name() string { return "netsuite" }

func (n *netsuite) Configure(_ context.Context, creds mcp.Credentials) error {
	n.accountID = strings.TrimSpace(creds["account_id"])
	n.consumerKey = creds["consumer_key"]
	n.consumerSecret = creds["consumer_secret"]
	n.tokenID = creds["token_id"]
	n.tokenSecret = creds["token_secret"]
	n.accessToken = creds["access_token"]
	if n.accountID == "" {
		return fmt.Errorf("netsuite: account_id is required")
	}
	hasTBA := n.consumerKey != "" && n.consumerSecret != "" && n.tokenID != "" && n.tokenSecret != ""
	hasOAuth2 := n.accessToken != ""
	if !hasTBA && !hasOAuth2 {
		return fmt.Errorf("netsuite: either access_token (OAuth 2.0) or TBA credentials (consumer_key, consumer_secret, token_id, token_secret) are required")
	}
	if v := creds["base_url"]; v != "" {
		n.baseURL = strings.TrimRight(v, "/")
	} else {
		// Account IDs with underscores (sandbox) use hyphens in the hostname.
		hostAccount := strings.ToLower(strings.ReplaceAll(n.accountID, "_", "-"))
		n.baseURL = fmt.Sprintf("https://%s.suitetalk.api.netsuite.com", hostAccount)
	}
	return nil
}

func (n *netsuite) Healthy(ctx context.Context) bool {
	_, err := n.get(ctx, "/services/rest/record/v1/metadata-catalog/")
	return err == nil
}

func (n *netsuite) Tools() []mcp.ToolDefinition {
	return tools
}

func (n *netsuite) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (n *netsuite) MaxBytes(toolName mcp.ToolName) (int, bool) {
	mb, ok := maxBytesByTool[toolName]
	return mb, ok
}

func (n *netsuite) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, n, args)
}

func (n *netsuite) doRequest(ctx context.Context, method, path string, body any, extraHeaders map[string]string) (json.RawMessage, error) {
	var bodyReader io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	fullURL := n.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	if err := n.setAuth(req); err != nil {
		return nil, err
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("netsuite API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("netsuite API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (n *netsuite) setAuth(req *http.Request) error {
	if n.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.accessToken)
		return nil
	}
	auth, err := n.tbaHeader(req.Method, req.URL.String())
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	return nil
}

// tbaHeader builds an OAuth 1.0 Authorization header for Token-Based Auth.
func (n *netsuite) tbaHeader(method, requestURL string) (string, error) {
	nonce, err := randomNonce(16)
	if err != nil {
		return "", err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	oauthParams := map[string]string{
		"oauth_consumer_key":     n.consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA256",
		"oauth_timestamp":        timestamp,
		"oauth_token":            n.tokenID,
		"oauth_version":          "1.0",
	}

	baseURL, queryParams, err := splitURL(requestURL)
	if err != nil {
		return "", err
	}

	// Collect all params for the signature base string.
	all := make(map[string]string, len(oauthParams)+len(queryParams))
	for k, v := range oauthParams {
		all[k] = v
	}
	for k, v := range queryParams {
		all[k] = v
	}

	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramParts []string
	for _, k := range keys {
		paramParts = append(paramParts, pctEncode(k)+"="+pctEncode(all[k]))
	}
	paramString := strings.Join(paramParts, "&")

	baseString := strings.ToUpper(method) + "&" + pctEncode(baseURL) + "&" + pctEncode(paramString)
	signingKey := pctEncode(n.consumerSecret) + "&" + pctEncode(n.tokenSecret)

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Realm uses the raw account ID (underscores preserved, uppercased).
	realm := strings.ToUpper(n.accountID)
	header := fmt.Sprintf(
		`OAuth realm="%s", oauth_consumer_key="%s", oauth_token="%s", oauth_signature_method="HMAC-SHA256", oauth_timestamp="%s", oauth_nonce="%s", oauth_version="1.0", oauth_signature="%s"`,
		realm,
		pctEncode(n.consumerKey),
		pctEncode(n.tokenID),
		timestamp,
		nonce,
		pctEncode(signature),
	)
	return header, nil
}

func splitURL(raw string) (base string, query map[string]string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, err
	}
	base = strings.Split(raw, "?")[0]
	// Prefer scheme://host/path without re-encoding issues.
	if u.Scheme != "" && u.Host != "" {
		base = u.Scheme + "://" + u.Host + u.EscapedPath()
	}
	query = map[string]string{}
	for k, vs := range u.Query() {
		if len(vs) > 0 {
			query[k] = vs[0]
		}
	}
	return base, query, nil
}

func pctEncode(s string) string {
	// RFC 3986 unreserved: ALPHA / DIGIT / "-" / "." / "_" / "~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func randomNonce(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (n *netsuite) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil, nil)
}

func (n *netsuite) getWithHeaders(ctx context.Context, path string, extraHeaders map[string]string) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodGet, path, nil, extraHeaders)
}

func (n *netsuite) post(ctx context.Context, path string, body any, extraHeaders map[string]string) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodPost, path, body, extraHeaders)
}

func (n *netsuite) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodPatch, path, body, nil)
}

func (n *netsuite) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return n.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil, nil)
}

type handlerFunc func(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error)

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

func pageParams(args map[string]any) map[string]string {
	r := mcp.NewArgs(args)
	limit := r.Str("limit")
	offset := r.Str("offset")
	q := r.Str("q")
	_ = r.Err()
	return map[string]string{
		"limit":  limit,
		"offset": offset,
		"q":      q,
	}
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("netsuite_suiteql"):              suiteQL,
	mcp.ToolName("netsuite_list_records"):         listRecords,
	mcp.ToolName("netsuite_get_record"):           getRecord,
	mcp.ToolName("netsuite_create_record"):        createRecord,
	mcp.ToolName("netsuite_update_record"):        updateRecord,
	mcp.ToolName("netsuite_delete_record"):        deleteRecord,
	mcp.ToolName("netsuite_list_customers"):       listCustomers,
	mcp.ToolName("netsuite_get_customer"):         getCustomer,
	mcp.ToolName("netsuite_list_vendors"):         listVendors,
	mcp.ToolName("netsuite_get_vendor"):           getVendor,
	mcp.ToolName("netsuite_list_invoices"):        listInvoices,
	mcp.ToolName("netsuite_get_invoice"):          getInvoice,
	mcp.ToolName("netsuite_list_bills"):           listBills,
	mcp.ToolName("netsuite_get_bill"):             getBill,
	mcp.ToolName("netsuite_list_purchase_orders"): listPurchaseOrders,
	mcp.ToolName("netsuite_get_purchase_order"):   getPurchaseOrder,
	mcp.ToolName("netsuite_list_sales_orders"):    listSalesOrders,
	mcp.ToolName("netsuite_get_sales_order"):      getSalesOrder,
	mcp.ToolName("netsuite_list_employees"):       listEmployees,
	mcp.ToolName("netsuite_get_employee"):         getEmployee,
	mcp.ToolName("netsuite_list_subsidiaries"):    listSubsidiaries,
	mcp.ToolName("netsuite_list_departments"):     listDepartments,
	mcp.ToolName("netsuite_list_journal_entries"): listJournalEntries,
	mcp.ToolName("netsuite_get_journal_entry"):    getJournalEntry,
	mcp.ToolName("netsuite_metadata_catalog"):     metadataCatalog,
}
