package gforms

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gforms_get_form":       renderFormMD,
	"gforms_list_responses": renderResponsesMD,
	"gforms_get_response":   renderResponseMD,
}

func (g *gforms) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawForm struct {
	FormID       string    `json:"formId"`
	RevisionID   string    `json:"revisionId"`
	ResponderURI string    `json:"responderUri"`
	LinkedSheet  string    `json:"linkedSheetId"`
	Info         rawInfo   `json:"info"`
	Settings     *rawSet   `json:"settings,omitempty"`
	Items        []rawItem `json:"items"`
}

type rawInfo struct {
	Title         string `json:"title"`
	DocumentTitle string `json:"documentTitle"`
	Description   string `json:"description"`
}

type rawSet struct {
	QuizSettings *rawQuiz `json:"quizSettings,omitempty"`
}

type rawQuiz struct {
	IsQuiz bool `json:"isQuiz"`
}

type rawItem struct {
	ItemID            string            `json:"itemId"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	QuestionItem      *rawQuestionItem  `json:"questionItem,omitempty"`
	QuestionGroupItem *rawQuestionGroup `json:"questionGroupItem,omitempty"`
	PageBreakItem     *rawPageBreak     `json:"pageBreakItem,omitempty"`
	TextItem          *json.RawMessage  `json:"textItem,omitempty"`
	ImageItem         *rawImageItem     `json:"imageItem,omitempty"`
	VideoItem         *rawVideoItem     `json:"videoItem,omitempty"`
}

type rawQuestionItem struct {
	Question rawQuestion `json:"question"`
	Image    *rawImage   `json:"image,omitempty"`
}

type rawQuestion struct {
	QuestionID         string      `json:"questionId"`
	Required           bool        `json:"required"`
	TextQuestion       *rawTextQ   `json:"textQuestion,omitempty"`
	ChoiceQuestion     *rawChoiceQ `json:"choiceQuestion,omitempty"`
	ScaleQuestion      *rawScaleQ  `json:"scaleQuestion,omitempty"`
	DateQuestion       *rawDateQ   `json:"dateQuestion,omitempty"`
	TimeQuestion       *rawTimeQ   `json:"timeQuestion,omitempty"`
	FileUploadQuestion *rawFileQ   `json:"fileUploadQuestion,omitempty"`
	RowQuestion        *rawRowQ    `json:"rowQuestion,omitempty"`
	Rating             *rawRatingQ `json:"rating,omitempty"`
	Grading            *rawGrading `json:"grading,omitempty"`
}

type rawTextQ struct {
	Paragraph bool `json:"paragraph"`
}

type rawChoiceQ struct {
	Type    string      `json:"type"`
	Shuffle bool        `json:"shuffle"`
	Options []rawOption `json:"options"`
}

type rawOption struct {
	Value         string `json:"value"`
	IsOther       bool   `json:"isOther"`
	GoToAction    string `json:"goToAction"`
	GoToSectionID string `json:"goToSectionId"`
}

type rawScaleQ struct {
	Low       int    `json:"low"`
	High      int    `json:"high"`
	LowLabel  string `json:"lowLabel"`
	HighLabel string `json:"highLabel"`
}

type rawDateQ struct {
	IncludeTime bool `json:"includeTime"`
	IncludeYear bool `json:"includeYear"`
}

type rawTimeQ struct {
	Duration bool `json:"duration"`
}

type rawFileQ struct {
	FolderID    string   `json:"folderId"`
	Types       []string `json:"types"`
	MaxFiles    int      `json:"maxFiles"`
	MaxFileSize string   `json:"maxFileSize"`
}

type rawRowQ struct {
	Title string `json:"title"`
}

type rawRatingQ struct {
	RatingScaleLevel int    `json:"ratingScaleLevel"`
	IconType         string `json:"iconType"`
}

type rawGrading struct {
	PointValue     int            `json:"pointValue"`
	CorrectAnswers *rawCorrectAns `json:"correctAnswers,omitempty"`
}

type rawCorrectAns struct {
	Answers []rawAnsValue `json:"answers"`
}

type rawAnsValue struct {
	Value string `json:"value"`
}

type rawQuestionGroup struct {
	Questions []rawQuestion `json:"questions"`
	Grid      *rawGrid      `json:"grid,omitempty"`
}

type rawGrid struct {
	Columns          *rawChoiceQ `json:"columns,omitempty"`
	ShuffleQuestions bool        `json:"shuffleQuestions"`
}

type rawPageBreak struct {
	Title string `json:"title"`
}

type rawImage struct {
	ContentURI string `json:"contentUri"`
	AltText    string `json:"altText"`
}

type rawImageItem struct {
	Image *rawImage `json:"image,omitempty"`
}

type rawVideoItem struct {
	Video   *rawVideo `json:"video,omitempty"`
	Caption string    `json:"caption"`
}

type rawVideo struct {
	YouTubeURI string `json:"youtubeUri"`
}

type rawResponseList struct {
	Responses     []rawResponse `json:"responses"`
	NextPageToken string        `json:"nextPageToken"`
}

type rawResponse struct {
	ResponseID        string               `json:"responseId"`
	FormID            string               `json:"formId"`
	CreateTime        string               `json:"createTime"`
	LastSubmittedTime string               `json:"lastSubmittedTime"`
	RespondentEmail   string               `json:"respondentEmail"`
	TotalScore        *float64             `json:"totalScore,omitempty"`
	Answers           map[string]rawAnswer `json:"answers"`
}

type rawAnswer struct {
	QuestionID        string            `json:"questionId"`
	TextAnswers       *rawAnswerSet     `json:"textAnswers,omitempty"`
	FileUploadAnswers *rawFileAnswerSet `json:"fileUploadAnswers,omitempty"`
	Grade             *rawGrade         `json:"grade,omitempty"`
}

type rawAnswerSet struct {
	Answers []rawAnsValue `json:"answers"`
}

type rawFileAnswerSet struct {
	Answers []rawFileAns `json:"answers"`
}

type rawFileAns struct {
	FileID   string `json:"fileId"`
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
}

type rawGrade struct {
	Score    *float64 `json:"score,omitempty"`
	Correct  bool     `json:"correct"`
	Feedback string   `json:"feedback"`
}

// ── Rendering: form ────────────────────────────────────────────────

func renderFormMD(data []byte) (markdown.Markdown, bool) {
	var f rawForm
	if err := json.Unmarshal(data, &f); err != nil {
		return "", false
	}
	if f.FormID == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gforms", "form_id", f.FormID, "items", fmt.Sprintf("%d", len(f.Items)))

	title := f.Info.Title
	if title == "" {
		title = f.Info.DocumentTitle
	}
	if title == "" {
		title = "(untitled form)"
	}
	b.Heading(1, title)

	attrs := []string{}
	if f.Settings != nil && f.Settings.QuizSettings != nil && f.Settings.QuizSettings.IsQuiz {
		attrs = append(attrs, "Quiz")
	}
	if f.RevisionID != "" {
		attrs = append(attrs, "Revision: "+f.RevisionID)
	}
	if f.ResponderURI != "" {
		attrs = append(attrs, "Responder URL: "+f.ResponderURI)
	}
	if len(attrs) > 0 {
		b.Attribution(attrs...)
	}
	if f.Info.Description != "" {
		b.BlankLine()
		b.Raw(f.Info.Description + "\n")
	}
	b.BlankLine()

	b.Heading(2, fmt.Sprintf("Items (%d)", len(f.Items)))
	b.BlankLine()
	for i, item := range f.Items {
		renderItem(b, i+1, &item)
	}
	return b.Build(), true
}

func renderItem(b *markdown.Builder, num int, item *rawItem) {
	heading := fmt.Sprintf("Item %d", num)
	if item.Title != "" {
		heading = fmt.Sprintf("Item %d: %s", num, item.Title)
	}
	if item.ItemID != "" {
		heading += fmt.Sprintf(" (`%s`)", item.ItemID)
	}
	b.Heading(3, heading)

	if item.Description != "" {
		b.Raw("_" + item.Description + "_\n")
	}

	switch {
	case item.QuestionItem != nil:
		renderQuestion(b, &item.QuestionItem.Question)
	case item.QuestionGroupItem != nil:
		b.Raw("**Question group** (" + fmt.Sprintf("%d sub-questions", len(item.QuestionGroupItem.Questions)) + ")\n")
		for _, q := range item.QuestionGroupItem.Questions {
			if q.RowQuestion != nil && q.RowQuestion.Title != "" {
				b.Raw("  - " + q.RowQuestion.Title + "\n")
			}
		}
	case item.PageBreakItem != nil:
		b.Raw("**Page break**")
		if item.PageBreakItem.Title != "" {
			b.Raw(": " + item.PageBreakItem.Title)
		}
		b.Raw("\n")
	case item.TextItem != nil:
		b.Raw("_Text-only item_\n")
	case item.ImageItem != nil:
		if item.ImageItem.Image != nil && item.ImageItem.Image.ContentURI != "" {
			alt := item.ImageItem.Image.AltText
			if alt == "" {
				alt = "image"
			}
			b.Raw(fmt.Sprintf("![%s](%s)\n", pipeSafe(alt), item.ImageItem.Image.ContentURI))
		}
	case item.VideoItem != nil:
		if item.VideoItem.Video != nil && item.VideoItem.Video.YouTubeURI != "" {
			caption := item.VideoItem.Caption
			if caption == "" {
				caption = "Video"
			}
			b.Raw(fmt.Sprintf("📺 [%s](%s)\n", caption, item.VideoItem.Video.YouTubeURI))
		}
	}
	b.BlankLine()
}

func renderQuestion(b *markdown.Builder, q *rawQuestion) {
	kind := questionKind(q)
	parts := []string{kind}
	if q.Required {
		parts = append(parts, "required")
	}
	if q.QuestionID != "" {
		parts = append(parts, "id=`"+q.QuestionID+"`")
	}
	b.Raw("_" + strings.Join(parts, " | ") + "_\n")

	switch {
	case q.ChoiceQuestion != nil && len(q.ChoiceQuestion.Options) > 0:
		for _, opt := range q.ChoiceQuestion.Options {
			val := opt.Value
			if opt.IsOther {
				val = "Other"
			}
			b.Raw("  - " + val + "\n")
		}
	case q.ScaleQuestion != nil:
		b.Raw(fmt.Sprintf("  - Scale: %d (%s) → %d (%s)\n",
			q.ScaleQuestion.Low, q.ScaleQuestion.LowLabel,
			q.ScaleQuestion.High, q.ScaleQuestion.HighLabel))
	case q.DateQuestion != nil:
		b.Raw(fmt.Sprintf("  - Date (includeTime=%v, includeYear=%v)\n", q.DateQuestion.IncludeTime, q.DateQuestion.IncludeYear))
	case q.TimeQuestion != nil:
		mode := "time of day"
		if q.TimeQuestion.Duration {
			mode = "duration"
		}
		b.Raw("  - Time (" + mode + ")\n")
	case q.FileUploadQuestion != nil:
		b.Raw(fmt.Sprintf("  - File upload (maxFiles=%d, maxFileSize=%s)\n", q.FileUploadQuestion.MaxFiles, q.FileUploadQuestion.MaxFileSize))
	case q.Rating != nil:
		b.Raw(fmt.Sprintf("  - Rating (%d %s)\n", q.Rating.RatingScaleLevel, q.Rating.IconType))
	}

	if q.Grading != nil {
		b.Raw(fmt.Sprintf("  - **Points: %d**\n", q.Grading.PointValue))
	}
}

func questionKind(q *rawQuestion) string {
	switch {
	case q.TextQuestion != nil:
		if q.TextQuestion.Paragraph {
			return "Paragraph"
		}
		return "Short answer"
	case q.ChoiceQuestion != nil:
		switch q.ChoiceQuestion.Type {
		case "RADIO":
			return "Multiple choice"
		case "CHECKBOX":
			return "Checkboxes"
		case "DROP_DOWN":
			return "Dropdown"
		default:
			return "Choice"
		}
	case q.ScaleQuestion != nil:
		return "Linear scale"
	case q.DateQuestion != nil:
		return "Date"
	case q.TimeQuestion != nil:
		return "Time"
	case q.FileUploadQuestion != nil:
		return "File upload"
	case q.Rating != nil:
		return "Rating"
	case q.RowQuestion != nil:
		return "Row"
	}
	return "Question"
}

// ── Rendering: responses ───────────────────────────────────────────

func renderResponsesMD(data []byte) (markdown.Markdown, bool) {
	var rl rawResponseList
	if err := json.Unmarshal(data, &rl); err != nil {
		return "", false
	}
	// An empty responses list is a valid shape — render an empty section.
	// Refuse only when the JSON has no "responses" key at all (parsed as
	// nil slice, but no other identifying fields either).
	if rl.Responses == nil && rl.NextPageToken == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gforms", "responses", fmt.Sprintf("%d", len(rl.Responses)))
	b.Heading(1, fmt.Sprintf("Form Responses (%d)", len(rl.Responses)))
	if rl.NextPageToken != "" {
		b.Attribution("Next page token: " + rl.NextPageToken)
	}
	b.BlankLine()

	for i, resp := range rl.Responses {
		renderResponseSection(b, i+1, &resp)
	}
	return b.Build(), true
}

func renderResponseMD(data []byte) (markdown.Markdown, bool) {
	var resp rawResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", false
	}
	if resp.ResponseID == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gforms", "response_id", resp.ResponseID, "answers", fmt.Sprintf("%d", len(resp.Answers)))
	renderResponseSection(b, 1, &resp)
	return b.Build(), true
}

func renderResponseSection(b *markdown.Builder, num int, resp *rawResponse) {
	heading := fmt.Sprintf("Response %d", num)
	if resp.ResponseID != "" {
		heading = fmt.Sprintf("Response %d (`%s`)", num, resp.ResponseID)
	}
	b.Heading(2, heading)

	attrs := []string{}
	if resp.RespondentEmail != "" {
		attrs = append(attrs, "From: "+resp.RespondentEmail)
	}
	if resp.LastSubmittedTime != "" {
		attrs = append(attrs, "Submitted: "+resp.LastSubmittedTime)
	} else if resp.CreateTime != "" {
		attrs = append(attrs, "Created: "+resp.CreateTime)
	}
	if resp.TotalScore != nil {
		attrs = append(attrs, fmt.Sprintf("Score: %g", *resp.TotalScore))
	}
	if len(attrs) > 0 {
		b.Attribution(attrs...)
	}
	b.BlankLine()

	if len(resp.Answers) == 0 {
		b.Raw("_(no answers)_\n")
		b.BlankLine()
		return
	}

	// Sort question IDs deterministically.
	keys := make([]string, 0, len(resp.Answers))
	for k := range resp.Answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, qid := range keys {
		ans := resp.Answers[qid]
		renderAnswer(b, qid, &ans)
	}
	b.BlankLine()
}

func renderAnswer(b *markdown.Builder, qid string, ans *rawAnswer) {
	b.Raw("- **`" + qid + "`**: ")
	switch {
	case ans.TextAnswers != nil && len(ans.TextAnswers.Answers) > 0:
		vals := make([]string, 0, len(ans.TextAnswers.Answers))
		for _, a := range ans.TextAnswers.Answers {
			vals = append(vals, a.Value)
		}
		b.Raw(strings.Join(vals, "; "))
	case ans.FileUploadAnswers != nil && len(ans.FileUploadAnswers.Answers) > 0:
		names := make([]string, 0, len(ans.FileUploadAnswers.Answers))
		for _, a := range ans.FileUploadAnswers.Answers {
			label := a.FileName
			if label == "" {
				label = a.FileID
			}
			names = append(names, label)
		}
		b.Raw("📎 " + strings.Join(names, ", "))
	default:
		b.Raw("(empty)")
	}
	if ans.Grade != nil {
		grade := "incorrect"
		if ans.Grade.Correct {
			grade = "correct"
		}
		score := ""
		if ans.Grade.Score != nil {
			score = fmt.Sprintf(" %g pts", *ans.Grade.Score)
		}
		b.Raw(fmt.Sprintf(" _(%s%s)_", grade, score))
	}
	b.Raw("\n")
}

// ── Helpers ─────────────────────────────────────────────────────────

func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}
