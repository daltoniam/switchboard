package gforms

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ─────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	for name := range markdownRenderers {
		_, ok := dispatch[name]
		assert.True(t, ok, "markdown renderer %s has no dispatch handler", name)
	}
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_nope"), []byte(`{}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_form ───────────────────────────────────────────────────────

func TestRenderForm_Basic(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "FORM_ABC",
		"revisionId": "rev1",
		"responderUri": "https://docs.google.com/forms/.../viewform",
		"info": {"title": "Customer Feedback", "description": "Tell us what you think"},
		"items": [
			{"itemId": "i1", "title": "How are you?"},
			{"itemId": "i2", "title": "Comments"}
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "form_id=FORM_ABC")
	assert.Contains(t, s, "# Customer Feedback")
	assert.Contains(t, s, "Tell us what you think")
	assert.Contains(t, s, "Revision: rev1")
	assert.Contains(t, s, "viewform")
	assert.Contains(t, s, "## Items (2)")
	assert.Contains(t, s, "Item 1: How are you?")
	assert.Contains(t, s, "Item 2: Comments")
}

func TestRenderForm_UntitledFallback(t *testing.T) {
	g := &gforms{}
	body := []byte(`{"formId":"F","info":{},"items":[]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "# (untitled form)")
}

func TestRenderForm_QuizBadge(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "Quiz"},
		"settings": {"quizSettings": {"isQuiz": true}}
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "Quiz")
}

func TestRenderForm_TextQuestion(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{
			"itemId": "i1",
			"title": "Name",
			"questionItem": {"question": {"questionId": "q1", "required": true, "textQuestion": {}}}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Short answer")
	assert.Contains(t, s, "required")
	assert.Contains(t, s, "id=`q1`")
}

func TestRenderForm_ParagraphQuestion(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{
			"itemId": "i1",
			"title": "Comments",
			"questionItem": {"question": {"questionId": "q1", "textQuestion": {"paragraph": true}}}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "Paragraph")
}

func TestRenderForm_ChoiceQuestion(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{
			"itemId": "i1",
			"title": "Favorite color?",
			"questionItem": {"question": {
				"questionId": "q1",
				"choiceQuestion": {
					"type": "RADIO",
					"options": [{"value": "Red"}, {"value": "Green"}, {"isOther": true}]
				}
			}}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Multiple choice")
	assert.Contains(t, s, "- Red")
	assert.Contains(t, s, "- Green")
	assert.Contains(t, s, "- Other")
}

func TestRenderForm_ScaleQuestion(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{
			"itemId": "i1",
			"title": "Satisfaction",
			"questionItem": {"question": {
				"scaleQuestion": {"low": 1, "high": 5, "lowLabel": "Bad", "highLabel": "Good"}
			}}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Linear scale")
	assert.Contains(t, s, "1 (Bad)")
	assert.Contains(t, s, "5 (Good)")
}

func TestRenderForm_DateAndTime(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [
			{"itemId": "i1", "title": "When?", "questionItem": {"question": {"dateQuestion": {"includeTime": true, "includeYear": true}}}},
			{"itemId": "i2", "title": "How long?", "questionItem": {"question": {"timeQuestion": {"duration": true}}}}
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Date")
	assert.Contains(t, s, "duration")
}

func TestRenderForm_GradingPoints(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "Quiz"},
		"items": [{
			"itemId": "i1",
			"title": "2+2?",
			"questionItem": {"question": {
				"textQuestion": {},
				"grading": {"pointValue": 5}
			}}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "**Points: 5**")
}

func TestRenderForm_PageBreak(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{"itemId": "i1", "title": "Section 2", "pageBreakItem": {"title": "Part 2"}}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Page break")
	assert.Contains(t, s, "Part 2")
}

func TestRenderForm_ImageItem(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{"itemId": "i1", "title": "Logo", "imageItem": {"image": {"contentUri": "https://example.com/x.png", "altText": "Logo"}}}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "![Logo](https://example.com/x.png)")
}

func TestRenderForm_VideoItem(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"formId": "F",
		"info": {"title": "F"},
		"items": [{"itemId": "i1", "title": "Watch", "videoItem": {"video": {"youtubeUri": "https://youtu.be/abc"}, "caption": "Intro"}}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "📺 [Intro](https://youtu.be/abc)")
}

func TestRenderForm_InvalidJSON(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), []byte(`{not json`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderForm_MissingID(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_form"), []byte(`{"info":{"title":"x"}}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── list_responses ─────────────────────────────────────────────────

func TestRenderResponses_Basic(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"responses": [
			{
				"responseId": "r1",
				"createTime": "2024-05-01T10:00:00Z",
				"lastSubmittedTime": "2024-05-01T10:05:00Z",
				"respondentEmail": "alice@example.com",
				"answers": {
					"q1": {"questionId": "q1", "textAnswers": {"answers": [{"value": "Red"}]}},
					"q2": {"questionId": "q2", "textAnswers": {"answers": [{"value": "Pretty good"}]}}
				}
			},
			{
				"responseId": "r2",
				"answers": {
					"q1": {"questionId": "q1", "textAnswers": {"answers": [{"value": "Blue"}]}}
				}
			}
		],
		"nextPageToken": "tok-2"
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "responses=2")
	assert.Contains(t, s, "# Form Responses (2)")
	assert.Contains(t, s, "Next page token: tok-2")
	assert.Contains(t, s, "Response 1 (`r1`)")
	assert.Contains(t, s, "From: alice@example.com")
	assert.Contains(t, s, "Submitted: 2024-05-01T10:05:00Z")
	assert.Contains(t, s, "**`q1`**: Red")
	assert.Contains(t, s, "**`q2`**: Pretty good")
	assert.Contains(t, s, "Response 2 (`r2`)")
}

func TestRenderResponses_Empty(t *testing.T) {
	g := &gforms{}
	body := []byte(`{"responses":[],"nextPageToken":"continue"}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Form Responses (0)")
	assert.Contains(t, s, "Next page token: continue")
}

func TestRenderResponses_FileAnswers(t *testing.T) {
	g := &gforms{}
	body := []byte(`{"responses":[{
		"responseId": "r1",
		"answers": {
			"q1": {"questionId": "q1", "fileUploadAnswers": {"answers": [
				{"fileId": "abc", "fileName": "essay.pdf", "mimeType": "application/pdf"}
			]}}
		}
	}]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "📎 essay.pdf")
}

func TestRenderResponses_GradedAnswer(t *testing.T) {
	g := &gforms{}
	score := 1.5
	body := []byte(`{"responses":[{
		"responseId": "r1",
		"totalScore": 5,
		"answers": {
			"q1": {
				"questionId": "q1",
				"textAnswers": {"answers": [{"value": "4"}]},
				"grade": {"score": ` + jsonNumberOrNull(&score) + `, "correct": true}
			}
		}
	}]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Score: 5")
	assert.Contains(t, s, "correct")
}

func TestRenderResponses_NoAnswers(t *testing.T) {
	g := &gforms{}
	body := []byte(`{"responses":[{"responseId":"r1"}]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "_(no answers)_")
}

func TestRenderResponses_InvalidJSON(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), []byte(`xxx`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderResponses_NoResponsesKey(t *testing.T) {
	// Object with no responses and no nextPageToken → not a list payload.
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_list_responses"), []byte(`{}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_response ───────────────────────────────────────────────────

func TestRenderResponse_Basic(t *testing.T) {
	g := &gforms{}
	body := []byte(`{
		"responseId": "r1",
		"createTime": "2024-05-01T10:00:00Z",
		"respondentEmail": "bob@example.com",
		"answers": {
			"q1": {"questionId": "q1", "textAnswers": {"answers": [{"value": "hello"}]}}
		}
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_response"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "response_id=r1")
	assert.Contains(t, s, "Response 1 (`r1`)")
	assert.Contains(t, s, "From: bob@example.com")
	assert.Contains(t, s, "**`q1`**: hello")
}

func TestRenderResponse_MissingID(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_response"), []byte(`{"answers":{}}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderResponse_InvalidJSON(t *testing.T) {
	g := &gforms{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gforms_get_response"), []byte(`xxx`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── Helpers ─────────────────────────────────────────────────────────

func TestPipeSafe(t *testing.T) {
	assert.Equal(t, "a b c", pipeSafe("a\nb\nc"))
	assert.Equal(t, `a\|b`, pipeSafe("a|b"))
}

func TestQuestionKind(t *testing.T) {
	cases := []struct {
		name string
		q    rawQuestion
		want string
	}{
		{"short", rawQuestion{TextQuestion: &rawTextQ{}}, "Short answer"},
		{"paragraph", rawQuestion{TextQuestion: &rawTextQ{Paragraph: true}}, "Paragraph"},
		{"radio", rawQuestion{ChoiceQuestion: &rawChoiceQ{Type: "RADIO"}}, "Multiple choice"},
		{"checkbox", rawQuestion{ChoiceQuestion: &rawChoiceQ{Type: "CHECKBOX"}}, "Checkboxes"},
		{"dropdown", rawQuestion{ChoiceQuestion: &rawChoiceQ{Type: "DROP_DOWN"}}, "Dropdown"},
		{"scale", rawQuestion{ScaleQuestion: &rawScaleQ{}}, "Linear scale"},
		{"date", rawQuestion{DateQuestion: &rawDateQ{}}, "Date"},
		{"time", rawQuestion{TimeQuestion: &rawTimeQ{}}, "Time"},
		{"file", rawQuestion{FileUploadQuestion: &rawFileQ{}}, "File upload"},
		{"rating", rawQuestion{Rating: &rawRatingQ{}}, "Rating"},
		{"unknown", rawQuestion{}, "Question"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, questionKind(&c.q))
		})
	}
}

// jsonNumberOrNull renders a float pointer as a JSON number, or "null" when nil.
// Used to dodge JSON's lack of NaN-style nullable numbers in fixtures.
func jsonNumberOrNull(f *float64) string {
	if f == nil {
		return "null"
	}
	// Always emit decimal so it parses as a JSON number.
	return formatFloat(*f)
}

func formatFloat(f float64) string {
	// Compact float formatting (enough for fixtures).
	if f == float64(int64(f)) {
		return itoa(int64(f))
	}
	return ftoa(f)
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func ftoa(f float64) string {
	// Use a tiny manual formatter rather than importing strconv just here.
	whole := int64(f)
	frac := f - float64(whole)
	if frac < 0 {
		frac = -frac
	}
	fracStr := ""
	for i := 0; i < 6 && frac > 0; i++ {
		frac *= 10
		d := int64(frac)
		fracStr += string(byte('0' + d))
		frac -= float64(d)
	}
	if fracStr == "" {
		fracStr = "0"
	}
	return itoa(whole) + "." + fracStr
}
