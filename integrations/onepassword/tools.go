package onepassword

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "onepassword_list_vaults",
		Description: "List the 1Password vaults the service account can access, with id, title, description and item count. Start here for 1Password passwords, secrets, credentials and logins: pick a vault for onepassword_list_items.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "onepassword_list_items",
		Description: "List or search 1Password items (logins, passwords, secure notes, API credentials) in a vault, with id, title, category, website URLs and tags. Never returns secret values. Use after onepassword_list_vaults, then onepassword_get_item.",
		Parameters: map[string]string{
			"vault":    "Vault id or title from onepassword_list_vaults",
			"query":    "Optional text matched against item titles, website URLs and tags (case-insensitive)",
			"category": "Optional category filter, e.g. Login, Password, SecureNote, ApiCredentials",
			"limit":    "Maximum items to return, 1-1000 (default 200)",
			"offset":   "Matching items to skip, for the next page (use next_offset from the previous result; default 0)",
		},
		Required: []string{"vault"},
		Access:   mcp.AccessRead,
	},
	{
		Name: "onepassword_get_item",
		Description: "Get a 1Password item's fields, sections, notes, tags and website URLs. Concealed values (passwords, secrets, card numbers, one-time password seeds, SSH private keys) are returned as [REDACTED] unless reveal is true. " +
			"Each field carries an op:// secret reference for onepassword_resolve_secret. Use after onepassword_list_items.",
		Parameters: map[string]string{
			"vault":  "Vault id or title from onepassword_list_vaults",
			"item":   "Item id or exact title from onepassword_list_items",
			"reveal": "Return concealed values in plain text (default false). Only set when the user needs the secret itself",
		},
		Required: []string{"vault", "item"},
		Access:   mcp.AccessRead,
	},
	{
		Name: "onepassword_create_item",
		Description: "Create (add, save, store) a new 1Password item such as a login, password, secure note or API credential. WRITE: adds an item to a shared vault. " +
			"Set username and password for logins, or generate_password for a random one; add other values through fields. Concealed values are redacted in the response. Use after onepassword_list_vaults.",
		Parameters: map[string]string{
			"vault":             "Vault id or title from onepassword_list_vaults",
			"title":             "Item title",
			"category":          "Item category (default Login): Login, Password, SecureNote, ApiCredentials, Database, Server, CreditCard, BankAccount, Identity, SoftwareLicense, SshKey, Email, Membership, Router, and others",
			"username":          "Username, for Login items",
			"password":          "Password, for Login and Password items",
			"generate_password": "Generate a random 32-character password instead of passing one, for Login and Password items (default false)",
			"url":               "Website URL, for Login and Password items",
			"notes":             "Notes (plain text or Markdown)",
			"tags":              "Optional array of tags",
			"fields":            `Optional array of extra fields, each {"title","value","type","section"}. type is text (default), concealed, url, email, phone, date (YYYY-MM-DD) or totp (otpauth:// URI or seed); section is an optional section title`,
		},
		Required: []string{"vault", "title"},
		Access:   mcp.AccessWrite,
	},
	{
		Name: "onepassword_update_item",
		Description: "Update (edit, change) an existing 1Password item: set or add fields, change the password, username, title, notes or tags. WRITE: changes an item in a shared vault; 1Password keeps earlier versions in the item's history. " +
			"A field given by id or title is changed when it exists and added otherwise; fields not mentioned stay as they are. Concealed values are redacted in the response. Use after onepassword_get_item.",
		Parameters: map[string]string{
			"vault":             "Vault id or title from onepassword_list_vaults",
			"item":              "Item id or exact title from onepassword_list_items",
			"title":             "Optional new title",
			"username":          "Optional new username, for Login items",
			"password":          "Optional new password, for Login and Password items",
			"generate_password": "Replace the password with a random 32-character one, for Login and Password items (default false)",
			"notes":             "Optional new notes, replacing the old ones",
			"tags":              "Optional array of tags, replacing the old ones",
			"fields":            `Optional array of fields to set, each {"id" or "title","value","type","section"}. Matched by id, then by title (case-insensitive) within the section if given; unmatched fields are added. Omit value to change only the type; pass "" to clear it. type is text (default for new fields), concealed, url, email, phone, date or totp`,
		},
		Required: []string{"vault", "item"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "onepassword_resolve_secret",
		Description: "Resolve (read, reveal) one secret value from a 1Password secret reference such as op://vault/item/field or op://vault/item/section/field. Returns the plain-text value. Prefer the id-based reference from onepassword_get_item: a reference built from names works only when every name uses letters, digits, spaces, _, . or -.",
		Parameters: map[string]string{
			"reference": "Secret reference starting with op://",
		},
		Required: []string{"reference"},
		Access:   mcp.AccessRead,
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"onepassword_list_vaults":    listVaults,
	"onepassword_list_items":     listItems,
	"onepassword_get_item":       getItem,
	"onepassword_create_item":    createItem,
	"onepassword_update_item":    updateItem,
	"onepassword_resolve_secret": resolveSecret,
}
