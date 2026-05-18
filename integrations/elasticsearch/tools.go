package elasticsearch

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

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
