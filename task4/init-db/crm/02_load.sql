-- Загрузка первоначальных данных из CSV
COPY customers (user_id, full_name, email)
FROM '/docker-entrypoint-initdb.d/data/customers.csv'
WITH (FORMAT csv, HEADER true);

COPY orders (order_number, total, discount, buyer_id)
FROM '/docker-entrypoint-initdb.d/data/orders.csv'
WITH (FORMAT csv, HEADER true);
