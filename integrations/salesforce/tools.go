package salesforce

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── SObject Describe ────────────────────────────────────────────
	{
		Name: mcp.ToolName("salesforce_describe_global"), Description: "List all SObjects available in the org with metadata (name, label, keyPrefix, queryable, createable, etc.). Start here for schema discovery.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("salesforce_describe_sobject"), Description: "Get detailed metadata for an SObject type including all fields, record types, child relationships, and URLs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type name (e.g. Account, Contact, Lead, Opportunity, Case, custom__c)", Required: true}},
	},

	// ── SObject CRUD ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("salesforce_get_record"), Description: "Get a single Salesforce record by ID. Optionally specify fields to return.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact, Lead, Opportunity)", Required: true}, {Name: mcp.ParamName("id"), Description: "Record ID (15 or 18 character Salesforce ID)", Required: true}, {Name: mcp.ParamName("fields"), Description: "Comma-separated field names to return (e.g. Id,Name,Email). Returns all fields if omitted."}},
	},
	{
		Name: mcp.ToolName("salesforce_create_record"), Description: "Create a new Salesforce record. Provide field values as a JSON object.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact, Lead, Opportunity)", Required: true}, {Name: mcp.ParamName("data"), Description: `JSON string of field name/value pairs (e.g. {"Name":"Acme","Industry":"Technology"})`, Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_update_record"), Description: "Update an existing Salesforce record. Provide only the fields to change.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact, Lead, Opportunity)", Required: true}, {Name: mcp.ParamName("id"), Description: "Record ID (15 or 18 character Salesforce ID)", Required: true}, {Name: mcp.ParamName("data"), Description: "JSON string of field name/value pairs to update", Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_delete_record"), Description: "Delete a Salesforce record by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact, Lead, Opportunity)", Required: true}, {Name: mcp.ParamName("id"), Description: "Record ID (15 or 18 character Salesforce ID)", Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_get_record_by_external_id"), Description: "Get a Salesforce record using an external ID field value",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact)", Required: true}, {Name: mcp.ParamName("field"), Description: "External ID field name", Required: true}, {Name: mcp.ParamName("value"), Description: "External ID value", Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_upsert_by_external_id"), Description: "Create or update a record using an external ID. Creates if not found, updates if exists.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sobject"), Description: "SObject type (e.g. Account, Contact)", Required: true}, {Name: mcp.ParamName("field"), Description: "External ID field name", Required: true}, {Name: mcp.ParamName("value"), Description: "External ID value", Required: true}, {Name:

		// ── Queries ─────────────────────────────────────────────────────
		mcp.ParamName("data"), Description: "JSON string of field name/value pairs", Required: true}},
	},

	{
		Name: mcp.ToolName("salesforce_query"), Description: "Execute a SOQL query to retrieve Salesforce records. Supports SELECT, WHERE, ORDER BY, LIMIT, GROUP BY, and relationship queries.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("q"), Description: "SOQL query string (e.g. SELECT Id, Name FROM Account WHERE Industry = 'Technology' LIMIT 10)", Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_query_more"), Description: "Retrieve the next batch of SOQL query results using a nextRecordsUrl from a previous query response",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("next_url"), Description: "The nextRecordsUrl value from a previous query response (e.g. /services/data/v62.0/query/01gxx0000...)", Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_search"), Description: "Execute a SOSL full-text search across multiple Salesforce objects. Use FIND clause with search terms.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("q"), Description: "SOSL search string (e.g. FIND {Acme} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email))", Required: true}},
	},

	// ── Metadata & Org ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("salesforce_list_api_versions"), Description: "List all available Salesforce REST API versions",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("salesforce_get_limits"), Description: "Get org API usage limits and current consumption (DailyApiRequests, DailyBulkApiRequests, etc.)",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("salesforce_list_recently_viewed"), Description: "List recently viewed records across all object types or for a specific object type",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Max number of records to return (default 200)"}},
	},

	// ── Composite ───────────────────────────────────────────────────
	{
		Name: mcp.ToolName("salesforce_composite_batch"), Description: "Execute up to 25 independent subrequests in a single API call. Each subrequest is a REST API request.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("requests"), Description: `JSON array of batch requests, each with method, url, and optional richInput (e.g. [{"method":"GET","url":"/services/data/v62.0/sobjects/Account/001xx"}])`, Required: true}},
	},
	{
		Name: mcp.ToolName("salesforce_sobject_collections"), Description: "Create, update, or delete up to 200 records of the same or different types in a single request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("method"), Description: "HTTP method: POST (create), PATCH (update), or DELETE (delete)", Required: true}, {Name: mcp.ParamName("records"), Description: `JSON array of records with attributes.type for create/update (e.g. [{"attributes":{"type":"Account"},"Name":"Acme"}])`}, {Name: mcp.ParamName("ids"), Description: "Comma-separated record IDs (required for DELETE only)"}, {Name: mcp.ParamName("all_or_none"), Description: "If true, roll back all if any fail (default false)"}},
	},
}
