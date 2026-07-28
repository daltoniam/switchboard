package ramp

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("ramp_list_transactions"), Description: "List corporate card spend transactions, expenses, and purchases on Ramp. Start here for expense tracking, spend review, merchant charges, and card transaction workflows.",
		Parameters: map[string]string{
			"from_date":     "Only return transactions on or after this ISO datetime",
			"to_date":       "Only return transactions on or before this ISO datetime",
			"user_id":       "Filter by cardholder user ID",
			"card_id":       "Filter by card ID",
			"department_id": "Filter by department ID",
			"location_id":   "Filter by location ID",
			"merchant_id":   "Filter by merchant ID",
			"state":         "Transaction state filter",
			"sync_status":   "Accounting sync status filter",
			"entity_id":     "Filter by entity ID",
			"page_size":     "Results per page (2-100, default 20)",
			"start":         "Pagination cursor (ID of last entity from previous page)",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_transaction"), Description: "Get details of a specific Ramp card transaction including merchant, amount, receipts, and accounting fields. Use after list_transactions.",
		Parameters: map[string]string{"transaction_id": "Transaction ID"},
		Required:   []string{"transaction_id"},
	},
	{
		Name: mcp.ToolName("ramp_update_transaction"), Description: "Update a Ramp transaction memo or accounting fields. Use after get_transaction.",
		Parameters: map[string]string{
			"transaction_id": "Transaction ID",
			"data":           "JSON object of fields to update (e.g. {\"memo\":\"Client dinner\"})",
		},
		Required: []string{"transaction_id", "data"},
	},
	{
		Name: mcp.ToolName("ramp_list_reimbursements"), Description: "List employee expense reimbursements and out-of-pocket spend claims on Ramp",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_reimbursement"), Description: "Get a specific employee reimbursement by ID. Use after list_reimbursements.",
		Parameters: map[string]string{"reimbursement_id": "Reimbursement ID"},
		Required:   []string{"reimbursement_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_bills"), Description: "List accounts payable bills and vendor invoices managed in Ramp Bill Pay",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_bill"), Description: "Get a specific bill or vendor invoice by ID including line items and payment status. Use after list_bills.",
		Parameters: map[string]string{"bill_id": "Bill ID"},
		Required:   []string{"bill_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_users"), Description: "List Ramp employees, cardholders, and users for the business",
		Parameters: map[string]string{
			"department_id": "Filter by department ID",
			"location_id":   "Filter by location ID",
			"page_size":     "Results per page (2-100, default 20)",
			"start":         "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_user"), Description: "Get a Ramp user or employee by ID. Use after list_users.",
		Parameters: map[string]string{"user_id": "User ID"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_virtual_cards"), Description: "List virtual corporate credit cards issued on Ramp",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_virtual_card"), Description: "Get a virtual card by ID. Use after list_virtual_cards.",
		Parameters: map[string]string{"card_id": "Virtual card ID"},
		Required:   []string{"card_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_physical_cards"), Description: "List physical corporate credit cards issued on Ramp",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_physical_card"), Description: "Get a physical card by ID. Use after list_physical_cards.",
		Parameters: map[string]string{"card_id": "Physical card ID"},
		Required:   []string{"card_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_receipts"), Description: "List receipts attached to Ramp spend and expenses",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_receipt"), Description: "Get a receipt by ID. Use after list_receipts.",
		Parameters: map[string]string{"receipt_id": "Receipt ID"},
		Required:   []string{"receipt_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_departments"), Description: "List departments used for Ramp spend allocation and reporting",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_department"), Description: "Get a department by ID. Use after list_departments.",
		Parameters: map[string]string{"department_id": "Department ID"},
		Required:   []string{"department_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_locations"), Description: "List office locations used for Ramp spend allocation",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_location"), Description: "Get a location by ID. Use after list_locations.",
		Parameters: map[string]string{"location_id": "Location ID"},
		Required:   []string{"location_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_merchants"), Description: "List merchants and vendors that appear on Ramp card transactions",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_list_vendors"), Description: "List accounts payable vendors configured for Ramp Bill Pay",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_vendor"), Description: "Get a bill-pay vendor by ID. Use after list_vendors.",
		Parameters: map[string]string{"vendor_id": "Vendor ID"},
		Required:   []string{"vendor_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_entities"), Description: "List legal entities on the Ramp business account",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_entity"), Description: "Get a legal entity by ID. Use after list_entities.",
		Parameters: map[string]string{"entity_id": "Entity ID"},
		Required:   []string{"entity_id"},
	},
	{
		Name: mcp.ToolName("ramp_list_bank_accounts"), Description: "List linked business bank accounts on Ramp",
		Parameters: map[string]string{
			"page_size": "Results per page (2-100, default 20)",
			"start":     "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("ramp_get_bank_account"), Description: "Get a linked bank account by ID. Use after list_bank_accounts.",
		Parameters: map[string]string{"bank_account_id": "Bank account ID"},
		Required:   []string{"bank_account_id"},
	},
}
