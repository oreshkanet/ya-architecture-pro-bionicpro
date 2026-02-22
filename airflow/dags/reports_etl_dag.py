"""
ETL DAG: подготовка витрины отчётности для сервиса отчётов.
Объединяет данные CRM (customers) и телеметрии (telemetry_events из отдельной БД)
в OLAP-витрину report_mart.
Структура: customers.user_id, telemetry_events в telemetry_db, report_mart в olap_db.
"""
from datetime import datetime, timedelta

from airflow import DAG
from airflow.providers.postgres.hooks.postgres import PostgresHook
from airflow.operators.python import PythonOperator

CRM_PG_CONN_ID = "crm_postgres"
TELEMETRY_PG_CONN_ID = "telemetry_postgres"
OLAP_PG_CONN_ID = "olap_postgres"


def extract_and_load(**context):
    """Извлечь данные из CRM и телеметрии (разные БД), объединить и загрузить в OLAP report_mart."""
    crm_hook = PostgresHook(postgres_conn_id=CRM_PG_CONN_ID)
    telemetry_hook = PostgresHook(postgres_conn_id=TELEMETRY_PG_CONN_ID)
    olap_hook = PostgresHook(postgres_conn_id=OLAP_PG_CONN_ID)

    period_end = datetime.utcnow().date()
    period_start = period_end - timedelta(days=30)
    params = {"period_start": period_start, "period_end": period_end}

    # 1. Клиенты из CRM (таблица customers: user_id, full_name, email, registered_at)
    conn_crm = crm_hook.get_conn()
    cur_crm = conn_crm.cursor()
    cur_crm.execute(
        """
        SELECT user_id, full_name, email, registered_at
        FROM customers
        """
    )
    customers = {row[0]: {"full_name": row[1], "email": row[2], "registered_at": row[3]} for row in cur_crm.fetchall()}
    cur_crm.close()
    conn_crm.close()

    # 2. Агрегация телеметрии по user_id (БД телеметрии: telemetry_events)
    conn_telem = telemetry_hook.get_conn()
    cur_telem = conn_telem.cursor()
    cur_telem.execute(
        """
        SELECT
            user_id,
            COUNT(*)::INTEGER AS session_count,
            COALESCE(SUM(CASE WHEN event_type = 'usage_hours' THEN value ELSE 0 END), 0) AS usage_hours,
            COALESCE(
                SUM(CASE WHEN event_type = 'usage_hours' THEN value ELSE 0 END)
                / NULLIF(COUNT(DISTINCT created_at::date), 0),
                0
            ) AS avg_daily_use
        FROM telemetry_events
        WHERE created_at >= %(period_start)s AND created_at < %(period_end)s + INTERVAL '1 day'
        GROUP BY user_id
        """,
        params,
    )
    telemetry_agg = {row[0]: {"session_count": row[1], "usage_hours": row[2], "avg_daily_use": row[3]} for row in cur_telem.fetchall()}
    cur_telem.close()
    conn_telem.close()

    # 3. Объединение: по каждому клиенту из CRM добавляем агрегаты телеметрии
    olap_conn = olap_hook.get_conn()
    olap_cur = olap_conn.cursor()
    for user_id, cust in customers.items():
        t = telemetry_agg.get(user_id, {})
        session_count = t.get("session_count") or 0
        usage_hours = t.get("usage_hours") or 0
        avg_daily_use = t.get("avg_daily_use") or 0
        olap_cur.execute(
            """
            INSERT INTO report_mart (
                user_id, full_name, email, registered_at,
                period_start, period_end, usage_hours, session_count, avg_daily_use, updated_at
            ) VALUES (
                %(user_id)s, %(full_name)s, %(email)s, %(registered_at)s,
                %(period_start)s, %(period_end)s, %(usage_hours)s, %(session_count)s, %(avg_daily_use)s, NOW()
            )
            ON CONFLICT (user_id) DO UPDATE SET
                full_name = EXCLUDED.full_name,
                email = EXCLUDED.email,
                registered_at = EXCLUDED.registered_at,
                period_start = EXCLUDED.period_start,
                period_end = EXCLUDED.period_end,
                usage_hours = EXCLUDED.usage_hours,
                session_count = EXCLUDED.session_count,
                avg_daily_use = EXCLUDED.avg_daily_use,
                updated_at = NOW()
            """,
            {
                "user_id": user_id,
                "full_name": cust["full_name"],
                "email": cust["email"],
                "registered_at": cust["registered_at"],
                "period_start": period_start,
                "period_end": period_end,
                "usage_hours": usage_hours,
                "session_count": session_count,
                "avg_daily_use": avg_daily_use,
            },
        )
    olap_conn.commit()
    olap_cur.close()
    olap_conn.close()

    return len(customers)


with DAG(
    dag_id="reports_etl",
    description="ETL: CRM + телеметрия → витрина report_mart в OLAP",
    schedule_interval="*/10 * * * *",
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=["reports", "etl", "olap"],
) as dag:
    run_etl = PythonOperator(
        task_id="extract_load_report_mart",
        python_callable=extract_and_load,
        provide_context=True,
    )
