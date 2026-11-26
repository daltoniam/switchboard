package ashby

import (
	"encoding/json"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// rawReadTools return small or opaque payloads that compaction would only
// damage: report rows, a download URL, key scopes, sender addresses.
var rawReadTools = map[mcp.ToolName]string{
	"ashby_get_api_key_info":         "a few fields describing the key itself",
	"ashby_generate_report":          "report rows are positional; dropping columns corrupts them",
	"ashby_get_report":               "report rows are positional; dropping columns corrupts them",
	"ashby_get_file_url":             "a URL and a category",
	"ashby_get_notetaker_transcript": "participants and a download URL",
	"ashby_list_email_senders":       "three fields per sender",
}

func TestFieldCompactionSpecs_Parse(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	require.NotEmpty(t, fieldCompactionSpecs)
	assert.Len(t, fieldCompactionSpecs, len(sf.Tools), "the loader dropped an entry")
	_, err := compact.Load(compactYAML, compact.Options{Strict: true})
	require.NoError(t, err)
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := nativeDispatch[name]
		assert.True(t, ok, "compaction spec for %s has no native tool", name)
	}
}

func TestFieldCompactionSpecs_ReadToolsOnly(t *testing.T) {
	for _, ep := range nativeEndpoints {
		_, hasSpec := fieldCompactionSpecs[ep.tool]
		_, raw := rawReadTools[ep.tool]
		switch {
		case ep.access != mcp.AccessRead:
			assert.False(t, hasSpec, "mutation tool %s must return its full confirmation", ep.tool)
		case raw:
			assert.False(t, hasSpec, "%s is listed as raw but has a spec", ep.tool)
		default:
			assert.True(t, hasSpec, "read tool %s has no compaction spec", ep.tool)
		}
	}
	for name := range rawReadTools {
		ep, ok := nativeDispatch[name]
		require.True(t, ok, "raw tool %s does not exist", name)
		assert.Equal(t, mcp.AccessRead, ep.access)
	}
}

// Every spec root must be a key the handler emits.
func TestFieldCompactionSpecs_RootsMatchHandlerOutput(t *testing.T) {
	roots := map[string]bool{"results": true, "moreDataAvailable": true, "nextCursor": true, "syncToken": true}
	for name, fields := range fieldCompactionSpecs {
		for _, field := range fields {
			root, _, _ := strings.Cut(strings.TrimPrefix(field.Spec(), "-"), ".")
			root, _, _ = strings.Cut(root, "[")
			assert.True(t, roots[root], "%s: spec %q does not start at a handler output key", name, field.Spec())
		}
	}
}

// Inside an array, a nested path keeps only its leaf name, so two paths with
// the same leaf silently overwrite each other in every compacted row.
func TestFieldCompactionSpecs_NoLeafCollisionsInArrays(t *testing.T) {
	for name, fields := range fieldCompactionSpecs {
		seen := map[string]string{}
		for _, field := range fields {
			array, rest, ok := strings.Cut(field.Spec(), "[].")
			if !ok || strings.Contains(rest, "[") {
				continue
			}
			leaf := rest[strings.LastIndex(rest, ".")+1:]
			key := array + "/" + leaf
			if previous, dup := seen[key]; dup {
				t.Errorf("%s: %q and %q both compact to %s[].%s", name, previous, field.Spec(), array, leaf)
			}
			seen[key] = field.Spec()
		}
	}
}

func TestMaxBytes_NoCapConfigured(t *testing.T) {
	native := configuredAPI(t, "https://api.example.test", fixtureAPIKey).(mcp.ToolMaxBytesIntegration)
	for name := range fieldCompactionSpecs {
		_, ok := native.MaxBytes(name)
		assert.False(t, ok, "%s declares a max_bytes cap; update this test deliberately", name)
	}
}

const candidateRecord = `{
	"id": "cand-1", "name": "Jordan Lee", "createdAt": "2026-01-02T03:04:05.000Z", "updatedAt": "2026-02-03T04:05:06.000Z",
	"doNotContact": false,
	"primaryEmailAddress": {"value": "jordan@example.com", "type": "Personal", "isPrimary": true},
	"emailAddresses": [{"value": "jordan@example.com", "type": "Personal", "isPrimary": true}, {"value": "jl@work.example", "type": "Work", "isPrimary": false}],
	"primaryPhoneNumber": {"value": "+1 555 0100", "type": "Mobile", "isPrimary": true},
	"phoneNumbers": [{"value": "+1 555 0100", "type": "Mobile", "isPrimary": true}],
	"socialLinks": [{"type": "LinkedIn", "url": "https://linkedin.example/jordan"}],
	"tags": [{"id": "tag-1", "title": "Referral", "isArchived": false}],
	"position": "Staff Engineer", "company": "Acme", "school": "State University",
	"applicationIds": ["app-1"],
	"resumeFileHandle": {"id": "file-1", "name": "resume.pdf", "handle": "secret-handle"},
	"fileHandles": [{"id": "file-1", "name": "resume.pdf", "handle": "secret-handle"}],
	"customFields": [{"id": "cf-1", "title": "Salary expectation", "value": "200000"}],
	"profileUrl": "https://app.ashbyhq.com/candidates/cand-1",
	"source": {"id": "src-1", "title": "Referral", "isArchived": false, "sourceType": {"id": "st-1", "title": "Inbound"}},
	"location": {"id": "loc-1", "locationSummary": "Austin, TX", "locationComponents": []},
	"timezone": "America/Chicago"
}`

const applicationRecord = `{
	"id": "app-1", "status": "Active", "createdAt": "2026-01-02T03:04:05.000Z", "updatedAt": "2026-02-03T04:05:06.000Z",
	"customFields": [{"id": "cf-1", "title": "Comp", "value": "secret"}],
	"candidate": {"id": "cand-1", "name": "Jordan Lee", "primaryEmailAddress": {"value": "jordan@example.com"}, "primaryPhoneNumber": {"value": "+1 555 0100"}},
	"currentInterviewStage": {"id": "stage-2", "title": "Onsite", "type": "Active", "orderInInterviewPlan": 3, "interviewPlanId": "plan-1"},
	"source": {"id": "src-1", "title": "Referral"},
	"job": {"id": "job-1", "title": "Staff Engineer", "locationId": "loc-1", "departmentId": "dep-1", "brandId": "brand-1"},
	"hiringTeam": [{"userId": "user-1", "firstName": "Ada", "lastName": "Recruiter", "email": "ada@example.com", "role": "Recruiter"}],
	"submitterClientIp": "203.0.113.9", "submitterUserAgent": "Mozilla/5.0"
}`

func compactFixture(t *testing.T, tool mcp.ToolName, response string) map[string]any {
	t.Helper()
	fields, ok := configuredAPI(t, "https://api.example.test", fixtureAPIKey).(mcp.FieldCompactionIntegration).CompactSpec(tool)
	require.True(t, ok, "%s has no spec", tool)
	var raw any
	require.NoError(t, json.Unmarshal([]byte(response), &raw))
	out, ok := mcp.CompactAny(raw, fields).(map[string]any)
	require.True(t, ok)
	return out
}

func TestCompaction_Shape(t *testing.T) {
	tests := []struct {
		name     string
		tool     mcp.ToolName
		response string
		keep     []string
		drop     []string
	}{
		{
			name:     "candidate list keeps identifiers and drops contact details",
			tool:     "ashby_list_candidates",
			response: `{"results":[` + candidateRecord + `],"moreDataAvailable":true,"nextCursor":"c2","syncToken":null}`,
			keep:     []string{`"id":"cand-1"`, "Jordan Lee", "jordan@example.com", "app-1", "2026-01-02T03:04:05.000Z", `"moreDataAvailable":true`, `"nextCursor":"c2"`},
			drop:     []string{"+1 555 0100", "linkedin.example", "secret-handle", "Salary expectation", "jl@work.example", "Austin, TX", "America/Chicago"},
		},
		{
			name:     "candidate profile keeps background but not files or custom fields",
			tool:     "ashby_get_candidate",
			response: `{"results":` + candidateRecord + `}`,
			keep:     []string{`"id":"cand-1"`, "Staff Engineer", "Acme", "State University", "Referral", "app-1", "https://app.ashbyhq.com/candidates/cand-1"},
			drop:     []string{"+1 555 0100", "secret-handle", "Salary expectation", "linkedin.example"},
		},
		{
			name:     "application list keeps candidate, job, and stage links",
			tool:     "ashby_list_applications",
			response: `{"results":[` + applicationRecord + `],"moreDataAvailable":false,"nextCursor":null,"syncToken":"s9"}`,
			keep:     []string{`"id":"app-1"`, "Active", "Jordan Lee", `"job":{`, "job-1", "Staff Engineer", `"currentInterviewStage":{`, "stage-2", "Onsite", `"syncToken":"s9"`},
			drop:     []string{"jordan@example.com", "+1 555 0100", "203.0.113.9", "Mozilla/5.0", "secret", "ada@example.com"},
		},
		{
			name:     "job list keeps routing fields",
			tool:     "ashby_list_jobs",
			response: `{"results":[{"id":"job-1","title":"Staff Engineer","status":"Open","employmentType":"FullTime","confidential":false,"departmentId":"dep-1","locationId":"loc-1","brandId":"b-1","createdAt":"2026-01-01T00:00:00.000Z","updatedAt":"2026-01-02T00:00:00.000Z","compensation":{"summary":"$200k"},"customFields":[{"title":"Budget code","value":"BC-7"}],"hiringTeam":[{"email":"ada@example.com"}]}],"moreDataAvailable":false,"nextCursor":null,"syncToken":null}`,
			keep:     []string{"job-1", "Staff Engineer", "Open", "FullTime", "dep-1", "loc-1"},
			drop:     []string{"$200k", "BC-7", "ada@example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := compactFixture(t, tt.tool, tt.response)
			data, err := json.Marshal(out)
			require.NoError(t, err)
			text := string(data)
			for _, want := range tt.keep {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range tt.drop {
				assert.NotContains(t, text, unwanted)
			}
		})
	}
}

func TestCompaction_CandidateProfileKeepsIDAtItsPath(t *testing.T) {
	out := compactFixture(t, "ashby_get_candidate", `{"results":`+candidateRecord+`}`)
	results, ok := out["results"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cand-1", results["id"])
	assert.Equal(t, "Jordan Lee", results["name"])
}
