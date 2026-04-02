package salesforce

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── SObject Describe ────────────────────────────────────────────
	{
		Name: "salesforce_describe_global", Description: "List all SObjects available in the org with metadata (name, label, keyPrefix, queryable, createable, etc.)",
		Parameters: map[string]string{},
	},
	{
		Name: "salesforce_describe_sobject", Description: "Get detailed metadata for an SObject type including all fields, record types, child relationships, and URLs",
		Parameters: map[string]string{"sobject": "SObject type name (e.g. Account, Contact, Lead, Opportunity, Case, custom__c)"},
		Required:   []string{"sobject"},
	},

	// ── SObject CRUD ────────────────────────────────────────────────
	{
		Name: "salesforce_get_record", Description: "Get a single Salesforce record by ID. Optionally specify fields to return.",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact, Lead, Opportunity)",
			"id":      "Record ID (15 or 18 character Salesforce ID)",
			"fields":  "Comma-separated field names to return (e.g. Id,Name,Email). Returns all fields if omitted.",
		},
		Required: []string{"sobject", "id"},
	},
	{
		Name: "salesforce_create_record", Description: "Create a new Salesforce record. Provide field values as a JSON object.",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact, Lead, Opportunity)",
			"data":    "JSON string of field name/value pairs (e.g. {\"Name\":\"Acme\",\"Industry\":\"Technology\"})",
		},
		Required: []string{"sobject", "data"},
	},
	{
		Name: "salesforce_update_record", Description: "Update an existing Salesforce record. Provide only the fields to change.",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact, Lead, Opportunity)",
			"id":      "Record ID (15 or 18 character Salesforce ID)",
			"data":    "JSON string of field name/value pairs to update",
		},
		Required: []string{"sobject", "id", "data"},
	},
	{
		Name: "salesforce_delete_record", Description: "Delete a Salesforce record by ID",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact, Lead, Opportunity)",
			"id":      "Record ID (15 or 18 character Salesforce ID)",
		},
		Required: []string{"sobject", "id"},
	},
	{
		Name: "salesforce_get_record_by_external_id", Description: "Get a Salesforce record using an external ID field value",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact)",
			"field":   "External ID field name",
			"value":   "External ID value",
		},
		Required: []string{"sobject", "field", "value"},
	},
	{
		Name: "salesforce_upsert_by_external_id", Description: "Create or update a record using an external ID. Creates if not found, updates if exists.",
		Parameters: map[string]string{
			"sobject": "SObject type (e.g. Account, Contact)",
			"field":   "External ID field name",
			"value":   "External ID value",
			"data":    "JSON string of field name/value pairs",
		},
		Required: []string{"sobject", "field", "value", "data"},
	},

	// ── Queries ─────────────────────────────────────────────────────
	{
		Name: "salesforce_query", Description: "Execute a SOQL query to retrieve Salesforce records. Supports SELECT, WHERE, ORDER BY, LIMIT, GROUP BY, and relationship queries.",
		Parameters: map[string]string{
			"q": "SOQL query string (e.g. SELECT Id, Name FROM Account WHERE Industry = 'Technology' LIMIT 10)",
		},
		Required: []string{"q"},
	},
	{
		Name: "salesforce_query_more", Description: "Retrieve the next batch of SOQL query results using a nextRecordsUrl from a previous query response",
		Parameters: map[string]string{
			"next_url": "The nextRecordsUrl value from a previous query response (e.g. /services/data/v62.0/query/01gxx0000...)",
		},
		Required: []string{"next_url"},
	},
	{
		Name: "salesforce_search", Description: "Execute a SOSL full-text search across multiple Salesforce objects. Use FIND clause with search terms.",
		Parameters: map[string]string{
			"q": "SOSL search string (e.g. FIND {Acme} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email))",
		},
		Required: []string{"q"},
	},

	// ── Metadata & Org ──────────────────────────────────────────────
	{
		Name: "salesforce_list_api_versions", Description: "List all available Salesforce REST API versions",
		Parameters: map[string]string{},
	},
	{
		Name: "salesforce_get_limits", Description: "Get org API usage limits and current consumption (DailyApiRequests, DailyBulkApiRequests, etc.)",
		Parameters: map[string]string{},
	},
	{
		Name: "salesforce_list_recently_viewed", Description: "List recently viewed records across all object types or for a specific object type",
		Parameters: map[string]string{
			"limit": "Max number of records to return (default 200)",
		},
	},

	// ── Composite ───────────────────────────────────────────────────
	{
		Name: "salesforce_composite_batch", Description: "Execute up to 25 independent subrequests in a single API call. Each subrequest is a REST API request.",
		Parameters: map[string]string{
			"requests": "JSON array of batch requests, each with method, url, and optional richInput (e.g. [{\"method\":\"GET\",\"url\":\"/services/data/v62.0/sobjects/Account/001xx\"}])",
		},
		Required: []string{"requests"},
	},
	{
		Name: "salesforce_sobject_collections", Description: "Create, update, or delete up to 200 records of the same or different types in a single request",
		Parameters: map[string]string{
			"method":      "HTTP method: POST (create), PATCH (update), or DELETE (delete)",
			"records":     "JSON array of records with attributes.type for create/update (e.g. [{\"attributes\":{\"type\":\"Account\"},\"Name\":\"Acme\"}])",
			"ids":         "Comma-separated record IDs (required for DELETE only)",
			"all_or_none": "If true, roll back all if any fail (default false)",
		},
		Required: []string{"method"},
	},
}
