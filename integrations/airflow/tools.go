package airflow

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(context.Context, *airflow, map[string]any) (*mcp.ToolResult, error)

var paging = map[string]string{"limit": "Results per page, 1-100 (default 25)", "offset": "Zero-based page offset (default 0)"}

var tools = []mcp.ToolDefinition{
	{Name: "airflow_list_dags", Description: "List Apache Airflow 3 pipeline DAGs. Start here to discover scheduled workflows and diagnose failed jobs.", Parameters: paging},
	{Name: "airflow_get_dag", Description: "Get metadata and status of one Airflow DAG by ID. Use after list_dags.", Parameters: map[string]string{"dag_id": "Exact DAG ID"}, Required: []string{"dag_id"}},
	{Name: "airflow_list_dag_runs", Description: "List executions of an Airflow DAG to find failed or stalled pipeline runs. Use after list_dags.", Parameters: map[string]string{"dag_id": "Exact DAG ID", "limit": "Results per page, 1-100 (default 25)", "offset": "Zero-based page offset"}, Required: []string{"dag_id"}},
	{Name: "airflow_get_dag_run", Description: "Get the state and timing of an Airflow pipeline run. Use after list_dag_runs.", Parameters: map[string]string{"dag_id": "Exact DAG ID", "dag_run_id": "Exact run ID"}, Required: []string{"dag_id", "dag_run_id"}},
	{Name: "airflow_list_task_instances", Description: "List task instance states within one Airflow DAG run to diagnose pipeline failures. Use after get_dag_run.", Parameters: map[string]string{"dag_id": "Exact DAG ID", "dag_run_id": "Exact run ID", "limit": "Results per page, 1-100 (default 25)", "offset": "Zero-based page offset"}, Required: []string{"dag_id", "dag_run_id"}},
	{Name: "airflow_trigger_dag", Description: "Trigger a single new Airflow DAG run; this starts real pipeline work. Use after get_dag to verify the exact DAG ID. Optional conf is a JSON object.", Parameters: map[string]string{"dag_id": "Exact DAG ID", "logical_date": "Optional RFC3339 logical date; omitted sends null", "conf": "Optional JSON object for run configuration"}, Required: []string{"dag_id"}},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"airflow_list_dags":           list("/api/v2/dags"),
	"airflow_get_dag":             read("/api/v2/dags/{}", "dag_id"),
	"airflow_list_dag_runs":       list("/api/v2/dags/{}/dagRuns", "dag_id"),
	"airflow_get_dag_run":         read("/api/v2/dags/{}/dagRuns/{}", "dag_id", "dag_run_id"),
	"airflow_list_task_instances": list("/api/v2/dags/{}/dagRuns/{}/taskInstances", "dag_id", "dag_run_id"),
	"airflow_trigger_dag":         trigger,
}
