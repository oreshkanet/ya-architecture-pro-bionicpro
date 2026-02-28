"""
DAG инициализации CRM БД: создание таблиц из sql/crm_init.sql и загрузка sample-данных
из data/customers.csv и data/orders.csv.
Подключение: crm_postgres (AIRFLOW_CONN_CRM_POSTGRES).
"""
import csv
import os
from datetime import datetime

from airflow import DAG
from airflow.providers.postgres.hooks.postgres import PostgresHook
from airflow.operators.python import PythonOperator

DAG_DIR = os.path.dirname(os.path.abspath(__file__))
CRM_CONN_ID = "crm_postgres"


def run_sql_file(filepath: str, conn_id: str):
    """Выполнить SQL-скрипт из файла в указанном подключении."""
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


def init_crm_schema():
    """Создать таблицы CRM из sql/crm_init.sql."""
    run_sql_file(os.path.join(DAG_DIR, "sql", "crm_init.sql"), CRM_CONN_ID)


def load_customers():
    """Загрузить данные из data/customers.csv в таблицу customers."""
    path = os.path.join(DAG_DIR, "data", "customers.csv")
    hook = PostgresHook(postgres_conn_id=CRM_CONN_ID)
    conn = hook.get_conn()
    cur = conn.cursor()
    try:
        with open(path, "r", encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                cur.execute(
                    """
                    INSERT INTO customers (user_id, full_name, email)
                    VALUES (%(user_id)s, %(full_name)s, %(email)s)
                    ON CONFLICT (user_id) DO UPDATE SET
                        full_name = EXCLUDED.full_name,
                        email = EXCLUDED.email
                    """,
                    row,
                )
        conn.commit()
    finally:
        cur.close()
        conn.close()


def load_orders():
    """Загрузить данные из data/orders.csv в таблицу orders."""
    path = os.path.join(DAG_DIR, "data", "orders.csv")
    hook = PostgresHook(postgres_conn_id=CRM_CONN_ID)
    conn = hook.get_conn()
    cur = conn.cursor()
    try:
        with open(path, "r", encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                cur.execute(
                    """
                    INSERT INTO orders (order_number, total, discount, buyer_id)
                    VALUES (%(order_number)s, %(total)s, %(discount)s, %(buyer_id)s)
                    ON CONFLICT (order_number) DO UPDATE SET
                        total = EXCLUDED.total,
                        discount = EXCLUDED.discount,
                        buyer_id = EXCLUDED.buyer_id
                    """,
                    row,
                )
        conn.commit()
    finally:
        cur.close()
        conn.close()


with DAG(
    dag_id="dag_crm_init",
    description="Инициализация CRM БД: схема + sample customers и orders",
    schedule_interval="@once",
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=["init", "crm"],
) as dag:
    create_tables = PythonOperator(
        task_id="create_tables",
        python_callable=init_crm_schema,
    )
    load_customers_task = PythonOperator(
        task_id="load_customers",
        python_callable=load_customers,
    )
    load_orders_task = PythonOperator(
        task_id="load_orders",
        python_callable=load_orders,
    )
    create_tables >> load_customers_task >> load_orders_task
