package ashby

import mcp "github.com/daltoniam/switchboard"

var offerEndpoints = []endpoint{
	{
		tool:   "ashby_list_offers",
		path:   "offer.list",
		access: mcp.AccessRead,
		desc:   "List Ashby offers (job offers to candidates) with offer status, acceptance status, application, and latest version (start date, salary, approval). Start here for recruiting questions about offers, pending offers, or accepted and declined offers.",
		params: params(synced(), ps(
			str("application_id", "Only offers for this application."),
			strs("offer_status", "Offer process statuses, such as WaitingOnOfferApproval, WaitingOnCandidateResponse, CandidateAccepted, CandidateRejected, OfferCancelled."),
			strs("acceptance_status", "Acceptance statuses: Accepted, Declined, Pending, Created, Cancelled."),
			strs("approval_status", "Latest version approval statuses (imported offers only)."),
			createdAfter("offers"),
		)),
	},
	{
		tool:   "ashby_get_offer",
		path:   "offer.info",
		access: mcp.AccessRead,
		desc:   "Get one Ashby offer with all versions, offer form answers, and document handles. Use after ashby_list_offers.",
		params: ps(
			str("offer_id", "The offer id.").req(),
			boolean("exclude_form_definition", "Leave out the offer form definition."),
			object("expand", "Extra documents, for example {\"individualSignedPacketDocuments\": true}."),
		),
	},
	{
		tool:   "ashby_start_offer_process",
		path:   "offerProcess.start",
		access: mcp.AccessWrite,
		desc:   "Start an Ashby offer process for an application. First step to make a candidate an offer; then ashby_start_offer and ashby_create_offer.",
		params: ps(str("application_id", "The application id.").req()),
	},
	{
		tool:   "ashby_start_offer",
		path:   "offer.start",
		access: mcp.AccessWrite,
		desc:   "Create a blank Ashby offer version in an offer process and return its form definition to fill in with ashby_create_offer.",
		params: ps(str("offer_process_id", "Offer process id from ashby_start_offer_process.").req()),
	},
	{
		tool:   "ashby_create_offer",
		path:   "offer.create",
		access: mcp.AccessWrite,
		desc:   "Submit a new Ashby offer (salary, start date, and other offer form fields) for an offer process.",
		params: ps(
			str("offer_process_id", "Offer process id from ashby_start_offer_process.").req(),
			str("offer_form_id", "Offer form id from ashby_start_offer.").req(),
			object("offer_form", "Form data: {\"fieldSubmissions\": [{\"path\", \"value\"}]}.").req(),
			boolean("exclude_form_definition", "Leave out the form definition in the response."),
		),
	},
	{
		tool:   "ashby_update_offer",
		path:   "offer.update",
		access: mcp.AccessWrite,
		desc:   "Update an existing Ashby offer's form answers, creating a new offer version.",
		params: ps(
			str("offer_id", "The offer id.").req(),
			object("offer_form", "Form data: {\"fieldSubmissions\": [{\"path\", \"value\"}]}.").req(),
			boolean("exclude_form_definition", "Leave out the form definition in the response."),
		),
	},
	{
		tool:   "ashby_approve_offer",
		path:   "offer.approve",
		access: mcp.AccessWrite,
		desc:   "Approve an Ashby offer version, or one approval step when approval_step_id and user_id are given. Use ashby_list_approvals for the offer version id.",
		params: ps(
			str("offer_version_id", "Offer version id (approval entityId).").req(),
			str("approval_step_id", "Approval step id; needs user_id."),
			str("user_id", "Approving user id."),
			str("comment", "Approval comment."),
			boolean("exclude_form_definition", "Leave out the form definition in the response."),
		),
	},
}
