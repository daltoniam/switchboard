package netsuite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

var recordTypePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

func validRecordType(recordType string) error {
	if !recordTypePattern.MatchString(recordType) {
		return fmt.Errorf("invalid record_type %q: must be alphanumeric/underscore", recordType)
	}
	return nil
}

func suiteQL(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := r.Str("q")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := pageParams(args)
	delete(params, "q")
	path := "/services/rest/query/v1/suiteql" + queryEncode(params)
	data, err := n.post(ctx, path, map[string]any{"q": q}, map[string]string{
		"Prefer": "transient",
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listRecords(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recordType := r.Str("record_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validRecordType(recordType); err != nil {
		return mcp.ErrResult(err)
	}
	return listTyped(ctx, n, recordType, args)
}

func getRecord(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recordType := r.Str("record_type")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validRecordType(recordType); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, recordType, id, args)
}

func createRecord(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recordType := r.Str("record_type")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validRecordType(recordType); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
	}
	path := fmt.Sprintf("/services/rest/record/v1/%s", url.PathEscape(recordType))
	data, err := n.post(ctx, path, body, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRecord(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recordType := r.Str("record_type")
	id := r.Str("id")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validRecordType(recordType); err != nil {
		return mcp.ErrResult(err)
	}
	var body any
	if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
	}
	path := fmt.Sprintf("/services/rest/record/v1/%s/%s", url.PathEscape(recordType), url.PathEscape(id))
	data, err := n.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteRecord(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recordType := r.Str("record_type")
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validRecordType(recordType); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.del(ctx, "/services/rest/record/v1/%s/%s", url.PathEscape(recordType), url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTyped(ctx context.Context, n *netsuite, recordType string, args map[string]any) (*mcp.ToolResult, error) {
	params := pageParams(args)
	path := fmt.Sprintf("/services/rest/record/v1/%s%s", url.PathEscape(recordType), queryEncode(params))
	data, err := n.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTyped(ctx context.Context, n *netsuite, recordType, id string, args map[string]any) (*mcp.ToolResult, error) {
	expand, _ := mcp.ArgStr(args, "expand")
	q := ""
	if strings.EqualFold(expand, "true") || expand == "1" {
		q = "?expandSubResources=true"
	}
	data, err := n.get(ctx, "/services/rest/record/v1/%s/%s%s", url.PathEscape(recordType), url.PathEscape(id), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCustomers(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "customer", args)
}

func getCustomer(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "customer", id, args)
}

func listVendors(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "vendor", args)
}

func getVendor(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "vendor", id, args)
}

func listInvoices(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "invoice", args)
}

func getInvoice(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "invoice", id, args)
}

func listBills(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "vendorbill", args)
}

func getBill(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "vendorbill", id, args)
}

func listPurchaseOrders(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "purchaseorder", args)
}

func getPurchaseOrder(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "purchaseorder", id, args)
}

func listSalesOrders(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "salesorder", args)
}

func getSalesOrder(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "salesorder", id, args)
}

func listEmployees(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "employee", args)
}

func getEmployee(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "employee", id, args)
}

func listSubsidiaries(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "subsidiary", args)
}

func listDepartments(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "department", args)
}

func listJournalEntries(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	return listTyped(ctx, n, "journalentry", args)
}

func getJournalEntry(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTyped(ctx, n, "journalentry", id, args)
}

func metadataCatalog(ctx context.Context, n *netsuite, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	selectType := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := "/services/rest/record/v1/metadata-catalog/"
	if selectType != "" {
		path += queryEncode(map[string]string{"select": selectType})
	}
	data, err := n.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
