package metabase

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Databases ────────────────────────────────────────────────────
	"metabase_list_databases":   {"id", "name", "engine", "created_at", "updated_at"},
	"metabase_get_database":     {"id", "name", "engine", "details", "tables", "created_at", "updated_at"},
	"metabase_list_tables":      {"id", "name", "schema", "rows", "entity_type", "db_id"},
	"metabase_get_table":        {"id", "name", "schema", "rows", "entity_type", "db_id", "fields"},
	"metabase_get_table_fields": {"id", "name", "database_type", "base_type", "semantic_type", "position", "description"},

	// ── Cards ────────────────────────────────────────────────────────
	"metabase_list_cards":   {"id", "name", "description", "display", "collection_id", "creator_id", "created_at", "updated_at"},
	"metabase_get_card":     {"id", "name", "description", "display", "dataset_query", "result_metadata", "collection_id", "created_at", "updated_at"},

	// ── Dashboards ───────────────────────────────────────────────────
	"metabase_list_dashboards": {"id", "name", "description", "collection_id", "creator_id", "created_at"},
	"metabase_get_dashboard":   {"id", "name", "description", "dashcards", "collection_id", "parameters", "created_at", "updated_at"},

	// ── Collections ──────────────────────────────────────────────────
	"metabase_list_collections": {"id", "name", "description", "location", "personal_owner_id"},
	"metabase_get_collection":   {"id", "name", "description", "items"},

	// ── Search ───────────────────────────────────────────────────────
	"metabase_search": {"id", "name", "description", "model", "collection.name", "created_at"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("metabase: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
