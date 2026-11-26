package ashby

import mcp "github.com/daltoniam/switchboard"

var interviewEndpoints = []endpoint{
	{
		tool:   "ashby_list_interview_schedules",
		path:   "interviewSchedule.list",
		access: mcp.AccessRead,
		desc:   "List Ashby interview schedules (scheduled interviews and onsites) with their interview events, times, interviewers, and status. Start here for recruiting questions about upcoming or past interviews; filter by application_id or interview_stage_id.",
		params: params(synced(), ps(
			str("application_id", "Only schedules for this application."),
			str("interview_stage_id", "Only schedules for this interview stage."),
			createdAfter("schedules"),
		)),
	},
	{
		tool:   "ashby_get_interview_schedule",
		path:   "interviewSchedule.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby interview schedule by id with its interview events, interviewers, meeting links, and feedback status.",
		params: ps(str("interview_schedule_id", "The interview schedule id.").as("id").req()),
	},
	{
		tool:   "ashby_create_interview_schedule",
		path:   "interviewSchedule.create",
		access: mcp.AccessWrite,
		desc:   "Schedule interviews in Ashby for an application: one or more interview events with times and interviewers.",
		params: ps(
			str("application_id", "The application id.").req(),
			value("interview_events", "Events: [{\"startTime\", \"endTime\" (ISO 8601), \"interviewers\": [{\"email\", \"feedbackRequired\"}], \"interviewId\"}].").req(),
		),
	},
	{
		tool:   "ashby_update_interview_schedule",
		path:   "interviewSchedule.update",
		access: mcp.AccessDestructive,
		desc:   "Add, reschedule, or cancel interview events on an Ashby interview schedule created by this API key. Cancelling an event can delete its submitted feedback when allow_feedback_deletion is true.",
		params: ps(
			str("interview_schedule_id", "The interview schedule id.").req(),
			value("interview_event", "Event or array of events to create or update; include interviewEventId to update one."),
			value("interview_event_id_to_cancel", "Interview event id, or array of ids, to cancel."),
			boolean("allow_feedback_deletion", "Allow cancelling events that already have feedback, deleting it."),
		),
	},
	{
		tool:   "ashby_cancel_interview_schedule",
		path:   "interviewSchedule.cancel",
		access: mcp.AccessDestructive,
		desc:   "Cancel an Ashby interview schedule and all of its interview events.",
		params: ps(
			str("interview_schedule_id", "The interview schedule id.").as("id").req(),
			boolean("allow_reschedule", "Whether the schedule can be rescheduled later."),
		),
	},
	{
		tool:   "ashby_list_interview_events",
		path:   "interviewEvent.list",
		access: mcp.AccessRead,
		desc:   "List the interview events (individual interviews) in an Ashby interview schedule with times, interviewers, meeting links, and whether feedback was submitted.",
		params: params(ps(
			str("interview_schedule_id", "The interview schedule id.").req(),
			strs("expand", "Related objects to include: interview."),
			createdAfter("events"),
		), synced()),
	},
	{
		tool:   "ashby_get_interview_briefing",
		path:   "interviewBriefing.info",
		access: mcp.AccessRead,
		desc:   "Get an Ashby interview briefing (interview prep) for an interview event: candidate, application, job, interview, interviewers, and feedback form.",
		params: ps(
			str("interview_event_id", "The interview event id.").req(),
			strs("expand", "Related objects: feedbackFormDefinition, application, candidate, job, interview, interviewers."),
		),
	},
	{
		tool:   "ashby_list_interviews",
		path:   "interview.list",
		access: mcp.AccessRead,
		desc:   "List Ashby interview types and take-home exercises configured in the organization (the interview library, not scheduled interviews).",
		params: params(synced(), ps(
			boolean("include_archived", "Include archived interviews."),
			boolean("include_non_shared_interviews", "Include interviews tied to specific jobs."),
		)),
	},
	{
		tool:   "ashby_get_interview",
		path:   "interview.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby interview type or take-home exercise by id with its instructions and feedback form.",
		params: ps(str("interview_id", "The interview id.").as("id").req()),
	},
	{
		tool:   "ashby_list_interview_plans",
		path:   "interviewPlan.list",
		access: mcp.AccessRead,
		desc:   "List published Ashby interview plans (hiring pipelines). Use ashby_list_interview_stages for a plan's stages.",
		params: params(synced(), ps(boolean("include_archived", "Include archived interview plans."))),
	},
	{
		tool:   "ashby_list_interview_stages",
		path:   "interviewStage.list",
		access: mcp.AccessRead,
		desc:   "List the interview stages of an Ashby interview plan in pipeline order (Lead, screens, onsite, offer, hired, archived).",
		params: ps(str("interview_plan_id", "The interview plan id.").req()),
	},
	{
		tool:   "ashby_get_interview_stage",
		path:   "interviewStage.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby interview stage by id.",
		params: ps(str("interview_stage_id", "The interview stage id.").req()),
	},
	{
		tool:   "ashby_list_interview_stage_groups",
		path:   "interviewStageGroup.list",
		access: mcp.AccessRead,
		desc:   "List Ashby interview stage groups (pipeline phases shared across interview plans) in order.",
		params: ps(),
	},
	{
		tool:   "ashby_list_interviewer_pools",
		path:   "interviewerPool.list",
		access: mcp.AccessRead,
		desc:   "List Ashby interviewer pools (groups of qualified interviewers) and their training paths.",
		params: params(synced(), ps(boolean("include_archived_pools", "Include archived pools."))),
	},
	{
		tool:   "ashby_get_interviewer_pool",
		path:   "interviewerPool.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby interviewer pool with its qualified members and trainees.",
		params: ps(str("interviewer_pool_id", "The interviewer pool id.").req()),
	},
	{
		tool:   "ashby_list_feedback_forms",
		path:   "feedbackFormDefinition.list",
		access: mcp.AccessRead,
		desc:   "List Ashby interview feedback forms (scorecard templates).",
		params: params(synced(), ps(
			boolean("include_archived", "Include archived forms."),
			boolean("shared_only", "Only shared forms, not forms specific to one interview."),
		)),
	},
	{
		tool:   "ashby_get_feedback_form",
		path:   "feedbackFormDefinition.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby feedback form (scorecard) with its field paths and types. Use before ashby_submit_application_feedback.",
		params: ps(str("feedback_form_definition_id", "The feedback form id.").req()),
	},
	{
		tool:   "ashby_list_take_home_assignments",
		path:   "takeHomeAssignment.list",
		access: mcp.AccessRead,
		desc:   "List Ashby take-home assignments (candidate assessments and coding exercises) with status, submission, and reviewer feedback status.",
		params: params(synced(), ps(
			str("application_id", "Only assignments for this application."),
			str("candidate_id", "Only assignments for this candidate."),
			strs("expand", "Related objects to include: interview."),
		)),
	},
	{
		tool:   "ashby_get_take_home_assignment",
		path:   "takeHomeAssignment.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby take-home assignment (candidate assessment) with its submission and reviewers.",
		params: ps(
			str("take_home_assignment_id", "The take-home assignment id.").req(),
			strs("expand", "Related objects to include: interview."),
		),
	},
	{
		tool:   "ashby_get_notetaker_transcript",
		path:   "notetakerTranscript.info",
		access: mcp.AccessRead,
		desc:   "Get an Ashby AI notetaker interview transcript: participants and a short-lived download URL. Needs the Notetaker read permission.",
		params: ps(str("notetaker_transcript_id", "Transcript id from an interview event.").req()),
	},
}
