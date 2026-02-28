"""
DAG инициализации БД телеметрии: создание таблиц из sql/telemetry_init.sql
и загрузка sample-данных из data/telemetry_events.csv.
Подключение: telemetry_postgres (AIRFLOW_CONN_TELEMETRY_POSTGRES).
"""
import csv
import os
from datetime import datetime

from airflow import DAG
from airflow.providers.postgres.hooks.postgres import PostgresHook
from airflow.operators.python import PythonOperator

DAG_DIR = os.path.dirname(os.path.abspath(__file__))
TELEMETRY_CONN_ID = "telemetry_postgres"


def run_sql_file(filepath: str, conn_id: str):
    """Выполнить SQL-скрипт из файла."""
    with open(filepath, "r", encoding="utf-8") as f:
        sql = f.read()
    hook = PostgresHook(postgres_conn_id=conn_id)
    conn = hook.get_conn()
    conn.autocommit = True
    cur = conn.cursor()
    try:
        cur.execute(sql)
    finally:
        cur.close()
        conn.close()


def init_telemetry_schema():
    """Создать таблицы из sql/telemetry_init.sql."""
    run_sql_file(os.path.join(DAG_DIR, "sql", "telemetry_init.sql"), TELEMETRY_CONN_ID)


def load_telemetry_events():
    """Загрузить данные из data/telemetry_events.csv в telemetry_events."""
    path = os.path.join(DAG_DIR, "data", "telemetry_events.csv")
    hook = PostgresHook(postgres_conn_id=TELEMETRY_CONN_ID)
    conn = hook.get_conn()
    cur = conn.cursor()
    try:
        with open(path, "r", encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                cur.execute(
                    """
                    INSERT INTO telemetry_events (user_id, device_id, event_type, value)
                    VALUES (%(user_id)s, %(device_id)s, %(event_type)s, %(value)s)
                    """,
                    row,
                )
        conn.commit()
    finally:
        cur.close()
        conn.close()


with DAG(
    dag_id="dag_telemetry_init",
    description="Инициализация БД телеметрии: схема + sample telemetry_events",
    schedule_interval="@once",
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=["init", "telemetry"],
) as dag:
    create_tables = PythonOperator(
        task_id="create_tables",
        python_callable=init_telemetry_schema,
    )
    load_telemetry_task = PythonOperator(
        task_id="load_telemetry_events",
        python_callable=load_telemetry_events,
    )
    create_tables >> load_telemetry_task
