package homeassistant

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── States / Entities ───────────────────────────────────────────
	{
		Name: "homeassistant_list_states", Description: "List all entity states in Home Assistant",
		Parameters: map[string]string{},
	},
	{
		Name: "homeassistant_get_state", Description: "Get the current state of a specific entity",
		Parameters: map[string]string{"entity_id": "Entity ID (e.g. light.living_room, sensor.temperature)"},
		Required:   []string{"entity_id"},
	},
	{
		Name: "homeassistant_set_state", Description: "Create or update the state of an entity",
		Parameters: map[string]string{"entity_id": "Entity ID", "state": "New state value", "attributes": "JSON string of entity attributes"},
		Required:   []string{"entity_id", "state"},
	},
	{
		Name: "homeassistant_delete_state", Description: "Delete an entity from Home Assistant",
		Parameters: map[string]string{"entity_id": "Entity ID to delete"},
		Required:   []string{"entity_id"},
	},

	// ── Services ────────────────────────────────────────────────────
	{
		Name: "homeassistant_list_services", Description: "List all available services grouped by domain",
		Parameters: map[string]string{},
	},
	{
		Name: "homeassistant_call_service", Description: "Call a Home Assistant service (e.g. turn on a light, lock a door)",
		Parameters: map[string]string{"domain": "Service domain (e.g. light, switch, automation)", "service": "Service name (e.g. turn_on, turn_off, toggle)", "service_data": "JSON string of service data (e.g. entity_id, brightness)", "return_response": "Return service response data (true/false)"},
		Required:   []string{"domain", "service"},
	},

	// ── Events ──────────────────────────────────────────────────────
	{
		Name: "homeassistant_list_events", Description: "List available event types with listener counts",
		Parameters: map[string]string{},
	},
	{
		Name: "homeassistant_fire_event", Description: "Fire a custom event in Home Assistant",
		Parameters: map[string]string{"event_type": "Event type name", "event_data": "JSON string of event data"},
		Required:   []string{"event_type"},
	},

	// ── History ─────────────────────────────────────────────────────
	{
		Name: "homeassistant_get_history", Description: "Get state change history for entities over a time period",
		Parameters: map[string]string{"entity_id": "Comma-separated entity IDs to filter (required)", "start_time": "Start time in ISO 8601 format (defaults to 1 day ago)", "end_time": "End time in ISO 8601 format", "minimal_response": "Only return last_changed and state for intermediate states (true/false)", "no_attributes": "Skip returning attributes for faster response (true/false)", "significant_changes_only": "Only return significant state changes (true/false)"},
		Required:   []string{"entity_id"},
	},

	// ── Logbook ─────────────────────────────────────────────────────
	{
		Name: "homeassistant_get_logbook", Description: "Get logbook entries showing what happened and when",
		Parameters: map[string]string{"start_time": "Start time in ISO 8601 format (defaults to 1 day ago)", "end_time": "End time in ISO 8601 format", "entity_id": "Filter by single entity ID"},
	},

	// ── Config ──────────────────────────────────────────────────────
	{
		Name: "homeassistant_get_config", Description: "Get Home Assistant configuration (location, version, components, units)",
		Parameters: map[string]string{},
	},
	{
		Name: "homeassistant_check_config", Description: "Validate the Home Assistant configuration.yaml file",
		Parameters: map[string]string{},
	},

	// ── Template ────────────────────────────────────────────────────
	{
		Name: "homeassistant_render_template", Description: "Render a Jinja2 template with Home Assistant context (access states, attributes, etc.)",
		Parameters: map[string]string{"template": "Jinja2 template string (e.g. '{{ states(\"sensor.temperature\") }}')"},
		Required:   []string{"template"},
	},

	// ── Error Log ───────────────────────────────────────────────────
	{
		Name: "homeassistant_get_error_log", Description: "Get the Home Assistant error log for the current session",
		Parameters: map[string]string{},
	},

	// ── Calendars ───────────────────────────────────────────────────
	{
		Name: "homeassistant_list_calendars", Description: "List all calendar entities",
		Parameters: map[string]string{},
	},
	{
		Name: "homeassistant_get_calendar_events", Description: "Get events from a specific calendar within a time range",
		Parameters: map[string]string{"entity_id": "Calendar entity ID (e.g. calendar.personal)", "start": "Start time in ISO 8601 format", "end": "End time in ISO 8601 format"},
		Required:   []string{"entity_id", "start", "end"},
	},

	// ── Intents ─────────────────────────────────────────────────────
	{
		Name: "homeassistant_handle_intent", Description: "Handle a voice assistant intent (e.g. turn on lights via natural language)",
		Parameters: map[string]string{"name": "Intent name (e.g. HassTurnOn)", "data": "JSON string of intent data (e.g. entity, area)"},
		Required:   []string{"name"},
	},
}
