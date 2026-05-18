package ynab

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── User ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("ynab_get_user"), Description: "Get the authenticated user's information",
	},

	// ── Budgets ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("ynab_list_budgets"), Description: "List all personal finance budgets the user has access to. Start here for budget, spending, and money management workflows.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("include_accounts"), Description: "Include account data (true/false)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_budget"), Description: "Get a single budget with all related entities (accounts, categories, payees, transactions, etc.)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_budget_settings"), Description: "Get budget settings including date and currency format",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},

	// ── Accounts ───────────────────────────────────────────────────
	{
		Name: mcp.ToolName("ynab_list_accounts"), Description: "List all accounts in a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_account"), Description: "Get a single account by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("account_id"), Description: "Account ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_create_account"), Description: "Create a new account in a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("name"), Description: "Account name", Required: true}, {Name: mcp.ParamName("type"), Description: "Account type: checking, savings, cash, creditCard, lineOfCredit, otherAsset, otherLiability, mortgage, autoLoan, studentLoan, personalLoan, medicalDebt, otherDebt", Required: true}, {Name: mcp.ParamName("balance"),

		// ── Categories ─────────────────────────────────────────────────
		Description: "Starting balance in milliunits (1000 = $1.00)", Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_categories"), Description: "List all category groups and their categories for a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_category"), Description: "Get a single category by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("category_id"), Description: "Category ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_create_category"), Description: "Create a new category in a category group",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("name"), Description: "Category name", Required: true}, {Name: mcp.ParamName("category_group_id"), Description: "Category group ID to add this category to", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_update_category"), Description: "Update a category's name or note",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("category_id"), Description: "Category ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New category name"}, {Name: mcp.ParamName("note"), Description: "Category note"}},
	},
	{
		Name: mcp.ToolName("ynab_get_month_category"), Description: "Get a category's budget amounts for a specific month",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("month"), Description: "Month in ISO date format (e.g. 2024-01-01) or 'current'", Required: true}, {Name: mcp.ParamName("category_id"), Description: "Category ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_update_month_category"), Description: "Update the budgeted/assigned amount for a category in a specific month",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("month"), Description: "Month in ISO date format or 'current'", Required: true}, {Name: mcp.ParamName("category_id"), Description: "Category ID", Required: true}, {Name: mcp.

		// ── Category Groups ────────────────────────────────────────────
		ParamName("budgeted"), Description: "Assigned amount in milliunits (1000 = $1.00)", Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_create_category_group"), Description: "Create a new category group",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("name"), Description: "Category group name (max 50 chars)", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_update_category_group"), Description: "Update a category group's name",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("category_group_id"), Description: "Category group ID", Required: true}, {Name: mcp.ParamName("name"), Description:

		// ── Payees ─────────────────────────────────────────────────────
		"New category group name (max 50 chars)", Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_payees"), Description: "List all payees in a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_payee"), Description: "Get a single payee by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_update_payee"), Description: "Update a payee's name",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID", Required: true}, {Name: mcp.ParamName(

		// ── Payee Locations ────────────────────────────────────────────
		"name"), Description: "New payee name (max 500 chars)", Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_payee_locations"), Description: "List all payee locations (latitude/longitude) in a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_payee_location"), Description: "Get a single payee location by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("payee_location_id"), Description: "Payee location ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_list_locations_for_payee"), Description: "List all locations for a specific payee",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("payee_id"), Description:

		// ── Months ─────────────────────────────────────────────────────
		"Payee ID", Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_months"), Description: "List all budget months (summary of each month's budget status)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_month"), Description: "Get a single budget month with category details",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("month"), Description: "Month in ISO date format (e.g. 2024-01-01) or 'current'",

		// ── Transactions ───────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_transactions"), Description: "List financial transactions (spending, expenses, purchases) for a budget, optionally filtered by date or type",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("since_date"), Description: "Only return transactions on or after this date (ISO format, e.g. 2024-01-01)"}, {Name: mcp.ParamName("type"), Description: "Filter: uncategorized or unapproved"}},
	},
	{
		Name: mcp.ToolName("ynab_get_transaction"), Description: "Get a single transaction by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("transaction_id"), Description: "Transaction ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_list_account_transactions"), Description: "List transactions for a specific account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("account_id"), Description: "Account ID", Required: true}, {Name: mcp.ParamName("since_date"), Description: "Only return transactions on or after this date (ISO format)"}, {Name: mcp.ParamName("type"), Description: "Filter: uncategorized or unapproved"}},
	},
	{
		Name: mcp.ToolName("ynab_list_category_transactions"), Description: "List transactions for a specific category",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("category_id"), Description: "Category ID", Required: true}, {Name: mcp.ParamName("since_date"), Description: "Only return transactions on or after this date (ISO format)"}, {Name: mcp.ParamName("type"), Description: "Filter: uncategorized or unapproved"}},
	},
	{
		Name: mcp.ToolName("ynab_list_payee_transactions"), Description: "List transactions for a specific payee",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID", Required: true}, {Name: mcp.ParamName("since_date"), Description: "Only return transactions on or after this date (ISO format)"}, {Name: mcp.ParamName("type"), Description: "Filter: uncategorized or unapproved"}},
	},
	{
		Name: mcp.ToolName("ynab_list_month_transactions"), Description: "List transactions for a specific month",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("month"), Description: "Month in ISO date format (e.g. 2024-01-01) or 'current'", Required: true}, {Name: mcp.ParamName("since_date"), Description: "Only return transactions on or after this date (ISO format)"}, {Name: mcp.ParamName("type"), Description: "Filter: uncategorized or unapproved"}},
	},
	{
		Name: mcp.ToolName("ynab_create_transaction"), Description: "Create a new transaction. Amounts are in milliunits (1000 = $1.00). Outflows are negative.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("account_id"), Description: "Account ID (required)", Required: true}, {Name: mcp.ParamName("date"), Description: "Transaction date in ISO format (required, e.g. 2024-01-15)", Required: true}, {Name: mcp.ParamName("amount"), Description: "Amount in milliunits (negative for outflows, e.g. -50000 = -$50.00)", Required: true}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID"}, {Name: mcp.ParamName("payee_name"), Description: "Payee name (max 200 chars, creates new payee if payee_id not given)"}, {Name: mcp.ParamName("category_id"), Description: "Category ID"}, {Name: mcp.ParamName("memo"), Description: "Memo text (max 500 chars)"}, {Name: mcp.ParamName("cleared"), Description: "Cleared status: cleared, uncleared, or reconciled"}, {Name: mcp.ParamName("approved"), Description: "Whether transaction is approved (true/false)"}, {Name: mcp.ParamName("flag_color"), Description: "Flag color: red, orange, yellow, green, blue, purple"}},
	},
	{
		Name: mcp.ToolName("ynab_update_transaction"), Description: "Update an existing transaction",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("transaction_id"), Description: "Transaction ID", Required: true}, {Name: mcp.ParamName("account_id"), Description: "Account ID"}, {Name: mcp.ParamName("date"), Description: "Transaction date in ISO format"}, {Name: mcp.ParamName("amount"), Description: "Amount in milliunits"}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID"}, {Name: mcp.ParamName("payee_name"), Description: "Payee name"}, {Name: mcp.ParamName("category_id"), Description: "Category ID"}, {Name: mcp.ParamName("memo"), Description: "Memo text"}, {Name: mcp.ParamName("cleared"), Description: "Cleared status: cleared, uncleared, or reconciled"}, {Name: mcp.ParamName("approved"), Description: "Whether transaction is approved (true/false)"}, {Name: mcp.ParamName("flag_color"), Description: "Flag color: red, orange, yellow, green, blue, purple"}},
	},
	{
		Name: mcp.ToolName("ynab_delete_transaction"), Description: "Delete a transaction",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("transaction_id"), Description: "Transaction ID",

		// ── Scheduled Transactions ─────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("ynab_list_scheduled_transactions"), Description: "List all scheduled (recurring) transactions for a budget",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}},
	},
	{
		Name: mcp.ToolName("ynab_get_scheduled_transaction"), Description: "Get a single scheduled transaction by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("scheduled_transaction_id"), Description: "Scheduled transaction ID", Required: true}},
	},
	{
		Name: mcp.ToolName("ynab_create_scheduled_transaction"), Description: "Create a new scheduled (recurring) transaction",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("account_id"), Description: "Account ID (required)", Required: true}, {Name: mcp.ParamName("date"), Description: "First occurrence date in ISO format (required, max 5 years in future)", Required: true}, {Name: mcp.ParamName("amount"), Description: "Amount in milliunits (negative for outflows)", Required: true}, {Name: mcp.ParamName("frequency"), Description: "Recurrence: never, daily, weekly, everyOtherWeek, twiceAMonth, every4Weeks, monthly, everyOtherMonth, every3Months, every4Months, twiceAYear, yearly, everyOtherYear", Required: true}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID"}, {Name: mcp.ParamName("payee_name"), Description: "Payee name (max 200 chars)"}, {Name: mcp.ParamName("category_id"), Description: "Category ID"}, {Name: mcp.ParamName("memo"), Description: "Memo text (max 500 chars)"}, {Name: mcp.ParamName("flag_color"), Description: "Flag color: red, orange, yellow, green, blue, purple"}},
	},
	{
		Name: mcp.ToolName("ynab_update_scheduled_transaction"), Description: "Update an existing scheduled transaction",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("scheduled_transaction_id"), Description: "Scheduled transaction ID", Required: true}, {Name: mcp.ParamName("account_id"), Description: "Account ID"}, {Name: mcp.ParamName("date"), Description: "First occurrence date in ISO format"}, {Name: mcp.ParamName("amount"), Description: "Amount in milliunits"}, {Name: mcp.ParamName("frequency"), Description: "Recurrence frequency"}, {Name: mcp.ParamName("payee_id"), Description: "Payee ID"}, {Name: mcp.ParamName("payee_name"), Description: "Payee name"}, {Name: mcp.ParamName("category_id"), Description: "Category ID"}, {Name: mcp.ParamName("memo"), Description: "Memo text"}, {Name: mcp.ParamName("flag_color"), Description: "Flag color: red, orange, yellow, green, blue, purple"}},
	},
	{
		Name: mcp.ToolName("ynab_delete_scheduled_transaction"), Description: "Delete a scheduled transaction",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("budget_id"), Description: "Budget ID (defaults to last-used)"}, {Name: mcp.ParamName("scheduled_transaction_id"), Description: "Scheduled transaction ID", Required: true}},
	},
}
