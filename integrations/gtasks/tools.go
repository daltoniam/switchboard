package gtasks

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Tasklist resource ───────────────────────────────────────────
	{
		Name: mcp.ToolName("gtasks_list_tasklists"), Description: "List the authenticated user's Google Tasks tasklists (todo lists, task lists, todo categories). Start here for todos, to-do items, action items, daily tasks, reminders, checklists — to discover the tasklist IDs needed by every other gtasks tool. The default tasklist (named 'My Tasks' or similar) always exists for any Google account.",
		Parameters: map[string]string{
			"max_results": "Optional max tasklists per page (1-100, default 100)",
			"page_token":  "Optional pagination token from a previous response's nextPageToken",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gtasks_create_tasklist"), Description: "Create a new Google Tasks tasklist (todo list, task category). Returns the new tasklist including its id.",
		Parameters: map[string]string{
			"title": "Tasklist title (shown in the Tasks UI)",
		},
		Required: []string{"title"},
	},
	{
		Name: mcp.ToolName("gtasks_delete_tasklist"), Description: "Delete a Google Tasks tasklist. Permanently removes the list AND all tasks within it. Cannot delete the default tasklist.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID (from gtasks_list_tasklists)",
		},
		Required: []string{"tasklist_id"},
	},

	// ── Task resource ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("gtasks_list_tasks"), Description: "List tasks (todos, action items) within a Google Tasks tasklist. Returns tasks with their title, notes, due date, status (needsAction/completed), parent, and position. Use to view a user's todo list, find overdue items, or build a daily agenda alongside gcal_list_events. Combine with show_completed=true to include finished tasks.",
		Parameters: map[string]string{
			"tasklist_id":    "The tasklist ID (from gtasks_list_tasklists)",
			"max_results":    "Optional max tasks per page (1-100, default 100)",
			"page_token":     "Optional pagination token from a previous response's nextPageToken",
			"show_completed": "Optional boolean — include completed tasks (default false)",
			"show_hidden":    "Optional boolean — include hidden tasks (default false)",
			"show_deleted":   "Optional boolean — include deleted tasks (default false)",
			"due_min":        "Optional lower bound on due date (RFC 3339 timestamp, e.g. 2024-01-01T00:00:00Z)",
			"due_max":        "Optional upper bound on due date (RFC 3339 timestamp)",
			"completed_min":  "Optional lower bound on completion time (RFC 3339 timestamp)",
			"completed_max":  "Optional upper bound on completion time (RFC 3339 timestamp)",
			"updated_min":    "Optional lower bound on last-modified time (RFC 3339 timestamp)",
		},
		Required: []string{"tasklist_id"},
	},
	{
		Name: mcp.ToolName("gtasks_get_task"), Description: "Retrieve a single Google Tasks task by ID. Useful when you already have the task ID from a previous list call.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID",
			"task_id":     "The task ID (from gtasks_list_tasks)",
		},
		Required: []string{"tasklist_id", "task_id"},
	},
	{
		Name: mcp.ToolName("gtasks_create_task"), Description: "Create a new task (todo, action item, reminder) in a Google Tasks tasklist. Returns the new task including its id. Use parent_id to make this a subtask of an existing task. Use previous_id to control position within siblings.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID to add the task to",
			"title":       "Task title",
			"notes":       "Optional task notes / description",
			"due":         "Optional due date (RFC 3339 timestamp). Note: the Tasks API only stores the date portion; the time is always 00:00:00Z.",
			"parent_id":   "Optional parent task ID — makes this a subtask of an existing task",
			"previous_id": "Optional sibling task ID — new task will be positioned immediately after this one (omit to insert at top)",
		},
		Required: []string{"tasklist_id", "title"},
	},
	{
		Name: mcp.ToolName("gtasks_update_task"), Description: "Update fields on an existing Google Tasks task (mark complete, change title, edit notes, set due date). Uses PATCH semantics — only the fields you provide are changed. Set status='completed' to mark a task done; set status='needsAction' to reopen.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID",
			"task_id":     "The task ID",
			"title":       "Optional new title",
			"notes":       "Optional new notes (pass empty string to clear)",
			"due":         "Optional new due date (RFC 3339 timestamp)",
			"status":      "Optional new status: 'needsAction' (open) or 'completed' (done)",
			"completed":   "Optional completion timestamp (RFC 3339); usually set automatically when status='completed'",
			"deleted":     "Optional boolean — soft-delete the task",
			"hidden":      "Optional boolean — hide the task from the default view",
		},
		Required: []string{"tasklist_id", "task_id"},
	},
	{
		Name: mcp.ToolName("gtasks_delete_task"), Description: "Permanently delete a Google Tasks task. To soft-delete instead (so it can be recovered), use gtasks_update_task with deleted=true.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID",
			"task_id":     "The task ID",
		},
		Required: []string{"tasklist_id", "task_id"},
	},
	{
		Name: mcp.ToolName("gtasks_move_task"), Description: "Move a Google Tasks task to a new position within its tasklist — reparent (make it a subtask) and/or reorder (place after a sibling). Returns the updated task.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID",
			"task_id":     "The task ID to move",
			"parent_id":   "Optional new parent task ID (omit for top-level)",
			"previous_id": "Optional sibling ID — task will be positioned immediately after this one (omit to move to top)",
		},
		Required: []string{"tasklist_id", "task_id"},
	},
	{
		Name: mcp.ToolName("gtasks_clear_completed"), Description: "Clear all completed tasks from a Google Tasks tasklist (hides them from the default view). Returns no content on success. Equivalent to the 'Delete all completed tasks' menu action in the Tasks UI — note: tasks are hidden, not permanently deleted.",
		Parameters: map[string]string{
			"tasklist_id": "The tasklist ID",
		},
		Required: []string{"tasklist_id"},
	},
}
