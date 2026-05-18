package gpeople

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ──────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	// Every tool that returns document-style data must have a renderer.
	// Mutation tools (create/update/delete) return raw JSON envelopes.
	wantRendered := map[mcp.ToolName]bool{
		"gpeople_list_contacts":           true,
		"gpeople_search_contacts":         true,
		"gpeople_get_person":              true,
		"gpeople_list_directory_people":   true,
		"gpeople_search_directory_people": true,
		"gpeople_list_other_contacts":     true,
	}
	for name := range wantRendered {
		_, ok := markdownRenderers[name]
		assert.True(t, ok, "tool %s must have a markdown renderer", name)
	}
	for name := range markdownRenderers {
		_, ok := wantRendered[name]
		assert.True(t, ok, "renderer %s has no corresponding intended tool", name)
	}
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gpeople{}
	_, ok := g.RenderMarkdown("gpeople_create_contact", []byte(`{}`))
	assert.False(t, ok)
}

// ── List contacts ───────────────────────────────────────────────────

func TestRenderConnectionsMD_Basic(t *testing.T) {
	in := []byte(`{
        "connections": [
            {"resourceName":"people/c1","names":[{"displayName":"Jane Doe"}],"emailAddresses":[{"value":"jane@example.com","type":"work"}],"phoneNumbers":[{"value":"+1-555-0100"}],"organizations":[{"name":"Acme","title":"VP"}]},
            {"resourceName":"people/c2","names":[{"givenName":"Bob","familyName":"Smith"}],"emailAddresses":[{"value":"bob@example.com"}]}
        ],
        "totalPeople": 2
    }`)
	md, ok := renderConnectionsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Contacts")
	assert.Contains(t, s, "Jane Doe")
	assert.Contains(t, s, "jane@example.com")
	assert.Contains(t, s, "+1-555-0100")
	assert.Contains(t, s, "VP at Acme")
	// Falls back to given+family when displayName is missing.
	assert.Contains(t, s, "Bob Smith")
	assert.Contains(t, s, "people/c1")
	assert.Contains(t, s, "total_people: 2")
}

func TestRenderConnectionsMD_Empty(t *testing.T) {
	md, ok := renderConnectionsMD([]byte(`{"connections":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No contacts._")
}

func TestRenderConnectionsMD_WithPageToken(t *testing.T) {
	md, ok := renderConnectionsMD([]byte(`{"connections":[{"resourceName":"people/c1","names":[{"displayName":"X"}]}],"nextPageToken":"abc"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "next_page_token: abc")
}

func TestRenderConnectionsMD_InvalidJSON(t *testing.T) {
	_, ok := renderConnectionsMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderConnectionsMD_WrongShape(t *testing.T) {
	_, ok := renderConnectionsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Search contacts ─────────────────────────────────────────────────

func TestRenderSearchResultsMD_Basic(t *testing.T) {
	in := []byte(`{
        "results": [
            {"person":{"resourceName":"people/c1","names":[{"displayName":"Jane"}],"emailAddresses":[{"value":"jane@example.com"}]}},
            {"person":{"resourceName":"people/c2","names":[{"displayName":"John"}]}}
        ]
    }`)
	md, ok := renderSearchResultsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Contact Search Results")
	assert.Contains(t, s, "Jane")
	assert.Contains(t, s, "John")
}

func TestRenderSearchResultsMD_Empty(t *testing.T) {
	md, ok := renderSearchResultsMD([]byte(`{"results":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No matches._")
}

func TestRenderSearchResultsMD_WrongShape(t *testing.T) {
	_, ok := renderSearchResultsMD([]byte(`{"connections":[]}`))
	assert.False(t, ok)
}

// ── Single person ───────────────────────────────────────────────────

func TestRenderPersonMD_Basic(t *testing.T) {
	in := []byte(`{
        "resourceName":"people/c12345",
        "etag":"etag-abc",
        "names":[{"displayName":"Jane Doe","givenName":"Jane","familyName":"Doe"}],
        "emailAddresses":[{"value":"jane@work.com","type":"work"},{"value":"jane@home.com","type":"home"}],
        "phoneNumbers":[{"value":"+1-555-0100","type":"mobile"}],
        "organizations":[{"name":"Acme","title":"VP","department":"Eng"}],
        "addresses":[{"formattedValue":"123 Main St","type":"work"}],
        "biographies":[{"value":"Engineer and writer."}],
        "urls":[{"value":"https://example.com","type":"personal"}]
    }`)
	md, ok := renderPersonMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Jane Doe")
	assert.Contains(t, s, "resource_name: people/c12345")
	assert.Contains(t, s, "etag: etag-abc")
	assert.Contains(t, s, "## Emails")
	assert.Contains(t, s, "jane@work.com")
	assert.Contains(t, s, "_(work)_")
	assert.Contains(t, s, "## Phones")
	assert.Contains(t, s, "+1-555-0100")
	assert.Contains(t, s, "## Organizations")
	assert.Contains(t, s, "VP at Acme")
	assert.Contains(t, s, "Eng")
	assert.Contains(t, s, "## Addresses")
	assert.Contains(t, s, "123 Main St")
	assert.Contains(t, s, "## Biography")
	assert.Contains(t, s, "Engineer and writer.")
	assert.Contains(t, s, "## URLs")
	assert.Contains(t, s, "https://example.com")
}

func TestRenderPersonMD_UnnamedPerson(t *testing.T) {
	md, ok := renderPersonMD([]byte(`{"resourceName":"people/c1"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "(unnamed person)")
}

func TestRenderPersonMD_NoResourceName(t *testing.T) {
	_, ok := renderPersonMD([]byte(`{"names":[{"displayName":"Jane"}]}`))
	assert.False(t, ok, "renderer should refuse a response with no resourceName")
}

func TestRenderPersonMD_InvalidJSON(t *testing.T) {
	_, ok := renderPersonMD([]byte(`not json`))
	assert.False(t, ok)
}

func TestRenderPersonMD_AddressFallback(t *testing.T) {
	// No formattedValue, falls back to streetAddress + city + region + postalCode + country.
	in := []byte(`{"resourceName":"people/c1","names":[{"displayName":"X"}],"addresses":[{"streetAddress":"5 St","city":"NYC","region":"NY","postalCode":"10001","country":"US"}]}`)
	md, ok := renderPersonMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "5 St, NYC, NY, 10001, US")
}

// ── Directory people ────────────────────────────────────────────────

func TestRenderDirectoryPeopleMD_Basic(t *testing.T) {
	in := []byte(`{
        "people":[
            {"resourceName":"people/c1","names":[{"displayName":"Alice"}],"emailAddresses":[{"value":"alice@corp.com"}],"organizations":[{"name":"Corp","title":"PM"}]}
        ],
        "nextPageToken":"tok"
    }`)
	md, ok := renderDirectoryPeopleMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Directory People")
	assert.Contains(t, s, "Alice")
	assert.Contains(t, s, "PM at Corp")
	assert.Contains(t, s, "next_page_token: tok")
}

func TestRenderDirectoryPeopleMD_Empty(t *testing.T) {
	md, ok := renderDirectoryPeopleMD([]byte(`{"people":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No directory people._")
}

func TestRenderDirectoryPeopleMD_WrongShape(t *testing.T) {
	_, ok := renderDirectoryPeopleMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Other contacts ──────────────────────────────────────────────────

func TestRenderOtherContactsMD_Basic(t *testing.T) {
	in := []byte(`{
        "otherContacts":[
            {"resourceName":"otherContacts/c1","names":[{"displayName":"Stranger"}],"emailAddresses":[{"value":"strange@example.com"}]}
        ]
    }`)
	md, ok := renderOtherContactsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Other Contacts")
	assert.Contains(t, s, "Stranger")
	assert.Contains(t, s, "strange@example.com")
}

func TestRenderOtherContactsMD_Empty(t *testing.T) {
	md, ok := renderOtherContactsMD([]byte(`{"otherContacts":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No other contacts._")
}

func TestRenderOtherContactsMD_WrongShape(t *testing.T) {
	_, ok := renderOtherContactsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestPrimaryName(t *testing.T) {
	// displayName wins.
	assert.Equal(t, "Jane Doe", primaryName([]rawName{{DisplayName: "Jane Doe", GivenName: "J", FamilyName: "D"}}))
	// Falls back to given+family.
	assert.Equal(t, "Jane Doe", primaryName([]rawName{{GivenName: "Jane", FamilyName: "Doe"}}))
	// Includes middle name.
	assert.Equal(t, "Jane Q Doe", primaryName([]rawName{{GivenName: "Jane", MiddleName: "Q", FamilyName: "Doe"}}))
	// Empty.
	assert.Equal(t, "", primaryName(nil))
	assert.Equal(t, "", primaryName([]rawName{{}}))
}

func TestPrimaryOrg(t *testing.T) {
	assert.Equal(t, "VP at Acme", primaryOrg([]rawOrg{{Name: "Acme", Title: "VP"}}))
	assert.Equal(t, "Acme", primaryOrg([]rawOrg{{Name: "Acme"}}))
	assert.Equal(t, "VP", primaryOrg([]rawOrg{{Title: "VP"}}))
	assert.Equal(t, "", primaryOrg(nil))
}

func TestJoinNonEmpty(t *testing.T) {
	assert.Equal(t, "a, b, c", joinNonEmpty([]string{"a", "b", "c"}, ", "))
	assert.Equal(t, "a, c", joinNonEmpty([]string{"a", "", "c"}, ", "))
	assert.Equal(t, "", joinNonEmpty([]string{"", ""}, ", "))
}

func TestPipeSafe(t *testing.T) {
	assert.Equal(t, "a b", pipeSafe("a\nb"))
	assert.Equal(t, "a\\|b", pipeSafe("a|b"))
}
