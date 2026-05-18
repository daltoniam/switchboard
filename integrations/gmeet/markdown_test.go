package gmeet

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ──────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	// Every tool that returns document-style data must have a renderer.
	// gmeet_end_active_conference returns just a status envelope, so no
	// renderer.
	wantRendered := map[mcp.ToolName]bool{
		"gmeet_create_space":            true,
		"gmeet_get_space":               true,
		"gmeet_update_space":            true,
		"gmeet_get_conference_record":   true,
		"gmeet_list_conference_records": true,
		"gmeet_list_participants":       true,
		"gmeet_list_recordings":         true,
		"gmeet_list_transcripts":        true,
		"gmeet_list_transcript_entries": true,
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
	g := &gmeet{}
	_, ok := g.RenderMarkdown("gmeet_end_active_conference", []byte(`{}`))
	assert.False(t, ok)
}

// ── Space ───────────────────────────────────────────────────────────

func TestRenderSpaceMD_Basic(t *testing.T) {
	in := []byte(`{
        "name":"spaces/abc",
        "meetingUri":"https://meet.google.com/abc-defg-hij",
        "meetingCode":"abc-defg-hij",
        "config":{
            "accessType":"TRUSTED",
            "entryPointAccess":"ALL",
            "moderation":"ON",
            "moderationRestrictions":{
                "chatRestriction":"HOSTS_ONLY",
                "reactionRestriction":"NO_RESTRICTION",
                "presentRestriction":"HOSTS_ONLY",
                "defaultJoinAsViewerType":"OFF"
            }
        },
        "activeConference":{"conferenceRecord":"conferenceRecords/cr-1"}
    }`)
	md, ok := renderSpaceMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Meet Space — abc-defg-hij")
	assert.Contains(t, s, "name: spaces/abc")
	assert.Contains(t, s, "join_url: https://meet.google.com/abc-defg-hij")
	assert.Contains(t, s, "meeting_code: abc-defg-hij")
	assert.Contains(t, s, "active_conference: conferenceRecords/cr-1")
	assert.Contains(t, s, "## Configuration")
	assert.Contains(t, s, "access_type: TRUSTED")
	assert.Contains(t, s, "moderation: ON")
	assert.Contains(t, s, "chat_restriction: HOSTS_ONLY")
}

func TestRenderSpaceMD_FallbackTitle(t *testing.T) {
	md, ok := renderSpaceMD([]byte(`{"name":"spaces/xyz"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "# Meet Space — spaces/xyz")
}

func TestRenderSpaceMD_NoName(t *testing.T) {
	_, ok := renderSpaceMD([]byte(`{"meetingUri":"x"}`))
	assert.False(t, ok)
}

func TestRenderSpaceMD_InvalidJSON(t *testing.T) {
	_, ok := renderSpaceMD([]byte(`not json`))
	assert.False(t, ok)
}

// ── Conference record ──────────────────────────────────────────────

func TestRenderConferenceRecordMD_Basic(t *testing.T) {
	in := []byte(`{
        "name":"conferenceRecords/cr-1",
        "space":"spaces/abc",
        "startTime":"2024-01-01T10:00:00Z",
        "endTime":"2024-01-01T11:00:00Z",
        "expireTime":"2024-02-01T00:00:00Z"
    }`)
	md, ok := renderConferenceRecordMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Conference Record")
	assert.Contains(t, s, "name: conferenceRecords/cr-1")
	assert.Contains(t, s, "space: spaces/abc")
	assert.Contains(t, s, "start_time: 2024-01-01T10:00:00Z")
	assert.Contains(t, s, "end_time: 2024-01-01T11:00:00Z")
}

func TestRenderConferenceRecordMD_NoName(t *testing.T) {
	_, ok := renderConferenceRecordMD([]byte(`{"space":"x"}`))
	assert.False(t, ok)
}

// ── Conference records (list) ──────────────────────────────────────

func TestRenderConferenceRecordsMD_Basic(t *testing.T) {
	in := []byte(`{
        "conferenceRecords":[
            {"name":"conferenceRecords/cr-1","space":"spaces/abc","startTime":"2024-01-01T10:00:00Z","endTime":"2024-01-01T11:00:00Z"},
            {"name":"conferenceRecords/cr-2","space":"spaces/xyz"}
        ],
        "nextPageToken":"tok"
    }`)
	md, ok := renderConferenceRecordsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Conference Records")
	assert.Contains(t, s, "conferenceRecords/cr-1")
	assert.Contains(t, s, "spaces/abc")
	assert.Contains(t, s, "next_page_token: tok")
}

func TestRenderConferenceRecordsMD_Empty(t *testing.T) {
	md, ok := renderConferenceRecordsMD([]byte(`{"conferenceRecords":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No conference records._")
}

func TestRenderConferenceRecordsMD_WrongShape(t *testing.T) {
	_, ok := renderConferenceRecordsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Participants ────────────────────────────────────────────────────

func TestRenderParticipantsMD_Basic(t *testing.T) {
	in := []byte(`{
        "participants":[
            {"name":"conferenceRecords/cr-1/participants/p1","signedinUser":{"user":"users/u1","displayName":"Alice"},"earliestStartTime":"2024-01-01T10:00:00Z","latestEndTime":"2024-01-01T11:00:00Z"},
            {"name":"conferenceRecords/cr-1/participants/p2","anonymousUser":{"displayName":"Guest"}},
            {"name":"conferenceRecords/cr-1/participants/p3","phoneUser":{"displayName":"+1-555-0100"}}
        ],
        "totalSize":3
    }`)
	md, ok := renderParticipantsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Participants")
	assert.Contains(t, s, "Alice")
	assert.Contains(t, s, "users/u1")
	assert.Contains(t, s, "signed-in")
	assert.Contains(t, s, "Guest")
	assert.Contains(t, s, "anonymous")
	assert.Contains(t, s, "phone")
	assert.Contains(t, s, "total_size: 3")
}

func TestRenderParticipantsMD_Empty(t *testing.T) {
	md, ok := renderParticipantsMD([]byte(`{"participants":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No participants._")
}

func TestRenderParticipantsMD_WrongShape(t *testing.T) {
	_, ok := renderParticipantsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

func TestParticipantIdentity(t *testing.T) {
	display, identity, kind := participantIdentity(rawParticipant{
		SignedinUser: rawSignedInUser{User: "users/u1", DisplayName: "Alice"},
	})
	assert.Equal(t, "Alice", display)
	assert.Equal(t, "users/u1", identity)
	assert.Equal(t, "signed-in", kind)

	display, identity, kind = participantIdentity(rawParticipant{
		AnonymousUser: rawAnonymousUser{DisplayName: "Guest"},
	})
	assert.Equal(t, "Guest", display)
	assert.Equal(t, "", identity)
	assert.Equal(t, "anonymous", kind)

	display, _, kind = participantIdentity(rawParticipant{
		PhoneUser: rawAnonymousUser{DisplayName: "+1-555-0100"},
	})
	assert.Equal(t, "+1-555-0100", display)
	assert.Equal(t, "phone", kind)

	display, identity, kind = participantIdentity(rawParticipant{})
	assert.Equal(t, "", display)
	assert.Equal(t, "", identity)
	assert.Equal(t, "", kind)
}

// ── Recordings ──────────────────────────────────────────────────────

func TestRenderRecordingsMD_Basic(t *testing.T) {
	in := []byte(`{
        "recordings":[
            {"name":"conferenceRecords/cr-1/recordings/r1","state":"FILE_GENERATED","driveDestination":{"file":"1abc..."},"startTime":"2024-01-01T10:00:00Z","endTime":"2024-01-01T11:00:00Z"}
        ]
    }`)
	md, ok := renderRecordingsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Recordings")
	assert.Contains(t, s, "FILE_GENERATED")
	assert.Contains(t, s, "1abc...")
}

func TestRenderRecordingsMD_Empty(t *testing.T) {
	md, ok := renderRecordingsMD([]byte(`{"recordings":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No recordings._")
}

func TestRenderRecordingsMD_WrongShape(t *testing.T) {
	_, ok := renderRecordingsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Transcripts ─────────────────────────────────────────────────────

func TestRenderTranscriptsMD_Basic(t *testing.T) {
	in := []byte(`{
        "transcripts":[
            {"name":"conferenceRecords/cr-1/transcripts/t1","state":"FILE_GENERATED","docsDestination":{"document":"docs/doc-1"},"startTime":"2024-01-01T10:00:00Z","endTime":"2024-01-01T11:00:00Z"}
        ]
    }`)
	md, ok := renderTranscriptsMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Transcripts")
	assert.Contains(t, s, "FILE_GENERATED")
	assert.Contains(t, s, "docs/doc-1")
}

func TestRenderTranscriptsMD_Empty(t *testing.T) {
	md, ok := renderTranscriptsMD([]byte(`{"transcripts":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No transcripts._")
}

func TestRenderTranscriptsMD_WrongShape(t *testing.T) {
	_, ok := renderTranscriptsMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Transcript entries ──────────────────────────────────────────────

func TestRenderTranscriptEntriesMD_Basic(t *testing.T) {
	in := []byte(`{
        "transcriptEntries":[
            {"name":"conferenceRecords/cr-1/transcripts/t1/entries/e1","participant":"conferenceRecords/cr-1/participants/p1","text":"Hello everyone.","languageCode":"en-US","startTime":"2024-01-01T10:00:00Z","endTime":"2024-01-01T10:00:03Z"},
            {"name":"conferenceRecords/cr-1/transcripts/t1/entries/e2","participant":"conferenceRecords/cr-1/participants/p2","text":"Good morning.","startTime":"2024-01-01T10:00:04Z","endTime":"2024-01-01T10:00:06Z"}
        ],
        "nextPageToken":"tok"
    }`)
	md, ok := renderTranscriptEntriesMD(in)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Transcript Entries")
	assert.Contains(t, s, "Hello everyone.")
	assert.Contains(t, s, "Good morning.")
	assert.Contains(t, s, "conferenceRecords/cr-1/participants/p1")
	assert.Contains(t, s, "next_page_token: tok")
}

func TestRenderTranscriptEntriesMD_Empty(t *testing.T) {
	md, ok := renderTranscriptEntriesMD([]byte(`{"transcriptEntries":[]}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "_No transcript entries._")
}

func TestRenderTranscriptEntriesMD_MissingParticipant(t *testing.T) {
	in := []byte(`{"transcriptEntries":[{"name":"x","text":"hello"}]}`)
	md, ok := renderTranscriptEntriesMD(in)
	require.True(t, ok)
	assert.Contains(t, string(md), "(unknown)")
}

func TestRenderTranscriptEntriesMD_WrongShape(t *testing.T) {
	_, ok := renderTranscriptEntriesMD([]byte(`{"unrelated":true}`))
	assert.False(t, ok)
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestPipeSafe(t *testing.T) {
	assert.Equal(t, "a b", pipeSafe("a\nb"))
	assert.Equal(t, "a\\|b", pipeSafe("a|b"))
}

func TestEscapePath(t *testing.T) {
	// Slash structure is preserved; only segments are escaped.
	assert.Equal(t, "spaces/abc", escapePath("spaces/abc"))
	assert.Equal(t, "spaces/abc%20def", escapePath("spaces/abc def"))
	assert.Equal(t, "conferenceRecords/cr-1/transcripts/t-1", escapePath("conferenceRecords/cr-1/transcripts/t-1"))
}
