package netsuite

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("netsuite_suiteql"), Description: "Run a SuiteQL query against NetSuite ERP data (customers, vendors, invoices, transactions, inventory). Start here for ad-hoc reporting, joins, and flexible ERP searches.",
		Parameters: map[string]string{
			"q":      "SuiteQL query string (e.g. SELECT id, companyname FROM customer WHERE isinactive = 'F')",
			"limit":  "Max rows per page (default 100, max 1000)",
			"offset": "Pagination offset (must be divisible by limit)",
		},
		Required: []string{"q"},
	},
	{
		Name: mcp.ToolName("netsuite_list_records"), Description: "List NetSuite REST records of any type with optional filter query. Prefer typed list tools when available.",
		Parameters: map[string]string{
			"record_type": "REST record type (e.g. customer, vendor, invoice, vendorBill, salesOrder, purchaseOrder, employee, journalEntry)",
			"q":           "Optional collection filter query (e.g. companyName CONTAIN \"Acme\")",
			"limit":       "Page size (default 100, max 1000)",
			"offset":      "Pagination offset",
		},
		Required: []string{"record_type"},
	},
	{
		Name: mcp.ToolName("netsuite_get_record"), Description: "Get a single NetSuite REST record by type and internal ID. Use after list_records or SuiteQL.",
		Parameters: map[string]string{
			"record_type": "REST record type (e.g. customer, invoice)",
			"id":          "Internal ID of the record",
			"expand":      "If true, expand subresources (expandSubResources=true)",
		},
		Required: []string{"record_type", "id"},
	},
	{
		Name: mcp.ToolName("netsuite_create_record"), Description: "Create a NetSuite REST record. Provide field values as JSON.",
		Parameters: map[string]string{
			"record_type": "REST record type (e.g. customer, invoice, vendorBill)",
			"data":        "JSON object of field values (e.g. {\"companyName\":\"Acme\",\"subsidiary\":{\"id\":\"1\"}})",
		},
		Required: []string{"record_type", "data"},
	},
	{
		Name: mcp.ToolName("netsuite_update_record"), Description: "Update (PATCH) a NetSuite REST record. Send only fields to change.",
		Parameters: map[string]string{
			"record_type": "REST record type",
			"id":          "Internal ID",
			"data":        "JSON object of fields to update",
		},
		Required: []string{"record_type", "id", "data"},
	},
	{
		Name: mcp.ToolName("netsuite_delete_record"), Description: "Delete a NetSuite REST record by type and internal ID",
		Parameters: map[string]string{
			"record_type": "REST record type",
			"id":          "Internal ID",
		},
		Required: []string{"record_type", "id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_customers"), Description: "List NetSuite customers and accounts receivable entities",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_customer"), Description: "Get a NetSuite customer by internal ID. Use after list_customers.",
		Parameters: map[string]string{"id": "Customer internal ID", "expand": "If true, expand subresources"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_vendors"), Description: "List NetSuite vendors and suppliers for accounts payable",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_vendor"), Description: "Get a NetSuite vendor by internal ID. Use after list_vendors.",
		Parameters: map[string]string{"id": "Vendor internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_invoices"), Description: "List NetSuite customer invoices and AR billing documents",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_invoice"), Description: "Get a NetSuite invoice by internal ID. Use after list_invoices.",
		Parameters: map[string]string{"id": "Invoice internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_bills"), Description: "List NetSuite vendor bills (AP bills / vendorBill records)",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_bill"), Description: "Get a NetSuite vendor bill by internal ID. Use after list_bills.",
		Parameters: map[string]string{"id": "Vendor bill internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_purchase_orders"), Description: "List NetSuite purchase orders (PO documents)",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_purchase_order"), Description: "Get a NetSuite purchase order by internal ID. Use after list_purchase_orders.",
		Parameters: map[string]string{"id": "Purchase order internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_sales_orders"), Description: "List NetSuite sales orders",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_sales_order"), Description: "Get a NetSuite sales order by internal ID. Use after list_sales_orders.",
		Parameters: map[string]string{"id": "Sales order internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_employees"), Description: "List NetSuite employees",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_employee"), Description: "Get a NetSuite employee by internal ID. Use after list_employees.",
		Parameters: map[string]string{"id": "Employee internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_list_subsidiaries"), Description: "List NetSuite subsidiaries in a OneWorld account",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_list_departments"), Description: "List NetSuite departments used for classification and reporting",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_list_journal_entries"), Description: "List NetSuite journal entries and GL adjustments",
		Parameters: map[string]string{
			"q":      "Optional filter query",
			"limit":  "Page size (default 100)",
			"offset": "Pagination offset",
		},
	},
	{
		Name: mcp.ToolName("netsuite_get_journal_entry"), Description: "Get a NetSuite journal entry by internal ID. Use after list_journal_entries.",
		Parameters: map[string]string{"id": "Journal entry internal ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("netsuite_metadata_catalog"), Description: "Fetch the NetSuite REST metadata catalog. Omit select for the full catalog; set select to a record type (e.g. customer) to describe that type's schema.",
		Parameters: map[string]string{
			"select": "Optional record type path segment to describe (e.g. customer). Uses application/schema+json. Omit for the full catalog listing.",
		},
	},
}
