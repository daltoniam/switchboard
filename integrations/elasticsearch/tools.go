package elasticsearch

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Cluster ---
	{
		Name:        "elasticsearch_cluster_health",
		Description: "Get cluster health status (green/yellow/red), node count, shard counts, and pending tasks",
		Parameters:  map[string]string{},
	},
	{
		Name:        "elasticsearch_cluster_stats",
		Description: "Get cluster-wide statistics including indices count, document count, store size, and node info",
		Parameters:  map[string]string{},
	},
	{
		Name:        "elasticsearch_node_stats",
		Description: "Get statistics for all nodes including JVM heap, CPU, disk, and indexing metrics",
		Parameters: map[string]string{
			"node_id": "Optional node ID or name to filter (omit for all nodes)",
		},
	},
	{
		Name:        "elasticsearch_cat_nodes",
		Description: "Get a compact summary of each node: name, IP, heap, RAM, CPU, load, role, and version",
		Parameters:  map[string]string{},
	},
	{
		Name:        "elasticsearch_pending_tasks",
		Description: "List pending cluster-level tasks (e.g. shard allocation, mapping updates)",
		Parameters:  map[string]string{},
	},

	// --- Indices ---
	{
		Name:        "elasticsearch_list_indices",
		Description: "List all indices with health, status, document count, and store size. Start here for index discovery",
		Parameters: map[string]string{
			"pattern": "Optional index name pattern with wildcards (e.g. 'logs-*'). Omit for all indices",
		},
	},
	{
		Name:        "elasticsearch_get_index",
		Description: "Get full index configuration including settings, mappings, and aliases",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_create_index",
		Description: "Create a new index with optional settings (shards, replicas) and field mappings",
		Parameters: map[string]string{
			"index":    "Index name to create",
			"settings": "Optional JSON object for index settings (e.g. {\"number_of_shards\": 1})",
			"mappings": "Optional JSON object for field mappings (e.g. {\"properties\": {\"title\": {\"type\": \"text\"}}})",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_delete_index",
		Description: "Delete an index and all its data. This action is irreversible",
		Parameters: map[string]string{
			"index": "Index name to delete",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_get_mapping",
		Description: "Get field mappings for an index — shows field names, types, and analyzers",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_put_mapping",
		Description: "Add new fields to an existing index mapping. Cannot change existing field types",
		Parameters: map[string]string{
			"index":      "Index name",
			"properties": "JSON object of field definitions (e.g. {\"new_field\": {\"type\": \"keyword\"}})",
		},
		Required: []string{"index", "properties"},
	},
	{
		Name:        "elasticsearch_get_settings",
		Description: "Get index settings (shards, replicas, refresh interval, analysis config)",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_put_settings",
		Description: "Update dynamic index settings (e.g. number_of_replicas, refresh_interval)",
		Parameters: map[string]string{
			"index":    "Index name",
			"settings": "JSON object of settings to update (e.g. {\"index\": {\"number_of_replicas\": 2}})",
		},
		Required: []string{"index", "settings"},
	},
	{
		Name:        "elasticsearch_index_stats",
		Description: "Get indexing, search, merge, and segment statistics for an index",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_open_index",
		Description: "Open a previously closed index to make it available for search and indexing",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_close_index",
		Description: "Close an index to reduce cluster overhead. Closed indices cannot be searched",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_refresh_index",
		Description: "Refresh an index to make recent changes visible to search immediately",
		Parameters: map[string]string{
			"index": "Index name",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_forcemerge_index",
		Description: "Force merge an index to reduce segment count. Useful for read-only indices",
		Parameters: map[string]string{
			"index":        "Index name",
			"max_segments": "Optional max number of segments to merge down to (default: 1)",
		},
		Required: []string{"index"},
	},

	// --- Documents ---
	{
		Name:        "elasticsearch_get_document",
		Description: "Get a single document by its ID from an index",
		Parameters: map[string]string{
			"index": "Index name",
			"id":    "Document ID",
		},
		Required: []string{"index", "id"},
	},
	{
		Name:        "elasticsearch_index_document",
		Description: "Index (create or replace) a document. If no ID is provided, one is auto-generated",
		Parameters: map[string]string{
			"index":    "Index name",
			"id":       "Optional document ID (auto-generated if omitted)",
			"document": "JSON object — the document body to index",
		},
		Required: []string{"index", "document"},
	},
	{
		Name:        "elasticsearch_update_document",
		Description: "Partially update a document by merging fields. Use 'doc' for partial updates or 'script' for scripted updates",
		Parameters: map[string]string{
			"index":  "Index name",
			"id":     "Document ID",
			"doc":    "JSON object of fields to merge into the existing document",
			"script": "Optional Painless script for scripted updates (e.g. {\"source\": \"ctx._source.count++\"})",
		},
		Required: []string{"index", "id"},
	},
	{
		Name:        "elasticsearch_delete_document",
		Description: "Delete a single document by ID",
		Parameters: map[string]string{
			"index": "Index name",
			"id":    "Document ID",
		},
		Required: []string{"index", "id"},
	},
	{
		Name:        "elasticsearch_bulk",
		Description: "Perform multiple index/create/update/delete operations in a single request. Each action is a JSON object with the operation type as key",
		Parameters: map[string]string{
			"operations": "JSON array of bulk operations. Each element is {\"action\": \"index|create|update|delete\", \"index\": \"...\", \"id\": \"...\", \"doc\": {...}}",
		},
		Required: []string{"operations"},
	},
	{
		Name:        "elasticsearch_mget",
		Description: "Get multiple documents by ID in a single request",
		Parameters: map[string]string{
			"index": "Optional default index name",
			"docs":  "JSON array of {\"_index\": \"...\", \"_id\": \"...\"} objects. _index can be omitted if index param is set",
		},
		Required: []string{"docs"},
	},
	{
		Name:        "elasticsearch_count",
		Description: "Count documents matching a query. Returns total count without fetching documents",
		Parameters: map[string]string{
			"index": "Index name or pattern (e.g. 'logs-*')",
			"query": "Optional Elasticsearch query DSL as JSON (omit to count all documents)",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_delete_by_query",
		Description: "Delete all documents matching a query. Use with caution — this is irreversible",
		Parameters: map[string]string{
			"index": "Index name or pattern",
			"query": "Elasticsearch query DSL as JSON to select documents for deletion",
		},
		Required: []string{"index", "query"},
	},
	{
		Name:        "elasticsearch_update_by_query",
		Description: "Update all documents matching a query using a script",
		Parameters: map[string]string{
			"index":  "Index name or pattern",
			"query":  "Elasticsearch query DSL as JSON to select documents",
			"script": "Painless script for the update (e.g. {\"source\": \"ctx._source.status = 'archived'\"})",
		},
		Required: []string{"index", "query", "script"},
	},
	{
		Name:        "elasticsearch_reindex",
		Description: "Copy documents from one index to another, optionally transforming with a script or filtering with a query. Runs asynchronously — returns a task ID that can be monitored with elasticsearch_list_tasks",
		Parameters: map[string]string{
			"source_index": "Source index name",
			"dest_index":   "Destination index name",
			"query":        "Optional query DSL to filter source documents",
			"script":       "Optional Painless script to transform documents during reindex",
		},
		Required: []string{"source_index", "dest_index"},
	},

	// --- Search ---
	{
		Name:        "elasticsearch_search",
		Description: "Search documents using Elasticsearch Query DSL. Supports full-text search, filters, aggregations, sorting, and highlighting. Start here for querying data",
		Parameters: map[string]string{
			"index":     "Index name or pattern (e.g. 'logs-*')",
			"query":     "Elasticsearch query DSL as JSON (e.g. {\"match\": {\"title\": \"search term\"}})",
			"size":      "Max results to return (default: 10, max: 10000)",
			"from":      "Offset for pagination (default: 0)",
			"sort":      "Optional sort as JSON array (e.g. [{\"timestamp\": \"desc\"}])",
			"aggs":      "Optional aggregations as JSON object",
			"_source":   "Optional source filtering — JSON array of field names to include, or false to exclude _source",
			"highlight": "Optional highlight configuration as JSON object",
		},
		Required: []string{"index"},
	},
	{
		Name:        "elasticsearch_msearch",
		Description: "Execute multiple searches in a single request. More efficient than individual search calls",
		Parameters: map[string]string{
			"searches": "JSON array of search objects, each with optional 'index' and required 'body' (query DSL)",
		},
		Required: []string{"searches"},
	},
	{
		Name:        "elasticsearch_sql_query",
		Description: "Execute a SQL query against Elasticsearch using the SQL plugin. Useful for analysts familiar with SQL",
		Parameters: map[string]string{
			"query":  "SQL query string (e.g. \"SELECT * FROM my_index WHERE status = 'active' LIMIT 10\")",
			"format": "Response format: json (default), csv, tsv, txt, yaml",
		},
		Required: []string{"query"},
	},

	// --- Aliases ---
	{
		Name:        "elasticsearch_list_aliases",
		Description: "List all index aliases or aliases for a specific index",
		Parameters: map[string]string{
			"index": "Optional index name to filter aliases",
		},
	},
	{
		Name:        "elasticsearch_update_aliases",
		Description: "Atomically add or remove index aliases. Useful for zero-downtime reindexing",
		Parameters: map[string]string{
			"actions": "JSON array of alias actions (e.g. [{\"add\": {\"index\": \"idx-v2\", \"alias\": \"idx\"}}, {\"remove\": {\"index\": \"idx-v1\", \"alias\": \"idx\"}}])",
		},
		Required: []string{"actions"},
	},

	// --- Templates ---
	{
		Name:        "elasticsearch_list_index_templates",
		Description: "List all index templates with their patterns, priority, and settings",
		Parameters:  map[string]string{},
	},
	{
		Name:        "elasticsearch_get_index_template",
		Description: "Get a specific index template definition including patterns, settings, mappings, and aliases",
		Parameters: map[string]string{
			"name": "Template name",
		},
		Required: []string{"name"},
	},

	// --- Snapshot / Restore ---
	{
		Name:        "elasticsearch_list_snapshot_repos",
		Description: "List registered snapshot repositories and their types",
		Parameters:  map[string]string{},
	},
	{
		Name:        "elasticsearch_list_snapshots",
		Description: "List snapshots in a repository with status, indices, and timing",
		Parameters: map[string]string{
			"repository": "Snapshot repository name",
		},
		Required: []string{"repository"},
	},

	// --- Tasks ---
	{
		Name:        "elasticsearch_list_tasks",
		Description: "List currently running tasks (reindex, update-by-query, etc.) with progress info",
		Parameters: map[string]string{
			"actions": "Optional comma-separated action patterns to filter (e.g. '*reindex*')",
		},
	},
	{
		Name:        "elasticsearch_cancel_task",
		Description: "Cancel a running task by its task ID",
		Parameters: map[string]string{
			"task_id": "Task ID (e.g. 'node_id:task_number')",
		},
		Required: []string{"task_id"},
	},

	// --- Cat APIs ---
	{
		Name:        "elasticsearch_cat_shards",
		Description: "Get shard allocation details: index, shard number, primary/replica, state, size, node",
		Parameters: map[string]string{
			"index": "Optional index name to filter",
		},
	},
	{
		Name:        "elasticsearch_cat_allocation",
		Description: "Get disk allocation per node: shards, disk used/available, host, IP",
		Parameters:  map[string]string{},
	},
}

var dispatch = map[string]handlerFunc{
	// Cluster
	"elasticsearch_cluster_health": clusterHealth,
	"elasticsearch_cluster_stats":  clusterStats,
	"elasticsearch_node_stats":     nodeStats,
	"elasticsearch_cat_nodes":      catNodes,
	"elasticsearch_pending_tasks":  pendingTasks,

	// Indices
	"elasticsearch_list_indices":     listIndices,
	"elasticsearch_get_index":        getIndex,
	"elasticsearch_create_index":     createIndex,
	"elasticsearch_delete_index":     deleteIndex,
	"elasticsearch_get_mapping":      getMapping,
	"elasticsearch_put_mapping":      putMapping,
	"elasticsearch_get_settings":     getSettings,
	"elasticsearch_put_settings":     putSettings,
	"elasticsearch_index_stats":      indexStats,
	"elasticsearch_open_index":       openIndex,
	"elasticsearch_close_index":      closeIndex,
	"elasticsearch_refresh_index":    refreshIndex,
	"elasticsearch_forcemerge_index": forcemergeIndex,

	// Documents
	"elasticsearch_get_document":    getDocument,
	"elasticsearch_index_document":  indexDocument,
	"elasticsearch_update_document": updateDocument,
	"elasticsearch_delete_document": deleteDocument,
	"elasticsearch_bulk":            bulkOp,
	"elasticsearch_mget":            mget,
	"elasticsearch_count":           countDocs,
	"elasticsearch_delete_by_query": deleteByQuery,
	"elasticsearch_update_by_query": updateByQuery,
	"elasticsearch_reindex":         reindex,

	// Search
	"elasticsearch_search":    search,
	"elasticsearch_msearch":   msearch,
	"elasticsearch_sql_query": sqlQuery,

	// Aliases
	"elasticsearch_list_aliases":   listAliases,
	"elasticsearch_update_aliases": updateAliases,

	// Templates
	"elasticsearch_list_index_templates": listIndexTemplates,
	"elasticsearch_get_index_template":   getIndexTemplate,

	// Snapshots
	"elasticsearch_list_snapshot_repos": listSnapshotRepos,
	"elasticsearch_list_snapshots":      listSnapshots,

	// Tasks
	"elasticsearch_list_tasks":  listTasks,
	"elasticsearch_cancel_task": cancelTask,

	// Cat
	"elasticsearch_cat_shards":     catShards,
	"elasticsearch_cat_allocation": catAllocation,
}
