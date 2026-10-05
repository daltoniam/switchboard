"""Minimal DAG for Switchboard Airflow adapter E2E proof."""

from datetime import datetime, timedelta

from airflow import DAG
from airflow.providers.standard.operators.empty import EmptyOperator

with DAG(
    dag_id="switchboard_proof",
    description="Tiny DAG for Switchboard native REST E2E",
    schedule=None,
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=["switchboard", "e2e"],
) as dag:
    done = EmptyOperator(task_id="done", retries=0)
