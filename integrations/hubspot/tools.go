package hubspot

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("hubspot_search_contacts"), Description: "Search HubSpot CRM contacts, leads, and people by email, name, phone, or company. Start here for CRM lookup, lead tracking, and contact records.",
		Parameters: map[string]string{
			"query":         "Simple text search across email, firstname, lastname, phone, and company",
			"filters":       "JSON array of HubSpot filters ANDed together (e.g. [{\"propertyName\":\"email\",\"operator\":\"EQ\",\"value\":\"a@b.com\"}])",
			"filter_groups": "JSON array of filter groups (OR across groups, AND within a group). Overrides filters when set.",
			"properties":    "Comma-separated extra contact properties to return",
			"sorts":         "JSON array of sorts (e.g. [{\"propertyName\":\"lastmodifieddate\",\"direction\":\"DESCENDING\"}])",
			"limit":         "Page size (default 10, max 100)",
			"after":         "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_list_contacts"), Description: "List HubSpot CRM contacts and leads. Prefer search_contacts when looking up a person by email or name.",
		Parameters: map[string]string{
			"properties":   "Comma-separated extra contact properties to return",
			"associations": "Comma-separated associated object types to include (e.g. companies,deals)",
			"archived":     "If true, list archived contacts",
			"limit":        "Page size (default 10, max 100)",
			"after":        "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_get_contact"), Description: "Get a HubSpot CRM contact or lead by ID (or email via id_property). Use after search_contacts or list_contacts.",
		Parameters: map[string]string{
			"contact_id":   "Contact ID, or email when id_property=email",
			"id_property":  "Unique property to look up by instead of ID (e.g. email)",
			"properties":   "Comma-separated extra contact properties to return",
			"associations": "Comma-separated associated object types to include (e.g. companies,deals)",
			"archived":     "If true, include archived contact",
		},
		Required: []string{"contact_id"},
	},
	{
		Name: mcp.ToolName("hubspot_create_contact"), Description: "Create a HubSpot CRM contact or lead. Provide properties as JSON (email recommended).",
		Parameters: map[string]string{
			"properties":   "JSON object of contact properties (e.g. {\"email\":\"a@b.com\",\"firstname\":\"Ada\",\"lastname\":\"Lovelace\"})",
			"associations": "Optional JSON array of associations to create with the contact",
		},
		Required: []string{"properties"},
	},
	{
		Name: mcp.ToolName("hubspot_update_contact"), Description: "Update a HubSpot CRM contact or lead. Send only properties to change. Use after get_contact.",
		Parameters: map[string]string{
			"contact_id": "Contact ID",
			"properties": "JSON object of properties to update",
		},
		Required: []string{"contact_id", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_search_companies"), Description: "Search HubSpot CRM companies and accounts by name, domain, website, or phone",
		Parameters: map[string]string{
			"query":         "Simple text search across name, domain, website, and phone",
			"filters":       "JSON array of HubSpot filters ANDed together",
			"filter_groups": "JSON array of filter groups. Overrides filters when set.",
			"properties":    "Comma-separated extra company properties to return",
			"sorts":         "JSON array of sorts",
			"limit":         "Page size (default 10, max 100)",
			"after":         "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_list_companies"), Description: "List HubSpot CRM companies and accounts. Prefer search_companies when looking up a firm by name or domain.",
		Parameters: map[string]string{
			"properties":   "Comma-separated extra company properties to return",
			"associations": "Comma-separated associated object types to include (e.g. contacts,deals)",
			"archived":     "If true, list archived companies",
			"limit":        "Page size (default 10, max 100)",
			"after":        "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_get_company"), Description: "Get a HubSpot CRM company or account by ID (or domain via id_property). Use after search_companies or list_companies.",
		Parameters: map[string]string{
			"company_id":   "Company ID, or domain when id_property=domain",
			"id_property":  "Unique property to look up by instead of ID (e.g. domain)",
			"properties":   "Comma-separated extra company properties to return",
			"associations": "Comma-separated associated object types to include",
			"archived":     "If true, include archived company",
		},
		Required: []string{"company_id"},
	},
	{
		Name: mcp.ToolName("hubspot_create_company"), Description: "Create a HubSpot CRM company or account. Provide properties as JSON (name or domain recommended).",
		Parameters: map[string]string{
			"properties":   "JSON object of company properties (e.g. {\"name\":\"Acme\",\"domain\":\"acme.com\"})",
			"associations": "Optional JSON array of associations to create with the company",
		},
		Required: []string{"properties"},
	},
	{
		Name: mcp.ToolName("hubspot_update_company"), Description: "Update a HubSpot CRM company or account. Send only properties to change. Use after get_company.",
		Parameters: map[string]string{
			"company_id": "Company ID",
			"properties": "JSON object of properties to update",
		},
		Required: []string{"company_id", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_search_deals"), Description: "Search HubSpot CRM deals, opportunities, and pipeline records by name or description",
		Parameters: map[string]string{
			"query":         "Simple text search across dealname and description",
			"filters":       "JSON array of HubSpot filters ANDed together",
			"filter_groups": "JSON array of filter groups. Overrides filters when set.",
			"properties":    "Comma-separated extra deal properties to return",
			"sorts":         "JSON array of sorts",
			"limit":         "Page size (default 10, max 100)",
			"after":         "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_list_deals"), Description: "List HubSpot CRM deals, opportunities, and sales pipeline records. Prefer search_deals when looking up a specific opportunity.",
		Parameters: map[string]string{
			"properties":   "Comma-separated extra deal properties to return",
			"associations": "Comma-separated associated object types to include (e.g. contacts,companies)",
			"archived":     "If true, list archived deals",
			"limit":        "Page size (default 10, max 100)",
			"after":        "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_get_deal"), Description: "Get a HubSpot CRM deal or opportunity by ID. Use after search_deals or list_deals.",
		Parameters: map[string]string{
			"deal_id":      "Deal ID",
			"properties":   "Comma-separated extra deal properties to return",
			"associations": "Comma-separated associated object types to include",
			"archived":     "If true, include archived deal",
		},
		Required: []string{"deal_id"},
	},
	{
		Name: mcp.ToolName("hubspot_create_deal"), Description: "Create a HubSpot CRM deal or opportunity. Provide properties as JSON (dealname recommended).",
		Parameters: map[string]string{
			"properties":   "JSON object of deal properties (e.g. {\"dealname\":\"Acme renewal\",\"amount\":\"50000\",\"dealstage\":\"appointmentscheduled\"})",
			"associations": "Optional JSON array of associations to create with the deal",
		},
		Required: []string{"properties"},
	},
	{
		Name: mcp.ToolName("hubspot_update_deal"), Description: "Update a HubSpot CRM deal or opportunity (stage, amount, close date). Use after get_deal.",
		Parameters: map[string]string{
			"deal_id":    "Deal ID",
			"properties": "JSON object of properties to update",
		},
		Required: []string{"deal_id", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_search_tickets"), Description: "Search HubSpot service tickets and support issues by subject or content",
		Parameters: map[string]string{
			"query":         "Simple text search across subject and content",
			"filters":       "JSON array of HubSpot filters ANDed together",
			"filter_groups": "JSON array of filter groups. Overrides filters when set.",
			"properties":    "Comma-separated extra ticket properties to return",
			"sorts":         "JSON array of sorts",
			"limit":         "Page size (default 10, max 100)",
			"after":         "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_list_tickets"), Description: "List HubSpot service tickets and support issues. Prefer search_tickets when looking up a specific ticket.",
		Parameters: map[string]string{
			"properties":   "Comma-separated extra ticket properties to return",
			"associations": "Comma-separated associated object types to include (e.g. contacts,companies)",
			"archived":     "If true, list archived tickets",
			"limit":        "Page size (default 10, max 100)",
			"after":        "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_get_ticket"), Description: "Get a HubSpot service ticket by ID. Use after search_tickets or list_tickets.",
		Parameters: map[string]string{
			"ticket_id":    "Ticket ID",
			"properties":   "Comma-separated extra ticket properties to return",
			"associations": "Comma-separated associated object types to include",
			"archived":     "If true, include archived ticket",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("hubspot_create_ticket"), Description: "Create a HubSpot service ticket or support issue. Provide properties as JSON (subject recommended).",
		Parameters: map[string]string{
			"properties":   "JSON object of ticket properties (e.g. {\"subject\":\"Login issue\",\"hs_pipeline_stage\":\"1\"})",
			"associations": "Optional JSON array of associations to create with the ticket",
		},
		Required: []string{"properties"},
	},
	{
		Name: mcp.ToolName("hubspot_update_ticket"), Description: "Update a HubSpot service ticket (stage, priority, owner). Use after get_ticket.",
		Parameters: map[string]string{
			"ticket_id":  "Ticket ID",
			"properties": "JSON object of properties to update",
		},
		Required: []string{"ticket_id", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_search_objects"), Description: "Search any HubSpot CRM object type (notes, tasks, meetings, calls, emails, or custom objects). Prefer typed search tools for contacts, companies, deals, and tickets.",
		Parameters: map[string]string{
			"object_type":   "Object type (e.g. notes, tasks, meetings, calls, emails, or a custom object name)",
			"query":         "Simple text search when the object type has default searchable properties",
			"filters":       "JSON array of HubSpot filters ANDed together",
			"filter_groups": "JSON array of filter groups. Overrides filters when set.",
			"properties":    "Comma-separated properties to return",
			"sorts":         "JSON array of sorts",
			"limit":         "Page size (default 10, max 100)",
			"after":         "Pagination cursor from paging.next.after",
		},
		Required: []string{"object_type"},
	},
	{
		Name: mcp.ToolName("hubspot_list_objects"), Description: "List records of any HubSpot CRM object type (notes, tasks, meetings, calls, emails, or custom objects). Prefer typed list tools for contacts, companies, deals, and tickets.",
		Parameters: map[string]string{
			"object_type":  "Object type (e.g. notes, tasks, meetings, calls, emails)",
			"properties":   "Comma-separated properties to return",
			"associations": "Comma-separated associated object types to include",
			"archived":     "If true, list archived records",
			"limit":        "Page size (default 10, max 100)",
			"after":        "Pagination cursor from paging.next.after",
		},
		Required: []string{"object_type"},
	},
	{
		Name: mcp.ToolName("hubspot_get_object"), Description: "Get any HubSpot CRM object by type and ID. Use after list_objects or search_objects.",
		Parameters: map[string]string{
			"object_type":  "Object type (e.g. notes, tasks, meetings, calls, emails)",
			"id":           "Record ID",
			"id_property":  "Unique property to look up by instead of ID",
			"properties":   "Comma-separated properties to return",
			"associations": "Comma-separated associated object types to include",
			"archived":     "If true, include archived record",
		},
		Required: []string{"object_type", "id"},
	},
	{
		Name: mcp.ToolName("hubspot_create_object"), Description: "Create any HubSpot CRM object (notes, tasks, meetings, calls, emails, or custom objects). Prefer typed create tools for contacts, companies, deals, and tickets.",
		Parameters: map[string]string{
			"object_type":  "Object type (e.g. notes, tasks, meetings, calls, emails)",
			"properties":   "JSON object of properties",
			"associations": "Optional JSON array of associations to create with the record",
		},
		Required: []string{"object_type", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_update_object"), Description: "Update any HubSpot CRM object by type and ID. Send only properties to change.",
		Parameters: map[string]string{
			"object_type": "Object type",
			"id":          "Record ID",
			"properties":  "JSON object of properties to update",
		},
		Required: []string{"object_type", "id", "properties"},
	},
	{
		Name: mcp.ToolName("hubspot_delete_object"), Description: "Archive (delete) any HubSpot CRM object by type and ID, including contacts, companies, deals, and tickets",
		Parameters: map[string]string{
			"object_type": "Object type (e.g. contacts, companies, deals, tickets, notes)",
			"id":          "Record ID",
		},
		Required: []string{"object_type", "id"},
	},
	{
		Name: mcp.ToolName("hubspot_list_associations"), Description: "List HubSpot CRM associations linking a record to another object type (contact-company, deal-contact, ticket-company)",
		Parameters: map[string]string{
			"from_object_type": "Source object type (e.g. contacts)",
			"from_object_id":   "Source record ID",
			"to_object_type":   "Target object type (e.g. companies)",
			"after":            "Pagination cursor from paging.next.after",
			"limit":            "Page size (default 10, max 100)",
		},
		Required: []string{"from_object_type", "from_object_id", "to_object_type"},
	},
	{
		Name: mcp.ToolName("hubspot_create_association"), Description: "Associate two HubSpot CRM records (contact to company, deal to contact, ticket to company). Uses the default association type unless association_type_id is set.",
		Parameters: map[string]string{
			"from_object_type":     "Source object type (e.g. contacts)",
			"from_object_id":       "Source record ID",
			"to_object_type":       "Target object type (e.g. companies)",
			"to_object_id":         "Target record ID",
			"association_type_id":  "Optional HubSpot association type ID for a labeled association",
			"association_category": "Association category when type_id is set (default HUBSPOT_DEFINED)",
		},
		Required: []string{"from_object_type", "from_object_id", "to_object_type", "to_object_id"},
	},
	{
		Name: mcp.ToolName("hubspot_list_owners"), Description: "List HubSpot CRM owners, sales reps, and users used to assign contacts, companies, deals, and tickets",
		Parameters: map[string]string{
			"email":    "Optional owner email filter",
			"archived": "If true, list archived owners",
			"limit":    "Page size (default 10, max 100)",
			"after":    "Pagination cursor from paging.next.after",
		},
	},
	{
		Name: mcp.ToolName("hubspot_list_pipelines"), Description: "List HubSpot CRM pipelines and stages for deals or tickets. Use to resolve dealstage and hs_pipeline_stage values.",
		Parameters: map[string]string{
			"object_type": "Object type that has pipelines (deals or tickets)",
		},
		Required: []string{"object_type"},
	},
	{
		Name: mcp.ToolName("hubspot_list_properties"), Description: "List HubSpot CRM property definitions for an object type (schema, custom fields, field types). Use before create/update when a property name is unknown.",
		Parameters: map[string]string{
			"object_type": "Object type (e.g. contacts, companies, deals, tickets)",
			"archived":    "If true, include archived properties",
		},
		Required: []string{"object_type"},
	},
}
