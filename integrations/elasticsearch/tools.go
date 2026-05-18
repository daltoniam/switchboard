package elasticsearch

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Cluster ---
	{
		Name:        mcp.ToolName("elasticsearch_cluster_health"),
		Description: "Get cluster health status (green/yellow/red), node count, shard counts, and pending tasks",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("elasticsearch_cluster_stats"),
		Description: "Get cluster-wide statistics including indices count, document count, store size, and node info",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("elasticsearch_node_stats"),
		Description: "Get statistics for all nodes including JVM heap, CPU, disk, and indexing metrics",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("node_id"), Description: "Optional node ID or name to filter (omit for all nodes)"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_cat_nodes"),
		Description: "Get a compact summary of each node: name, IP, heap, RAM, CPU, load, role, and version",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("elasticsearch_pending_tasks"),
		Description: "List pending cluster-level tasks (e.g. shard allocation, mapping updates)",
		Parameters:  []mcp.Parameter{},
	},

	// --- Indices ---
	{
		Name:        mcp.ToolName("elasticsearch_list_indices"),
		Description: "List all indices with health, status, document count, and store size. Start here for index discovery",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("pattern"), Description: "Optional index name pattern with wildcards (e.g. 'logs-*'). Omit for all indices"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_get_index"),
		Description: "Get full index configuration including settings, mappings, and aliases",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_create_index"),
		Description: "Create a new index with optional settings (shards, replicas) and field mappings",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name to create", Required: true}, {Name: mcp.ParamName("settings"), Description: `Optional JSON object for index settings (e.g. {"number_of_shards": 1})`}, {Name: mcp.ParamName("mappings"), Description: `Optional JSON object for field mappings (e.g. {"properties": {"title": {"type": "text"}}})`}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_delete_index"),
		Description: "Delete an index and all its data. This action is irreversible",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name to delete", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_get_mapping"),
		Description: "Get field mappings for an index — shows field names, types, and analyzers",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_put_mapping"),
		Description: "Add new fields to an existing index mapping. Cannot change existing field types",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("properties"), Description: `JSON object of field definitions (e.g. {"new_field": {"type": "keyword"}})`, Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_get_settings"),
		Description: "Get index settings (shards, replicas, refresh interval, analysis config)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_put_settings"),
		Description: "Update dynamic index settings (e.g. number_of_replicas, refresh_interval)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("settings"), Description: `JSON object of settings to update (e.g. {"index": {"number_of_replicas": 2}})`, Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_index_stats"),
		Description: "Get indexing, search, merge, and segment statistics for an index",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_open_index"),
		Description: "Open a previously closed index to make it available for search and indexing",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_close_index"),
		Description: "Close an index to reduce cluster overhead. Closed indices cannot be searched",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_refresh_index"),
		Description: "Refresh an index to make recent changes visible to search immediately",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_forcemerge_index"),
		Description: "Force merge an index to reduce segment count. Useful for read-only indices",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("max_segments"), Description: "Optional max number of segments to merge down to (default: 1)"}},
	},

	// --- Documents ---
	{
		Name:        mcp.ToolName("elasticsearch_get_document"),
		Description: "Get a single document by its ID from an index",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("id"), Description: "Document ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_index_document"),
		Description: "Index (create or replace) a document. If no ID is provided, one is auto-generated",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("id"), Description: "Optional document ID (auto-generated if omitted)"}, {Name: mcp.ParamName("document"), Description: "JSON object — the document body to index", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_update_document"),
		Description: "Partially update a document by merging fields. Use 'doc' for partial updates or 'script' for scripted updates",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("id"), Description: "Document ID", Required: true}, {Name: mcp.ParamName("doc"), Description: "JSON object of fields to merge into the existing document"}, {Name: mcp.ParamName("script"), Description: `Optional Painless script for scripted updates (e.g. {"source": "ctx._source.count++"})`}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_delete_document"),
		Description: "Delete a single document by ID",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name", Required: true}, {Name: mcp.ParamName("id"), Description: "Document ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_bulk"),
		Description: "Perform multiple index/create/update/delete operations in a single request. Each action is a JSON object with the operation type as key",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("operations"), Description: `JSON array of bulk operations. Each element is {"action": "index|create|update|delete", "index": "...", "id": "...", "doc": {...}}`, Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_mget"),
		Description: "Get multiple documents by ID in a single request",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Optional default index name"}, {Name: mcp.ParamName("docs"), Description: `JSON array of {"_index": "...", "_id": "..."} objects. _index can be omitted if index param is set`, Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_count"),
		Description: "Count documents matching a query. Returns total count without fetching documents",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name or pattern (e.g. 'logs-*')", Required: true}, {Name: mcp.ParamName("query"), Description: "Optional Elasticsearch query DSL as JSON (omit to count all documents)"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_delete_by_query"),
		Description: "Delete all documents matching a query. Use with caution — this is irreversible",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name or pattern", Required: true}, {Name: mcp.ParamName("query"), Description: "Elasticsearch query DSL as JSON to select documents for deletion", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_update_by_query"),
		Description: "Update all documents matching a query using a script",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name or pattern", Required: true}, {Name: mcp.ParamName("query"), Description: "Elasticsearch query DSL as JSON to select documents", Required: true}, {Name: mcp.ParamName("script"), Description: `Painless script for the update (e.g. {"source": "ctx._source.status = 'archived'"})`, Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_reindex"),
		Description: "Copy documents from one index to another, optionally transforming with a script or filtering with a query. Runs asynchronously — returns a task ID that can be monitored with elasticsearch_list_tasks",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("source_index"), Description: "Source index name", Required: true}, {Name: mcp.ParamName("dest_index"), Description: "Destination index name", Required: true}, {Name: mcp.ParamName("query"), Description: "Optional query DSL to filter source documents"}, {Name: mcp.ParamName("script"), Description: "Optional Painless script to transform documents during reindex"}},
	},

	// --- Search ---
	{
		Name:        mcp.ToolName("elasticsearch_search"),
		Description: "Search documents using Elasticsearch Query DSL. Supports full-text search, filters, aggregations, sorting, and highlighting. Start here for querying data",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Index name or pattern (e.g. 'logs-*')", Required: true}, {Name: mcp.ParamName("query"), Description: `Elasticsearch query DSL as JSON (e.g. {"match": {"title": "search term"}})`}, {Name: mcp.ParamName("size"), Description: "Max results to return (default: 10, max: 10000)"}, {Name: mcp.ParamName("from"), Description: "Offset for pagination (default: 0)"}, {Name: mcp.ParamName("sort"), Description: `Optional sort as JSON array (e.g. [{"timestamp": "desc"}])`}, {Name: mcp.ParamName("aggs"), Description: "Optional aggregations as JSON object"}, {Name: mcp.ParamName("_source"), Description: "Optional source filtering — JSON array of field names to include, or false to exclude _source"}, {Name: mcp.ParamName("highlight"), Description: "Optional highlight configuration as JSON object"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_msearch"),
		Description: "Execute multiple searches in a single request. More efficient than individual search calls",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("searches"), Description: "JSON array of search objects, each with optional 'index' and required 'body' (query DSL)", Required: true}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_sql_query"),
		Description: "Execute a SQL query against Elasticsearch using the SQL plugin. Useful for analysts familiar with SQL",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: `SQL query string (e.g. "SELECT * FROM my_index WHERE status = 'active' LIMIT 10")`, Required: true}, {Name: mcp.ParamName("format"), Description: "Response format: json (default), csv, tsv, txt, yaml"}},
	},

	// --- Aliases ---
	{
		Name:        mcp.ToolName("elasticsearch_list_aliases"),
		Description: "List all index aliases or aliases for a specific index",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Optional index name to filter aliases"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_update_aliases"),
		Description: "Atomically add or remove index aliases. Useful for zero-downtime reindexing",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("actions"), Description: `JSON array of alias actions (e.g. [{"add": {"index": "idx-v2", "alias": "idx"}}, {"remove": {"index": "idx-v1", "alias": "idx"}}])`, Required: true}},
	},

	// --- Templates ---
	{
		Name:        mcp.ToolName("elasticsearch_list_index_templates"),
		Description: "List all index templates with their patterns, priority, and settings",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("elasticsearch_get_index_template"),
		Description: "Get a specific index template definition including patterns, settings, mappings, and aliases",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Template name", Required: true}},
	},

	// --- Snapshot / Restore ---
	{
		Name:        mcp.ToolName("elasticsearch_list_snapshot_repos"),
		Description: "List registered snapshot repositories and their types",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("elasticsearch_list_snapshots"),
		Description: "List snapshots in a repository with status, indices, and timing",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("repository"), Description: "Snapshot repository name", Required: true}},
	},

	// --- Tasks ---
	{
		Name:        mcp.ToolName("elasticsearch_list_tasks"),
		Description: "List currently running tasks (reindex, update-by-query, etc.) with progress info",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("actions"), Description: "Optional comma-separated action patterns to filter (e.g. '*reindex*')"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_cancel_task"),
		Description: "Cancel a running task by its task ID",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "Task ID (e.g. 'node_id:task_number')", Required: true}},
	},

	// --- Cat APIs ---
	{
		Name:        mcp.ToolName("elasticsearch_cat_shards"),
		Description: "Get shard allocation details: index, shard number, primary/replica, state, size, node",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("index"), Description: "Optional index name to filter"}},
	},
	{
		Name:        mcp.ToolName("elasticsearch_cat_allocation"),
		Description: "Get disk allocation per node: shards, disk used/available, host, IP",
		Parameters:  []mcp.Parameter{},
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	// Cluster
	mcp.ToolName("elasticsearch_cluster_health"): clusterHealth,
	mcp.ToolName("elasticsearch_cluster_stats"):  clusterStats,
	mcp.ToolName("elasticsearch_node_stats"):     nodeStats,
	mcp.ToolName("elasticsearch_cat_nodes"):      catNodes,
	mcp.ToolName("elasticsearch_pending_tasks"):  pendingTasks,

	// Indices
	mcp.ToolName("elasticsearch_list_indices"):     listIndices,
	mcp.ToolName("elasticsearch_get_index"):        getIndex,
	mcp.ToolName("elasticsearch_create_index"):     createIndex,
	mcp.ToolName("elasticsearch_delete_index"):     deleteIndex,
	mcp.ToolName("elasticsearch_get_mapping"):      getMapping,
	mcp.ToolName("elasticsearch_put_mapping"):      putMapping,
	mcp.ToolName("elasticsearch_get_settings"):     getSettings,
	mcp.ToolName("elasticsearch_put_settings"):     putSettings,
	mcp.ToolName("elasticsearch_index_stats"):      indexStats,
	mcp.ToolName("elasticsearch_open_index"):       openIndex,
	mcp.ToolName("elasticsearch_close_index"):      closeIndex,
	mcp.ToolName("elasticsearch_refresh_index"):    refreshIndex,
	mcp.ToolName("elasticsearch_forcemerge_index"): forcemergeIndex,

	// Documents
	mcp.ToolName("elasticsearch_get_document"):    getDocument,
	mcp.ToolName("elasticsearch_index_document"):  indexDocument,
	mcp.ToolName("elasticsearch_update_document"): updateDocument,
	mcp.ToolName("elasticsearch_delete_document"): deleteDocument,
	mcp.ToolName("elasticsearch_bulk"):            bulkOp,
	mcp.ToolName("elasticsearch_mget"):            mget,
	mcp.ToolName("elasticsearch_count"):           countDocs,
	mcp.ToolName("elasticsearch_delete_by_query"): deleteByQuery,
	mcp.ToolName("elasticsearch_update_by_query"): updateByQuery,
	mcp.ToolName("elasticsearch_reindex"):         reindex,

	// Search
	mcp.ToolName("elasticsearch_search"):    search,
	mcp.ToolName("elasticsearch_msearch"):   msearch,
	mcp.ToolName("elasticsearch_sql_query"): sqlQuery,

	// Aliases
	mcp.ToolName("elasticsearch_list_aliases"):   listAliases,
	mcp.ToolName("elasticsearch_update_aliases"): updateAliases,

	// Templates
	mcp.ToolName("elasticsearch_list_index_templates"): listIndexTemplates,
	mcp.ToolName("elasticsearch_get_index_template"):   getIndexTemplate,

	// Snapshots
	mcp.ToolName("elasticsearch_list_snapshot_repos"): listSnapshotRepos,
	mcp.ToolName("elasticsearch_list_snapshots"):      listSnapshots,

	// Tasks
	mcp.ToolName("elasticsearch_list_tasks"):  listTasks,
	mcp.ToolName("elasticsearch_cancel_task"): cancelTask,

	// Cat
	mcp.ToolName("elasticsearch_cat_shards"):     catShards,
	mcp.ToolName("elasticsearch_cat_allocation"): catAllocation,
}
