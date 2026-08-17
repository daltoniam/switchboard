package gpeople

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Contacts ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gpeople_list_contacts"), Description: "List the authenticated user's Google Contacts (address book entries). Start here for contacts, people, address book, phone book, coworkers, emails of people the user knows — paginates through the full /people/me/connections collection. Returns each person with names, email addresses, phone numbers, organizations, and addresses by default; override person_fields for a different shape.",
		Parameters: map[string]string{
			"person_fields": "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,organizations,addresses,biographies,urls,photos,metadata'). Valid values: addresses, ageRanges, biographies, birthdays, calendarUrls, clientData, coverPhotos, emailAddresses, events, externalIds, genders, imClients, interests, locales, locations, memberships, metadata, miscKeywords, names, nicknames, occupations, organizations, phoneNumbers, photos, relations, sipAddresses, skills, urls, userDefined.",
			"page_size":     "Optional page size (1-1000, default 100)",
			"page_token":    "Optional pagination token from a previous response's nextPageToken",
			"sort_order":    "Optional sort: LAST_MODIFIED_ASCENDING, LAST_MODIFIED_DESCENDING, FIRST_NAME_ASCENDING, or LAST_NAME_ASCENDING (default LAST_MODIFIED_DESCENDING)",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gpeople_search_contacts"), Description: "Search the authenticated user's Google Contacts by name, email, phone, or organization. Use for queries like 'find John', 'who is jane@example.com', 'contacts at Acme'. Returns matched people with names, email addresses, phone numbers, organizations, and addresses by default. The search is across name + email + phone + nickname + organization; partial / prefix matches are supported.",
		Parameters: map[string]string{
			"query":     "Substring to match against the contact's names/nicknames/emails/phone numbers/organizations (e.g. 'jane', 'acme.com')",
			"read_mask": "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,organizations,addresses'). See gpeople_list_contacts for valid values.",
			"page_size": "Optional page size (1-30, default 10). The People API caps searchContacts at 30 results per page.",
		},
		Required: []string{"query"},
	},
	{
		Name: mcp.ToolName("gpeople_get_person"), Description: "Retrieve a single person (contact or directory profile) by resource name. Accepts 'people/c12345', the bare ID 'c12345', or 'me' for the authenticated user's own profile. Use when you already have a resource name from a previous list/search call.",
		Parameters: map[string]string{
			"resource_name": "Person resource name (e.g. 'people/c12345'), bare ID, or 'me' for the authenticated user",
			"person_fields": "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,organizations,addresses,biographies,urls,photos,metadata'). See gpeople_list_contacts for valid values.",
		},
		Required: []string{"resource_name"},
	},
	{
		Name: mcp.ToolName("gpeople_create_contact"), Description: "Create a new Google Contact in the authenticated user's address book. Pass a 'person' object containing the field arrays you want set (names, emailAddresses, phoneNumbers, organizations, addresses, biographies, urls, etc.). Returns the new person including its resourceName. Use person_fields to control which fields appear in the response.",
		Parameters: map[string]string{
			"person":        "Person resource object (JSON object OR JSON string). Example: {\"names\":[{\"givenName\":\"Jane\",\"familyName\":\"Doe\"}],\"emailAddresses\":[{\"value\":\"jane@example.com\",\"type\":\"work\"}],\"phoneNumbers\":[{\"value\":\"+1-555-0100\",\"type\":\"mobile\"}],\"organizations\":[{\"name\":\"Acme\",\"title\":\"Engineer\"}]}",
			"person_fields": "Optional comma-separated field mask controlling the response shape (default 'names,emailAddresses,phoneNumbers,organizations,addresses,metadata').",
		},
		Required: []string{"person"},
	},
	{
		Name: mcp.ToolName("gpeople_update_contact"), Description: "Update fields on an existing Google Contact. Uses PATCH semantics — only the fields named in update_person_fields are replaced. Requires the contact's etag (from a previous list/get) to detect concurrent modifications; mismatched etags return 400 with reason 'failedPrecondition'.",
		Parameters: map[string]string{
			"resource_name":        "Person resource name (e.g. 'people/c12345') or bare ID",
			"person":               "Person object (JSON object OR JSON string) with the new field values and the current etag. The etag MUST be present and match the server's current value. Example: {\"etag\":\"%EgwBAi4...\",\"names\":[{\"givenName\":\"Jane\",\"familyName\":\"Smith\"}],\"emailAddresses\":[{\"value\":\"jane@new.com\",\"type\":\"work\"}]}",
			"update_person_fields": "Required comma-separated mask of fields to update (e.g. 'names,emailAddresses'). Valid values: addresses, biographies, birthdays, calendarUrls, clientData, emailAddresses, events, externalIds, genders, imClients, interests, locales, locations, memberships, miscKeywords, names, nicknames, occupations, organizations, phoneNumbers, relations, sipAddresses, urls, userDefined. NOTE: photos and coverPhotos cannot be updated via this endpoint.",
			"person_fields":        "Optional comma-separated mask controlling the response shape (default 'names,emailAddresses,phoneNumbers,organizations,addresses,metadata').",
		},
		Required: []string{"resource_name", "person", "update_person_fields"},
	},
	{
		Name: mcp.ToolName("gpeople_delete_contact"), Description: "Permanently delete a Google Contact. The contact is removed from the authenticated user's address book and is not recoverable. Returns no content on success.",
		Parameters: map[string]string{
			"resource_name": "Person resource name (e.g. 'people/c12345') or bare ID",
		},
		Required: []string{"resource_name"},
	},

	// ── Directory (Google Workspace) ────────────────────────────────
	{
		Name: mcp.ToolName("gpeople_list_directory_people"), Description: "List people from the authenticated user's Google Workspace directory (coworkers in the same organization). Requires the user to be on a Google Workspace account with directory access enabled. Returns each person with their name, email, phone, organization, and job title by default.",
		Parameters: map[string]string{
			"read_mask":  "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,organizations,locations,metadata'). See gpeople_list_contacts for valid values.",
			"sources":    "Optional comma-separated source types (default 'DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT,DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE'). Values: DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT (shared contacts), DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE (organization members).",
			"page_size":  "Optional page size (1-1000, default 100)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gpeople_search_directory_people"), Description: "Search the authenticated user's Google Workspace directory by name, email, or job title. Use for queries like 'find coworker John', 'who is the VP of Engineering', 'people on the design team'. Requires directory access on a Google Workspace account.",
		Parameters: map[string]string{
			"query":      "Substring to match against directory people's name/email/job title/department",
			"read_mask":  "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,organizations,locations,metadata').",
			"sources":    "Optional comma-separated source types (default 'DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT,DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE').",
			"page_size":  "Optional page size (1-500, default 30)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{"query"},
	},

	// ── Other contacts (auto-collected) ─────────────────────────────
	{
		Name: mcp.ToolName("gpeople_list_other_contacts"), Description: "List the authenticated user's 'Other contacts' — auto-collected contacts (people the user has emailed but never explicitly saved to their address book). Useful for surfacing implicit connections. Read-only.",
		Parameters: map[string]string{
			"read_mask":  "Optional comma-separated field mask (default 'names,emailAddresses,phoneNumbers,metadata'). NOTE: otherContacts only supports a limited set of fields — names, emailAddresses, phoneNumbers, photos, metadata.",
			"page_size":  "Optional page size (1-1000, default 100)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{},
	},
}
