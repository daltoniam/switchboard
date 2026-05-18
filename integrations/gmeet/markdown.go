package gmeet

import (
	"encoding/json"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gmeet_create_space":            renderSpaceMD,
	"gmeet_get_space":               renderSpaceMD,
	"gmeet_update_space":            renderSpaceMD,
	"gmeet_get_conference_record":   renderConferenceRecordMD,
	"gmeet_list_conference_records": renderConferenceRecordsMD,
	"gmeet_list_participants":       renderParticipantsMD,
	"gmeet_list_recordings":         renderRecordingsMD,
	"gmeet_list_transcripts":        renderTranscriptsMD,
	"gmeet_list_transcript_entries": renderTranscriptEntriesMD,
}

func (g *gmeet) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawModerationRestrictions struct {
	ChatRestriction         string `json:"chatRestriction"`
	ReactionRestriction     string `json:"reactionRestriction"`
	PresentRestriction      string `json:"presentRestriction"`
	DefaultJoinAsViewerType string `json:"defaultJoinAsViewerType"`
}

type rawSpaceConfig struct {
	AccessType             string                    `json:"accessType"`
	EntryPointAccess       string                    `json:"entryPointAccess"`
	Moderation             string                    `json:"moderation"`
	ModerationRestrictions rawModerationRestrictions `json:"moderationRestrictions"`
}

type rawActiveConference struct {
	ConferenceRecord string `json:"conferenceRecord"`
}

type rawSpace struct {
	Name             string              `json:"name"`
	MeetingURI       string              `json:"meetingUri"`
	MeetingCode      string              `json:"meetingCode"`
	Config           rawSpaceConfig      `json:"config"`
	ActiveConference rawActiveConference `json:"activeConference"`
}

type rawConferenceRecord struct {
	Name       string `json:"name"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	ExpireTime string `json:"expireTime"`
	Space      string `json:"space"`
}

type rawConferenceRecordsPage struct {
	ConferenceRecords []rawConferenceRecord `json:"conferenceRecords"`
	NextPageToken     string                `json:"nextPageToken"`
}

type rawSignedInUser struct {
	User        string `json:"user"`
	DisplayName string `json:"displayName"`
}

type rawAnonymousUser struct {
	DisplayName string `json:"displayName"`
}

type rawParticipant struct {
	Name              string           `json:"name"`
	EarliestStartTime string           `json:"earliestStartTime"`
	LatestEndTime     string           `json:"latestEndTime"`
	SignedinUser      rawSignedInUser  `json:"signedinUser"`
	AnonymousUser     rawAnonymousUser `json:"anonymousUser"`
	PhoneUser         rawAnonymousUser `json:"phoneUser"`
}

type rawParticipantsPage struct {
	Participants  []rawParticipant `json:"participants"`
	NextPageToken string           `json:"nextPageToken"`
	TotalSize     int              `json:"totalSize"`
}

type rawDriveDestination struct {
	File      string `json:"file"`
	ExportURI string `json:"exportUri"`
}

type rawDocsDestination struct {
	Document  string `json:"document"`
	ExportURI string `json:"exportUri"`
}

type rawRecording struct {
	Name             string              `json:"name"`
	State            string              `json:"state"`
	StartTime        string              `json:"startTime"`
	EndTime          string              `json:"endTime"`
	DriveDestination rawDriveDestination `json:"driveDestination"`
}

type rawRecordingsPage struct {
	Recordings    []rawRecording `json:"recordings"`
	NextPageToken string         `json:"nextPageToken"`
}

type rawTranscript struct {
	Name            string             `json:"name"`
	State           string             `json:"state"`
	StartTime       string             `json:"startTime"`
	EndTime         string             `json:"endTime"`
	DocsDestination rawDocsDestination `json:"docsDestination"`
}

type rawTranscriptsPage struct {
	Transcripts   []rawTranscript `json:"transcripts"`
	NextPageToken string          `json:"nextPageToken"`
}

type rawTranscriptEntry struct {
	Name         string `json:"name"`
	Participant  string `json:"participant"`
	Text         string `json:"text"`
	LanguageCode string `json:"languageCode"`
	StartTime    string `json:"startTime"`
	EndTime      string `json:"endTime"`
}

type rawTranscriptEntriesPage struct {
	TranscriptEntries []rawTranscriptEntry `json:"transcriptEntries"`
	NextPageToken     string               `json:"nextPageToken"`
}

// ── Space (create/get/update) ───────────────────────────────────────

func renderSpaceMD(data []byte) (markdown.Markdown, bool) {
	var s rawSpace
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false
	}
	if s.Name == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	title := s.MeetingCode
	if title == "" {
		title = s.Name
	}
	b.Heading(1, "Meet Space — "+title)

	var attrs []string
	attrs = append(attrs, "name: "+s.Name)
	if s.MeetingURI != "" {
		attrs = append(attrs, "join_url: "+s.MeetingURI)
	}
	if s.MeetingCode != "" {
		attrs = append(attrs, "meeting_code: "+s.MeetingCode)
	}
	if s.ActiveConference.ConferenceRecord != "" {
		attrs = append(attrs, "active_conference: "+s.ActiveConference.ConferenceRecord)
	}
	b.Attribution(attrs...)

	hasConfig := s.Config.AccessType != "" || s.Config.EntryPointAccess != "" || s.Config.Moderation != ""
	if hasConfig {
		b.BlankLine()
		b.Heading(2, "Configuration")
		var sb strings.Builder
		if s.Config.AccessType != "" {
			sb.WriteString("- access_type: ")
			sb.WriteString(s.Config.AccessType)
			sb.WriteString("\n")
		}
		if s.Config.EntryPointAccess != "" {
			sb.WriteString("- entry_point_access: ")
			sb.WriteString(s.Config.EntryPointAccess)
			sb.WriteString("\n")
		}
		if s.Config.Moderation != "" {
			sb.WriteString("- moderation: ")
			sb.WriteString(s.Config.Moderation)
			sb.WriteString("\n")
		}
		mr := s.Config.ModerationRestrictions
		if mr.ChatRestriction != "" {
			sb.WriteString("- chat_restriction: ")
			sb.WriteString(mr.ChatRestriction)
			sb.WriteString("\n")
		}
		if mr.ReactionRestriction != "" {
			sb.WriteString("- reaction_restriction: ")
			sb.WriteString(mr.ReactionRestriction)
			sb.WriteString("\n")
		}
		if mr.PresentRestriction != "" {
			sb.WriteString("- present_restriction: ")
			sb.WriteString(mr.PresentRestriction)
			sb.WriteString("\n")
		}
		if mr.DefaultJoinAsViewerType != "" {
			sb.WriteString("- default_join_as_viewer_type: ")
			sb.WriteString(mr.DefaultJoinAsViewerType)
			sb.WriteString("\n")
		}
		b.Raw(sb.String())
	}

	return b.Build(), true
}

// ── Conference record (single) ──────────────────────────────────────

func renderConferenceRecordMD(data []byte) (markdown.Markdown, bool) {
	var cr rawConferenceRecord
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", false
	}
	if cr.Name == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Conference Record")

	var attrs []string
	attrs = append(attrs, "name: "+cr.Name)
	if cr.Space != "" {
		attrs = append(attrs, "space: "+cr.Space)
	}
	if cr.StartTime != "" {
		attrs = append(attrs, "start_time: "+cr.StartTime)
	}
	if cr.EndTime != "" {
		attrs = append(attrs, "end_time: "+cr.EndTime)
	}
	if cr.ExpireTime != "" {
		attrs = append(attrs, "expire_time: "+cr.ExpireTime)
	}
	b.Attribution(attrs...)
	return b.Build(), true
}

// ── Conference records (list) ───────────────────────────────────────

func renderConferenceRecordsMD(data []byte) (markdown.Markdown, bool) {
	var page rawConferenceRecordsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasRecords := probe["conferenceRecords"]; !hasRecords {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Conference Records")
	if len(page.ConferenceRecords) == 0 {
		b.BlankLine()
		b.Raw("_No conference records._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Name", "Space", "Start", "End"}}
	for _, cr := range page.ConferenceRecords {
		rows = append(rows, []string{
			pipeSafe(cr.Name),
			pipeSafe(cr.Space),
			pipeSafe(cr.StartTime),
			pipeSafe(cr.EndTime),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Participants ────────────────────────────────────────────────────

func renderParticipantsMD(data []byte) (markdown.Markdown, bool) {
	var page rawParticipantsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasParticipants := probe["participants"]; !hasParticipants {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Participants")
	if len(page.Participants) == 0 {
		b.BlankLine()
		b.Raw("_No participants._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Display Name", "Identity", "Type", "Earliest Start", "Latest End", "Name"}}
	for _, p := range page.Participants {
		display, identity, kind := participantIdentity(p)
		rows = append(rows, []string{
			pipeSafe(display),
			pipeSafe(identity),
			pipeSafe(kind),
			pipeSafe(p.EarliestStartTime),
			pipeSafe(p.LatestEndTime),
			pipeSafe(p.Name),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	var notes []string
	if page.TotalSize > 0 {
		notes = append(notes, "total_size: "+itoa(page.TotalSize))
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

// participantIdentity returns (displayName, identity, kind) where kind is
// "signed-in", "anonymous", or "phone". For signed-in users the identity
// is the user resource (e.g. "users/123"), otherwise it's empty.
func participantIdentity(p rawParticipant) (string, string, string) {
	switch {
	case p.SignedinUser.DisplayName != "" || p.SignedinUser.User != "":
		return p.SignedinUser.DisplayName, p.SignedinUser.User, "signed-in"
	case p.AnonymousUser.DisplayName != "":
		return p.AnonymousUser.DisplayName, "", "anonymous"
	case p.PhoneUser.DisplayName != "":
		return p.PhoneUser.DisplayName, "", "phone"
	}
	return "", "", ""
}

// ── Recordings ──────────────────────────────────────────────────────

func renderRecordingsMD(data []byte) (markdown.Markdown, bool) {
	var page rawRecordingsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasRecordings := probe["recordings"]; !hasRecordings {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Recordings")
	if len(page.Recordings) == 0 {
		b.BlankLine()
		b.Raw("_No recordings._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Name", "State", "Drive File", "Start", "End"}}
	for _, r := range page.Recordings {
		rows = append(rows, []string{
			pipeSafe(r.Name),
			pipeSafe(r.State),
			pipeSafe(r.DriveDestination.File),
			pipeSafe(r.StartTime),
			pipeSafe(r.EndTime),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Transcripts ─────────────────────────────────────────────────────

func renderTranscriptsMD(data []byte) (markdown.Markdown, bool) {
	var page rawTranscriptsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasTranscripts := probe["transcripts"]; !hasTranscripts {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Transcripts")
	if len(page.Transcripts) == 0 {
		b.BlankLine()
		b.Raw("_No transcripts._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	rows := [][]string{{"Name", "State", "Document", "Start", "End"}}
	for _, t := range page.Transcripts {
		rows = append(rows, []string{
			pipeSafe(t.Name),
			pipeSafe(t.State),
			pipeSafe(t.DocsDestination.Document),
			pipeSafe(t.StartTime),
			pipeSafe(t.EndTime),
		})
	}
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.BlankLine()
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Transcript entries (captions) ───────────────────────────────────

func renderTranscriptEntriesMD(data []byte) (markdown.Markdown, bool) {
	var page rawTranscriptEntriesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return "", false
	}
	if _, hasEntries := probe["transcriptEntries"]; !hasEntries {
		if _, hasToken := probe["nextPageToken"]; !hasToken {
			return "", false
		}
	}

	b := markdown.NewBuilder()
	b.Heading(1, "Transcript Entries")
	if len(page.TranscriptEntries) == 0 {
		b.BlankLine()
		b.Raw("_No transcript entries._\n")
		return b.Build(), true
	}

	var sb strings.Builder
	for _, e := range page.TranscriptEntries {
		sb.WriteString("**")
		if e.Participant != "" {
			sb.WriteString(e.Participant)
		} else {
			sb.WriteString("(unknown)")
		}
		sb.WriteString("**")
		if e.StartTime != "" {
			sb.WriteString(" _(")
			sb.WriteString(e.StartTime)
			if e.EndTime != "" {
				sb.WriteString(" – ")
				sb.WriteString(e.EndTime)
			}
			sb.WriteString(")_")
		}
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(e.Text))
		sb.WriteString("\n\n")
	}
	b.Raw(sb.String())

	if page.NextPageToken != "" {
		b.Attribution("next_page_token: " + page.NextPageToken)
	}
	return b.Build(), true
}

// ── Helpers ─────────────────────────────────────────────────────────

// pipeSafe escapes newlines and pipes so a cell stays on one row.
func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// itoa wraps strconv.Itoa for terse formatting in attribution lines.
func itoa(n int) string { return strconv.Itoa(n) }
