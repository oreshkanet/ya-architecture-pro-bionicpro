"""
DAG инициализации OLAP БД: создание таблиц из sql/olap_init.sql
и загрузка sample-данных из data/report_mart.csv.
Подключение: olap_postgres (AIRFLOW_CONN_OLAP_POSTGRES).
"""
import csv
import os
from datetime import datetime

from airflow import DAG
from airflow.providers.postgres.hooks.postgres import PostgresHook
from airflow.operators.python import PythonOperator

DAG_DIR = os.path.dirname(os.path.abspath(__file__))
OLAP_CONN_ID = "olap_postgres"


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


def init_olap_schema():
    """Создать таблицы из sql/olap_init.sql."""
    run_sql_file(os.path.join(DAG_DIR, "sql", "olap_init.sql"), OLAP_CONN_ID)

with DAG(
    dag_id="dag_olap_init",
    description="Инициализация OLAP БД: схема + sample report_mart",
    schedule_interval="@once",
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=["init", "olap"],
) as dag:
    create_tables = PythonOperator(
        task_id="create_tables",
        python_callable=init_olap_schema,
    )
    
    create_tables
