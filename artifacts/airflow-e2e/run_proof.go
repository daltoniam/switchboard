//go:build ignore

// One-off E2E proof runner for the Airflow adapter (native REST via integrations/airflow).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/airflow"
)

func main() {
	base := os.Getenv("AIRFLOW_BASE_URL")
	user := os.Getenv("AIRFLOW_USERNAME")
	pass := os.Getenv("AIRFLOW_PASSWORD")
	if base == "" || user == "" || pass == "" {
		fmt.Fprintln(os.Stderr, "set AIRFLOW_BASE_URL, AIRFLOW_USERNAME, AIRFLOW_PASSWORD")
		os.Exit(2)
	}

	a := airflow.New()
	if err := a.Configure(context.Background(), mcp.Credentials{
		"base_url": base,
		"username": user,
		"password": pass,
	}); err != nil {
		fail("configure", err)
	}
	if !a.Healthy(context.Background()) {
		fail("healthy", fmt.Errorf("integration not healthy against %s", base))
	}
	fmt.Println("OK healthy")

	run := func(name mcp.ToolName, args map[string]any) string {
		fmt.Printf("\n==> %s %v\n", name, args)
		res, err := a.Execute(context.Background(), name, args)
		if err != nil {
			fail(string(name), err)
		}
		if res.IsError {
			fail(string(name), fmt.Errorf("%s", res.Data))
		}
		fmt.Println(res.Data)
		return res.Data
	}

	list := run("airflow_list_dags", map[string]any{"limit": 10})
	if !strings.Contains(list, "switchboard_proof") {
		fail("airflow_list_dags", fmt.Errorf("expected switchboard_proof in response"))
	}

	run("airflow_get_dag", map[string]any{"dag_id": "switchboard_proof"})

	triggered := run("airflow_trigger_dag", map[string]any{
		"dag_id": "switchboard_proof",
		"conf":   map[string]any{"proof": "switchboard-e2e"},
	})
	var runMeta map[string]any
	if err := json.Unmarshal([]byte(triggered), &runMeta); err != nil {
		fail("airflow_trigger_dag", err)
	}
	dagRunID, _ := runMeta["dag_run_id"].(string)
	if dagRunID == "" {
		fail("airflow_trigger_dag", fmt.Errorf("missing dag_run_id"))
	}

	run("airflow_list_dag_runs", map[string]any{"dag_id": "switchboard_proof", "limit": 5})
	run("airflow_get_dag_run", map[string]any{"dag_id": "switchboard_proof", "dag_run_id": dagRunID})

	// Task instances may appear after a short scheduler delay.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for {
		out := run("airflow_list_task_instances", map[string]any{
			"dag_id":     "switchboard_proof",
			"dag_run_id": dagRunID,
			"limit":      10,
		})
		if strings.Contains(out, "done") {
			break
		}
		select {
		case <-ctx.Done():
			fail("airflow_list_task_instances", fmt.Errorf("timed out waiting for task instance"))
		case <-time.After(3 * time.Second):
		}
	}
	fmt.Println("\nALL STEPS PASSED")
}

func fail(step string, err error) {
	fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", step, err)
	os.Exit(1)
}
