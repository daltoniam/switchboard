package gpeople

import (
	"encoding/json"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gpeople_list_contacts":           renderConnectionsMD,
	"gpeople_search_contacts":         renderSearchResultsMD,
	"gpeople_get_person":              renderPersonMD,
	"gpeople_list_directory_people":   renderDirectoryPeopleMD,
	"gpeople_search_directory_people": renderDirectoryPeopleMD,
	"gpeople_list_other_contacts":     renderOtherContactsMD,
}

func (g *gpeople) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawName struct {
	DisplayName string `json:"displayName"`
	GivenName   string `json:"givenName"`
	FamilyName  string `json:"familyName"`
	MiddleName  string `json:"middleName"`
}

type rawEmail struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type rawPhone struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type rawOrg struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	Department string `json:"department"`
	StartDate  any    `json:"startDate"`
	EndDate    any    `json:"endDate"`
}

type rawAddress struct {
	FormattedValue string `json:"formattedValue"`
	StreetAddress  string `json:"streetAddress"`
	City           string `json:"city"`
	Region         string `json:"region"`
	PostalCode     string `json:"postalCode"`
	Country        string `json:"country"`
	Type           string `json:"type"`
}

type rawBio struct {
	Value       string `json:"value"`
	ContentType string `json:"contentType"`
}

type rawURL struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type rawLocation struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type rawPerson struct {
	ResourceName   string        `json:"resourceName"`
	Etag           string        `json:"etag"`
	Names          []rawName     `json:"names"`
	EmailAddresses []rawEmail    `json:"emailAddresses"`
	PhoneNumbers   []rawPhone    `json:"phoneNumbers"`
	Organizations  []rawOrg      `json:"organizations"`
	Addresses      []rawAddress  `json:"addresses"`
	Biographies    []rawBio      `json:"biographies"`
	URLs           []rawURL      `json:"urls"`
	Locations      []rawLocation `json:"locations"`
}

type rawConnectionsPage struct {
	Connections   []rawPerson `json:"connections"`
	NextPageToken string      `json:"nextPageToken"`
	TotalPeople   int         `json:"totalPeople"`
	TotalItems    int         `json:"totalItems"`
}

type rawSearchResult struct {
	Person rawPerson `json:"person"`
}

type rawSearchPage struct {
	Results []rawSearchResult `json:"results"`
}

type rawDirectoryPage struct {
	People        []rawPerson `json:"people"`
	NextPageToken string      `json:"nextPageToken"`
}

type rawOtherContactsPage struct {
	OtherContacts []rawPerson `json:"otherContacts"`
	NextPageToken string      `json:"nextPageToken"`
}

// ── Connections list (gpeople_list_contacts) ────────────────────────

func renderConnectionsMD(data []byte) (markdown.Markdown, bool) {
	var page rawConnectionsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasConn := probe["connections"]; !hasConn {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Contacts")
	if len(page.Connections) == 0 {
		b.BlankLine()
		b.Raw("_No contacts._\n")
		return b.Build(), true
	}

	writePeopleTable(b, page.Connections)

	var notes []string
	if page.TotalPeople > 0 {
		notes = append(notes, "total_people: "+itoa(page.TotalPeople))
	} else if page.TotalItems > 0 {
		notes = append(notes, "total_items: "+itoa(page.TotalItems))
	}
	if page.NextPageToken != "" {
		notes = append(notes, "next_page_token: "+page.NextPageToken)
	}
	if len(notes) > 0 {
		b.BlankLine()
		b.Attribution(notes...)
	}
	return b.Build(), true
}

// ── Search contacts (gpeople_search_contacts) ───────────────────────

func renderSearchResultsMD(data []byte) (markdown.Markdown, bool) {
	var page rawSearchPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasResults := probe["results"]; !hasResults {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Contact Search Results")
	if len(page.Results) == 0 {
		b.BlankLine()
		b.Raw("_No matches._\n")
		return b.Build(), true
	}

	people := make([]rawPerson, 0, len(page.Results))
	for _, r := range page.Results {
		people = append(people, r.Person)
	}
	writePeopleTable(b, people)
	return b.Build(), true
}

// ── Single person (gpeople_get_person) ──────────────────────────────

func renderPersonMD(data []byte) (markdown.Markdown, bool) {
	var p rawPerson
	if err := json.Unmarshal(data, &p); err != nil {
		return "", false
	}
	if p.ResourceName == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	title := primaryName(p.Names)
	if title == "" {
		title = "(unnamed person)"
	}
	b.Heading(1, title)

	var attrs []string
	attrs = append(attrs, "resource_name: "+p.ResourceName)
	if p.Etag != "" {
		attrs = append(attrs, "etag: "+p.Etag)
	}
	b.Attribution(attrs...)

	if len(p.EmailAddresses) > 0 {
		b.BlankLine()
		b.Heading(2, "Emails")
		var sb strings.Builder
		for _, e := range p.EmailAddresses {
			sb.WriteString("- ")
			sb.WriteString(e.Value)
			if e.Type != "" {
				sb.WriteString(" _(")
				sb.WriteString(e.Type)
				sb.WriteString(")_")
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	if len(p.PhoneNumbers) > 0 {
		b.BlankLine()
		b.Heading(2, "Phones")
		var sb strings.Builder
		for _, ph := range p.PhoneNumbers {
			sb.WriteString("- ")
			sb.WriteString(ph.Value)
			if ph.Type != "" {
				sb.WriteString(" _(")
				sb.WriteString(ph.Type)
				sb.WriteString(")_")
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	if len(p.Organizations) > 0 {
		b.BlankLine()
		b.Heading(2, "Organizations")
		var sb strings.Builder
		for _, o := range p.Organizations {
			sb.WriteString("- ")
			if o.Title != "" {
				sb.WriteString(o.Title)
				if o.Name != "" {
					sb.WriteString(" at ")
				}
			}
			if o.Name != "" {
				sb.WriteString(o.Name)
			}
			if o.Department != "" {
				sb.WriteString(" — ")
				sb.WriteString(o.Department)
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	if len(p.Addresses) > 0 {
		b.BlankLine()
		b.Heading(2, "Addresses")
		var sb strings.Builder
		for _, a := range p.Addresses {
			val := a.FormattedValue
			if val == "" {
				val = joinNonEmpty([]string{a.StreetAddress, a.City, a.Region, a.PostalCode, a.Country}, ", ")
			}
			sb.WriteString("- ")
			sb.WriteString(val)
			if a.Type != "" {
				sb.WriteString(" _(")
				sb.WriteString(a.Type)
				sb.WriteString(")_")
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	if len(p.Biographies) > 0 {
		bio := p.Biographies[0].Value
		if bio != "" {
			b.BlankLine()
			b.Heading(2, "Biography")
			b.Raw(bio)
			if !strings.HasSuffix(bio, "\n") {
				b.Raw("\n")
			}
		}
	}

	if len(p.URLs) > 0 {
		b.BlankLine()
		b.Heading(2, "URLs")
		var sb strings.Builder
		for _, u := range p.URLs {
			sb.WriteString("- ")
			sb.WriteString(u.Value)
			if u.Type != "" {
				sb.WriteString(" _(")
				sb.WriteString(u.Type)
				sb.WriteString(")_")
			}
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	return b.Build(), true
}

// ── Directory people (list + search) ────────────────────────────────

func renderDirectoryPeopleMD(data []byte) (markdown.Markdown, bool) {
	var page rawDirectoryPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasPeople := probe["people"]; !hasPeople {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Directory People")
	if len(page.People) == 0 {
		b.BlankLine()
		b.Raw("_No directory people._\n")
		return b.Build(), true
	}

	writePeopleTable(b, page.People)

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Other contacts ──────────────────────────────────────────────────

func renderOtherContactsMD(data []byte) (markdown.Markdown, bool) {
	var page rawOtherContactsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasOther := probe["otherContacts"]; !hasOther {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Other Contacts")
	if len(page.OtherContacts) == 0 {
		b.BlankLine()
		b.Raw("_No other contacts._\n")
		return b.Build(), true
	}

	writePeopleTable(b, page.OtherContacts)

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Helpers ─────────────────────────────────────────────────────────

// writePeopleTable writes a markdown table of people with name/email/phone/
// organization/resource columns. Cells are pipe-escaped so multi-valued
// fields (which we comma-join) don't break the row layout.
func writePeopleTable(b *markdown.Builder, people []rawPerson) {
	var sb strings.Builder
	rows := [][]string{{"Name", "Email", "Phone", "Organization", "Resource Name"}}
	for _, p := range people {
		rows = append(rows, []string{
			pipeSafe(primaryName(p.Names)),
			pipeSafe(joinEmails(p.EmailAddresses)),
			pipeSafe(joinPhones(p.PhoneNumbers)),
			pipeSafe(primaryOrg(p.Organizations)),
			pipeSafe(p.ResourceName),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())
}

func primaryName(names []rawName) string {
	for _, n := range names {
		if n.DisplayName != "" {
			return n.DisplayName
		}
	}
	for _, n := range names {
		joined := joinNonEmpty([]string{n.GivenName, n.MiddleName, n.FamilyName}, " ")
		if joined != "" {
			return joined
		}
	}
	return ""
}

func joinEmails(emails []rawEmail) string {
	parts := make([]string, 0, len(emails))
	for _, e := range emails {
		if e.Value != "" {
			parts = append(parts, e.Value)
		}
	}
	return strings.Join(parts, ", ")
}

func joinPhones(phones []rawPhone) string {
	parts := make([]string, 0, len(phones))
	for _, p := range phones {
		if p.Value != "" {
			parts = append(parts, p.Value)
		}
	}
	return strings.Join(parts, ", ")
}

func primaryOrg(orgs []rawOrg) string {
	for _, o := range orgs {
		switch {
		case o.Title != "" && o.Name != "":
			return o.Title + " at " + o.Name
		case o.Name != "":
			return o.Name
		case o.Title != "":
			return o.Title
		}
	}
	return ""
}

func joinNonEmpty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// pipeSafe escapes newlines and pipes so a cell stays on one row.
func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// itoa wraps strconv.Itoa for terse formatting in attribution lines.
func itoa(n int) string { return strconv.Itoa(n) }
