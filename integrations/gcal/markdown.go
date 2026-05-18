package gcal

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types ────────────────────────────────────────────

type renderedEvent struct {
	ID          string
	Summary     string
	Status      string
	Description string
	Location    string
	HTMLLink    string
	Start       string
	End         string
	Recurring   string
	Organizer   string
	Attendees   []renderedAttendee
	MeetLink    string
}

type renderedAttendee struct {
	Email    string
	Name     string
	Response string
	Optional bool
}

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gcal_get_event": renderEventMD,
}

func (g *gcal) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawEvent struct {
	ID               string        `json:"id"`
	Summary          string        `json:"summary"`
	Description      string        `json:"description"`
	Location         string        `json:"location"`
	Status           string        `json:"status"`
	HTMLLink         string        `json:"htmlLink"`
	HangoutLink      string        `json:"hangoutLink"`
	Start            rawEventTime  `json:"start"`
	End              rawEventTime  `json:"end"`
	RecurringEventID string        `json:"recurringEventId"`
	Organizer        rawPerson     `json:"organizer"`
	Attendees        []rawAttendee `json:"attendees"`
	ConferenceData   rawConference `json:"conferenceData"`
}

type rawEventTime struct {
	Date     string `json:"date"`
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type rawPerson struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type rawAttendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	ResponseStatus string `json:"responseStatus"`
	Optional       bool   `json:"optional"`
}

type rawConference struct {
	EntryPoints []rawEntryPoint `json:"entryPoints"`
}

type rawEntryPoint struct {
	URI            string `json:"uri"`
	EntryPointType string `json:"entryPointType"`
}

// ── Rendering ───────────────────────────────────────────────────────

func renderEventMD(data []byte) (markdown.Markdown, bool) {
	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	ev := renderedEvent{
		ID:          raw.ID,
		Summary:     raw.Summary,
		Status:      raw.Status,
		Description: raw.Description,
		Location:    raw.Location,
		HTMLLink:    raw.HTMLLink,
		Start:       formatEventTime(raw.Start),
		End:         formatEventTime(raw.End),
		Recurring:   raw.RecurringEventID,
		Organizer:   formatPerson(raw.Organizer),
		MeetLink:    extractMeetLink(raw),
	}
	for _, a := range raw.Attendees {
		ev.Attendees = append(ev.Attendees, renderedAttendee{
			Email:    a.Email,
			Name:     a.DisplayName,
			Response: a.ResponseStatus,
			Optional: a.Optional,
		})
	}
	return eventToMarkdown(ev), true
}

func formatEventTime(t rawEventTime) string {
	if t.DateTime != "" {
		if t.TimeZone != "" {
			return fmt.Sprintf("%s (%s)", t.DateTime, t.TimeZone)
		}
		return t.DateTime
	}
	if t.Date != "" {
		return t.Date + " (all-day)"
	}
	return ""
}

func formatPerson(p rawPerson) string {
	if p.DisplayName != "" && p.Email != "" {
		return fmt.Sprintf("%s <%s>", p.DisplayName, p.Email)
	}
	if p.Email != "" {
		return p.Email
	}
	return p.DisplayName
}

func extractMeetLink(raw rawEvent) string {
	if raw.HangoutLink != "" {
		return raw.HangoutLink
	}
	for _, ep := range raw.ConferenceData.EntryPoints {
		if ep.EntryPointType == "video" && ep.URI != "" {
			return ep.URI
		}
	}
	return ""
}

func eventToMarkdown(ev renderedEvent) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("gcal", "event_id", ev.ID, "status", ev.Status)

	title := ev.Summary
	if title == "" {
		title = "(no title)"
	}
	b.Heading(1, title)

	if ev.Start != "" || ev.End != "" {
		b.Attribution(fmt.Sprintf("When: %s → %s", ev.Start, ev.End))
	}
	if ev.Location != "" {
		b.Attribution("Where: " + ev.Location)
	}
	if ev.Organizer != "" {
		b.Attribution("Organizer: " + ev.Organizer)
	}
	if ev.MeetLink != "" {
		b.Attribution("Meet: " + ev.MeetLink)
	}
	if ev.HTMLLink != "" {
		b.Attribution("Link: " + ev.HTMLLink)
	}
	if ev.Recurring != "" {
		b.Attribution("Part of recurring event: " + ev.Recurring)
	}

	if len(ev.Attendees) > 0 {
		b.BlankLine()
		b.Heading(2, "Attendees")
		for _, a := range ev.Attendees {
			line := "- " + formatAttendee(a)
			b.Raw(line + "\n")
		}
	}

	if ev.Description != "" {
		b.BlankLine()
		b.Heading(2, "Description")
		b.WriteMarkdown(markdown.Markdown(ev.Description))
		if !strings.HasSuffix(ev.Description, "\n") {
			b.Raw("\n")
		}
	}

	return b.Build()
}

func formatAttendee(a renderedAttendee) string {
	label := a.Email
	if a.Name != "" {
		label = fmt.Sprintf("%s <%s>", a.Name, a.Email)
	}
	parts := []string{label}
	if a.Response != "" {
		parts = append(parts, "("+a.Response+")")
	}
	if a.Optional {
		parts = append(parts, "(optional)")
	}
	return strings.Join(parts, " ")
}
