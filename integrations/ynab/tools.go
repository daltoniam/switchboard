package ynab

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── User ───────────────────────────────────────────────────────
	{
		Name: "ynab_get_user", Description: "Get the authenticated user's information",
	},

	// ── Budgets ────────────────────────────────────────────────────
	{
		Name: "ynab_list_budgets", Description: "List all budgets the user has access to",
		Parameters: map[string]string{"include_accounts": "Include account data (true/false)"},
	},
	{
		Name: "ynab_get_budget", Description: "Get a single budget with all related entities (accounts, categories, payees, transactions, etc.)",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_budget_settings", Description: "Get budget settings including date and currency format",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},

	// ── Accounts ───────────────────────────────────────────────────
	{
		Name: "ynab_list_accounts", Description: "List all accounts in a budget",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_account", Description: "Get a single account by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "account_id": "Account ID"},
		Required:   []string{"account_id"},
	},
	{
		Name: "ynab_create_account", Description: "Create a new account in a budget",
		Parameters: map[string]string{
			"budget_id": "Budget ID (defaults to last-used)",
			"name":      "Account name",
			"type":      "Account type: checking, savings, cash, creditCard, lineOfCredit, otherAsset, otherLiability, mortgage, autoLoan, studentLoan, personalLoan, medicalDebt, otherDebt",
			"balance":   "Starting balance in milliunits (1000 = $1.00)",
		},
		Required: []string{"name", "type", "balance"},
	},

	// ── Categories ─────────────────────────────────────────────────
	{
		Name: "ynab_list_categories", Description: "List all category groups and their categories for a budget",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_category", Description: "Get a single category by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "category_id": "Category ID"},
		Required:   []string{"category_id"},
	},
	{
		Name: "ynab_create_category", Description: "Create a new category in a category group",
		Parameters: map[string]string{
			"budget_id":         "Budget ID (defaults to last-used)",
			"name":              "Category name",
			"category_group_id": "Category group ID to add this category to",
		},
		Required: []string{"name", "category_group_id"},
	},
	{
		Name: "ynab_update_category", Description: "Update a category's name or note",
		Parameters: map[string]string{
			"budget_id":   "Budget ID (defaults to last-used)",
			"category_id": "Category ID",
			"name":        "New category name",
			"note":        "Category note",
		},
		Required: []string{"category_id"},
	},
	{
		Name: "ynab_get_month_category", Description: "Get a category's budget amounts for a specific month",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "month": "Month in ISO date format (e.g. 2024-01-01) or 'current'", "category_id": "Category ID"},
		Required:   []string{"month", "category_id"},
	},
	{
		Name: "ynab_update_month_category", Description: "Update the budgeted/assigned amount for a category in a specific month",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "month": "Month in ISO date format or 'current'", "category_id": "Category ID", "budgeted": "Assigned amount in milliunits (1000 = $1.00)"},
		Required:   []string{"month", "category_id", "budgeted"},
	},

	// ── Category Groups ────────────────────────────────────────────
	{
		Name: "ynab_create_category_group", Description: "Create a new category group",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "name": "Category group name (max 50 chars)"},
		Required:   []string{"name"},
	},
	{
		Name: "ynab_update_category_group", Description: "Update a category group's name",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "category_group_id": "Category group ID", "name": "New category group name (max 50 chars)"},
		Required:   []string{"category_group_id", "name"},
	},

	// ── Payees ─────────────────────────────────────────────────────
	{
		Name: "ynab_list_payees", Description: "List all payees in a budget",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_payee", Description: "Get a single payee by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "payee_id": "Payee ID"},
		Required:   []string{"payee_id"},
	},
	{
		Name: "ynab_update_payee", Description: "Update a payee's name",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "payee_id": "Payee ID", "name": "New payee name (max 500 chars)"},
		Required:   []string{"payee_id", "name"},
	},

	// ── Payee Locations ────────────────────────────────────────────
	{
		Name: "ynab_list_payee_locations", Description: "List all payee locations (latitude/longitude) in a budget",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_payee_location", Description: "Get a single payee location by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "payee_location_id": "Payee location ID"},
		Required:   []string{"payee_location_id"},
	},
	{
		Name: "ynab_list_locations_for_payee", Description: "List all locations for a specific payee",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "payee_id": "Payee ID"},
		Required:   []string{"payee_id"},
	},

	// ── Months ─────────────────────────────────────────────────────
	{
		Name: "ynab_list_months", Description: "List all budget months (summary of each month's budget status)",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_month", Description: "Get a single budget month with category details",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "month": "Month in ISO date format (e.g. 2024-01-01) or 'current'"},
		Required:   []string{"month"},
	},

	// ── Transactions ───────────────────────────────────────────────
	{
		Name: "ynab_list_transactions", Description: "List transactions for a budget, optionally filtered by date or type",
		Parameters: map[string]string{
			"budget_id":  "Budget ID (defaults to last-used)",
			"since_date": "Only return transactions on or after this date (ISO format, e.g. 2024-01-01)",
			"type":       "Filter: uncategorized or unapproved",
		},
	},
	{
		Name: "ynab_get_transaction", Description: "Get a single transaction by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "transaction_id": "Transaction ID"},
		Required:   []string{"transaction_id"},
	},
	{
		Name: "ynab_list_account_transactions", Description: "List transactions for a specific account",
		Parameters: map[string]string{
			"budget_id":  "Budget ID (defaults to last-used)",
			"account_id": "Account ID",
			"since_date": "Only return transactions on or after this date (ISO format)",
			"type":       "Filter: uncategorized or unapproved",
		},
		Required: []string{"account_id"},
	},
	{
		Name: "ynab_list_category_transactions", Description: "List transactions for a specific category",
		Parameters: map[string]string{
			"budget_id":   "Budget ID (defaults to last-used)",
			"category_id": "Category ID",
			"since_date":  "Only return transactions on or after this date (ISO format)",
			"type":        "Filter: uncategorized or unapproved",
		},
		Required: []string{"category_id"},
	},
	{
		Name: "ynab_list_payee_transactions", Description: "List transactions for a specific payee",
		Parameters: map[string]string{
			"budget_id":  "Budget ID (defaults to last-used)",
			"payee_id":   "Payee ID",
			"since_date": "Only return transactions on or after this date (ISO format)",
			"type":       "Filter: uncategorized or unapproved",
		},
		Required: []string{"payee_id"},
	},
	{
		Name: "ynab_list_month_transactions", Description: "List transactions for a specific month",
		Parameters: map[string]string{
			"budget_id":  "Budget ID (defaults to last-used)",
			"month":      "Month in ISO date format (e.g. 2024-01-01) or 'current'",
			"since_date": "Only return transactions on or after this date (ISO format)",
			"type":       "Filter: uncategorized or unapproved",
		},
		Required: []string{"month"},
	},
	{
		Name: "ynab_create_transaction", Description: "Create a new transaction. Amounts are in milliunits (1000 = $1.00). Outflows are negative.",
		Parameters: map[string]string{
			"budget_id":   "Budget ID (defaults to last-used)",
			"account_id":  "Account ID (required)",
			"date":        "Transaction date in ISO format (required, e.g. 2024-01-15)",
			"amount":      "Amount in milliunits (negative for outflows, e.g. -50000 = -$50.00)",
			"payee_id":    "Payee ID",
			"payee_name":  "Payee name (max 200 chars, creates new payee if payee_id not given)",
			"category_id": "Category ID",
			"memo":        "Memo text (max 500 chars)",
			"cleared":     "Cleared status: cleared, uncleared, or reconciled",
			"approved":    "Whether transaction is approved (true/false)",
			"flag_color":  "Flag color: red, orange, yellow, green, blue, purple",
		},
		Required: []string{"account_id", "date", "amount"},
	},
	{
		Name: "ynab_update_transaction", Description: "Update an existing transaction",
		Parameters: map[string]string{
			"budget_id":      "Budget ID (defaults to last-used)",
			"transaction_id": "Transaction ID",
			"account_id":     "Account ID",
			"date":           "Transaction date in ISO format",
			"amount":         "Amount in milliunits",
			"payee_id":       "Payee ID",
			"payee_name":     "Payee name",
			"category_id":    "Category ID",
			"memo":           "Memo text",
			"cleared":        "Cleared status: cleared, uncleared, or reconciled",
			"approved":       "Whether transaction is approved (true/false)",
			"flag_color":     "Flag color: red, orange, yellow, green, blue, purple",
		},
		Required: []string{"transaction_id"},
	},
	{
		Name: "ynab_delete_transaction", Description: "Delete a transaction",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "transaction_id": "Transaction ID"},
		Required:   []string{"transaction_id"},
	},

	// ── Scheduled Transactions ─────────────────────────────────────
	{
		Name: "ynab_list_scheduled_transactions", Description: "List all scheduled (recurring) transactions for a budget",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)"},
	},
	{
		Name: "ynab_get_scheduled_transaction", Description: "Get a single scheduled transaction by ID",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "scheduled_transaction_id": "Scheduled transaction ID"},
		Required:   []string{"scheduled_transaction_id"},
	},
	{
		Name: "ynab_create_scheduled_transaction", Description: "Create a new scheduled (recurring) transaction",
		Parameters: map[string]string{
			"budget_id":   "Budget ID (defaults to last-used)",
			"account_id":  "Account ID (required)",
			"date":        "First occurrence date in ISO format (required, max 5 years in future)",
			"amount":      "Amount in milliunits (negative for outflows)",
			"frequency":   "Recurrence: never, daily, weekly, everyOtherWeek, twiceAMonth, every4Weeks, monthly, everyOtherMonth, every3Months, every4Months, twiceAYear, yearly, everyOtherYear",
			"payee_id":    "Payee ID",
			"payee_name":  "Payee name (max 200 chars)",
			"category_id": "Category ID",
			"memo":        "Memo text (max 500 chars)",
			"flag_color":  "Flag color: red, orange, yellow, green, blue, purple",
		},
		Required: []string{"account_id", "date", "amount", "frequency"},
	},
	{
		Name: "ynab_update_scheduled_transaction", Description: "Update an existing scheduled transaction",
		Parameters: map[string]string{
			"budget_id":                "Budget ID (defaults to last-used)",
			"scheduled_transaction_id": "Scheduled transaction ID",
			"account_id":               "Account ID",
			"date":                     "First occurrence date in ISO format",
			"amount":                   "Amount in milliunits",
			"frequency":                "Recurrence frequency",
			"payee_id":                 "Payee ID",
			"payee_name":               "Payee name",
			"category_id":              "Category ID",
			"memo":                     "Memo text",
			"flag_color":               "Flag color: red, orange, yellow, green, blue, purple",
		},
		Required: []string{"scheduled_transaction_id"},
	},
	{
		Name: "ynab_delete_scheduled_transaction", Description: "Delete a scheduled transaction",
		Parameters: map[string]string{"budget_id": "Budget ID (defaults to last-used)", "scheduled_transaction_id": "Scheduled transaction ID"},
		Required:   []string{"scheduled_transaction_id"},
	},
}
