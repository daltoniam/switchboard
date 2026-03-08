package ynab

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"ynab_list_budgets": {
		"data.budgets[].id", "data.budgets[].name", "data.budgets[].last_modified_on",
		"data.budgets[].first_month", "data.budgets[].last_month",
	},
	"ynab_list_accounts": {
		"data.accounts[].id", "data.accounts[].name", "data.accounts[].type",
		"data.accounts[].on_budget", "data.accounts[].closed", "data.accounts[].balance",
		"data.accounts[].cleared_balance", "data.accounts[].uncleared_balance",
		"data.accounts[].deleted",
	},
	"ynab_list_categories": {
		"data.category_groups[].id", "data.category_groups[].name",
		"data.category_groups[].hidden", "data.category_groups[].deleted",
		"data.category_groups[].categories[].id", "data.category_groups[].categories[].name",
		"data.category_groups[].categories[].hidden", "data.category_groups[].categories[].budgeted",
		"data.category_groups[].categories[].activity", "data.category_groups[].categories[].balance",
		"data.category_groups[].categories[].goal_type", "data.category_groups[].categories[].deleted",
	},
	"ynab_list_payees": {
		"data.payees[].id", "data.payees[].name",
		"data.payees[].transfer_account_id", "data.payees[].deleted",
	},
	"ynab_list_months": {
		"data.months[].month", "data.months[].income", "data.months[].budgeted",
		"data.months[].activity", "data.months[].to_be_budgeted", "data.months[].age_of_money",
		"data.months[].deleted",
	},
	"ynab_list_transactions": {
		"data.transactions[].id", "data.transactions[].date", "data.transactions[].amount",
		"data.transactions[].memo", "data.transactions[].cleared", "data.transactions[].approved",
		"data.transactions[].account_id", "data.transactions[].account_name",
		"data.transactions[].payee_id", "data.transactions[].payee_name",
		"data.transactions[].category_id", "data.transactions[].category_name",
		"data.transactions[].flag_color", "data.transactions[].deleted",
	},
	"ynab_list_account_transactions": {
		"data.transactions[].id", "data.transactions[].date", "data.transactions[].amount",
		"data.transactions[].memo", "data.transactions[].cleared", "data.transactions[].approved",
		"data.transactions[].payee_id", "data.transactions[].payee_name",
		"data.transactions[].category_id", "data.transactions[].category_name",
		"data.transactions[].flag_color", "data.transactions[].deleted",
	},
	"ynab_list_category_transactions": {
		"data.transactions[].id", "data.transactions[].date", "data.transactions[].amount",
		"data.transactions[].memo", "data.transactions[].cleared", "data.transactions[].approved",
		"data.transactions[].account_id", "data.transactions[].account_name",
		"data.transactions[].payee_id", "data.transactions[].payee_name",
		"data.transactions[].flag_color", "data.transactions[].deleted",
	},
	"ynab_list_payee_transactions": {
		"data.transactions[].id", "data.transactions[].date", "data.transactions[].amount",
		"data.transactions[].memo", "data.transactions[].cleared", "data.transactions[].approved",
		"data.transactions[].account_id", "data.transactions[].account_name",
		"data.transactions[].category_id", "data.transactions[].category_name",
		"data.transactions[].flag_color", "data.transactions[].deleted",
	},
	"ynab_list_scheduled_transactions": {
		"data.scheduled_transactions[].id", "data.scheduled_transactions[].date_first",
		"data.scheduled_transactions[].date_next", "data.scheduled_transactions[].frequency",
		"data.scheduled_transactions[].amount", "data.scheduled_transactions[].memo",
		"data.scheduled_transactions[].account_id", "data.scheduled_transactions[].account_name",
		"data.scheduled_transactions[].payee_id", "data.scheduled_transactions[].payee_name",
		"data.scheduled_transactions[].category_id", "data.scheduled_transactions[].category_name",
		"data.scheduled_transactions[].flag_color", "data.scheduled_transactions[].deleted",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("ynab: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
