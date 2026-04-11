package elasticsearch

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Cluster ──────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_cluster_health"): {"cluster_name", "status", "number_of_nodes", "number_of_data_nodes", "active_primary_shards", "active_shards", "relocating_shards", "initializing_shards", "unassigned_shards", "pending_tasks"},
	mcp.ToolName("elasticsearch_cluster_stats"):  {"cluster_name", "status", "nodes.count", "nodes.jvm.mem", "nodes.fs", "indices.count", "indices.docs", "indices.store"},
	mcp.ToolName("elasticsearch_node_stats"):     {"nodes.*"},
	mcp.ToolName("elasticsearch_cat_nodes"):      {"name", "ip", "heap.percent", "ram.percent", "cpu", "load_1m", "node.role", "master", "version"},
	mcp.ToolName("elasticsearch_pending_tasks"):  {"tasks"},

	// ── Indices ──────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_list_indices"): {"index", "health", "status", "docs.count", "store.size", "pri", "rep"},
	mcp.ToolName("elasticsearch_index_stats"):  {"_all.total.indexing", "_all.total.search", "_all.total.store", "_all.total.docs"},
	mcp.ToolName("elasticsearch_get_mapping"):  {"-defaults"},
	mcp.ToolName("elasticsearch_get_settings"): {"-defaults"},

	// ── Documents ────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_get_document"): {"_index", "_id", "_version", "found", "_source"},
	mcp.ToolName("elasticsearch_mget"):         {"docs[]._index", "docs[]._id", "docs[].found", "docs[]._source"},
	mcp.ToolName("elasticsearch_count"):        {"count"},

	// ── Search ───────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_search"):  {"hits.total", "hits.hits[]._index", "hits.hits[]._id", "hits.hits[]._score", "hits.hits[]._source", "hits.hits[].highlight", "aggregations"},
	mcp.ToolName("elasticsearch_msearch"): {"responses[].hits.total", "responses[].hits.hits[]._index", "responses[].hits.hits[]._id", "responses[].hits.hits[]._score", "responses[].hits.hits[]._source", "responses[].aggregations"},

	// ── Aliases ──────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_list_aliases"): {"alias", "index", "filter", "is_write_index"},

	// ── Templates ────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_list_index_templates"): {"index_templates[].name", "index_templates[].index_template.index_patterns", "index_templates[].index_template.priority"},
	mcp.ToolName("elasticsearch_get_index_template"):   {"index_templates[].name", "index_templates[].index_template.index_patterns", "index_templates[].index_template.priority", "index_templates[].index_template.template"},

	// ── Snapshots ────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_list_snapshots"): {"snapshots[].snapshot", "snapshots[].state", "snapshots[].indices", "snapshots[].start_time", "snapshots[].end_time", "snapshots[].duration_in_millis"},

	// ── Cat ──────────────────────────────────────────────────────────
	mcp.ToolName("elasticsearch_cat_shards"):     {"index", "shard", "prirep", "state", "docs", "store", "node"},
	mcp.ToolName("elasticsearch_cat_allocation"): {"shards", "disk.indices", "disk.used", "disk.avail", "disk.percent", "node"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("elasticsearch: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
