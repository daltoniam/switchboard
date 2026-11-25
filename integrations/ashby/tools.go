package ashby

import mcp "github.com/daltoniam/switchboard"

// dispatch maps each exposed tool to its upstream hosted MCP tool name. Only
// these tools are forwarded; any other upstream tool (including
// submit_mcp_feedback, which messages Ashby's product team) is refused
// locally until it has a reviewed definition and access class.
var dispatch = map[mcp.ToolName]string{
	"ashby_search_records_by_name":     "search_records_by_name",
	"ashby_filter_records":             "filter_records",
	"ashby_describe_object_fields":     "describe_object_fields",
	"ashby_get_record_details":         "get_record_details",
	"ashby_get_candidate":              "get_candidate",
	"ashby_get_upcoming_interviews":    "get_upcoming_interviews",
	"ashby_get_interview_details":      "get_interview_details",
	"ashby_get_interview_plan":         "get_interview_plan",
	"ashby_get_submitted_feedback":     "get_submitted_feedback",
	"ashby_get_job_pipeline":           "get_job_pipeline",
	"ashby_get_pending_tasks":          "get_pending_tasks",
	"ashby_search_product_docs":        "search_product_docs",
	"ashby_create_candidate":           "create_candidate",
	"ashby_add_note_to_candidate":      "add_note_to_candidate",
	"ashby_change_application_stage":   "change_application_stage",
	"ashby_consider_candidate_for_job": "consider_candidate_for_job",
}

// tools is the reviewed catalog. Ashby does not publish input schemas and
// says they may change, so Parameters stay empty here and are filled from
// the hosted server's own tool list once the integration is connected.
var tools = []mcp.ToolDefinition{
	// ── Find records ─────────────────────────────────────────────────
	{
		Name:        "ashby_search_records_by_name",
		Description: "Find, search, and look up (lookup) Ashby ATS recruiting records by name: candidate or applicant, candidates, applicants, applications, jobs, job openings, and email templates. Start here for hiring and recruiting questions such as finding a candidate or pulling up a job, then drill in with ashby_get_candidate or ashby_get_record_details.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_filter_records",
		Description: "Filter or count Ashby recruiting records matching criteria: candidates, applicants, applications, jobs, job postings, openings, scheduled interviews, and offers (for example applications in an interview stage for a role). Can return a total count instead of a list. Use ashby_describe_object_fields first to learn the filterable fields.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_describe_object_fields",
		Description: "Describe the filterable fields of an Ashby record type (candidate, application, job, job posting, opening, scheduled interview, offer). Use before ashby_filter_records so filters use valid field names.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_record_details",
		Description: "Get full details of Ashby recruiting records already located: jobs, job postings, applications, email templates, openings, scheduled interviews, and offers. Use after ashby_search_records_by_name or ashby_filter_records.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_candidate",
		Description: "Get an Ashby candidate or applicant profile: resume, interview feedback and scorecards, emails, texts, notes, interview transcripts, referrals, follow-ups, and form submissions. Use after ashby_search_records_by_name to summarize a candidate's background.",
		Access:      mcp.AccessRead,
	},
	// ── Interviews and hiring pipeline ───────────────────────────────
	{
		Name:        "ashby_get_upcoming_interviews",
		Description: "List your upcoming Ashby interviews: upcoming interview schedule with candidate names, job titles, times, and meeting links. Use to answer which interviews are scheduled this week.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_interview_details",
		Description: "Get an interview prep briefing from Ashby: candidate background, resume summary, interview panel, and hiring team. Use after ashby_get_upcoming_interviews, right before an interview.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_interview_plan",
		Description: "Get the interview plan and hiring stages a candidate moves through for an Ashby job or application. Use before ashby_change_application_stage to find the target stage.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_submitted_feedback",
		Description: "Get interview scorecard feedback you personally submitted in Ashby for a candidate or interview. Shows only your own feedback, not other interviewers'.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_job_pipeline",
		Description: "Get Ashby hiring pipeline counts: how many candidates and applicants are at each stage of a job, comparing several job openings at once.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_get_pending_tasks",
		Description: "List your pending Ashby recruiting tasks: scorecards you owe, approvals waiting on you, follow-ups, candidate reviews, and survey requests.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "ashby_search_product_docs",
		Description: "Search Ashby product documentation and get an answer with links, for how-to questions about the Ashby recruiting platform such as setting up approval chains or job postings.",
		Access:      mcp.AccessRead,
	},
	// ── Changes ──────────────────────────────────────────────────────
	{
		Name:        "ashby_create_candidate",
		Description: "Create (add) a new Ashby candidate record (new applicant) with name and contact details such as email and phone. Search with ashby_search_records_by_name first to avoid duplicates, then use ashby_consider_candidate_for_job for a job.",
		Access:      mcp.AccessWrite,
	},
	{
		Name:        "ashby_add_note_to_candidate",
		Description: "Add a note to an Ashby candidate profile, or reply to an existing note. Ashby drafts the note for confirmation before saving. Use after ashby_get_candidate.",
		Access:      mcp.AccessWrite,
	},
	{
		Name:        "ashby_change_application_stage",
		Description: "Move an Ashby application to another interview stage, or archive (reject) it with an archive reason. Archiving cancels the candidate's scheduled interviews unless told not to, closes unscheduled interview requests, and runs the stage's automatic activities. Use after ashby_get_interview_plan.",
		Access:      mcp.AccessDestructive,
	},
	{
		Name:        "ashby_consider_candidate_for_job",
		Description: "Consider an existing Ashby candidate for a job by creating an application at the first stage of that job's interview plan. Use after ashby_search_records_by_name finds both the candidate and the job.",
		Access:      mcp.AccessWrite,
	},
}
