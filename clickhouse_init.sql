/* ===============================
   CONTÁBIL
   =============================== */
CREATE DATABASE IF NOT EXISTS Contabil_1;
CREATE DATABASE IF NOT EXISTS Contabil_2;
CREATE DATABASE IF NOT EXISTS Contabil_3;
CREATE DATABASE IF NOT EXISTS Contabil_4;
CREATE DATABASE IF NOT EXISTS Contabil_5;
CREATE DATABASE IF NOT EXISTS Contabil_6;
CREATE DATABASE IF NOT EXISTS Contabil_7;

/* ===============================
   FINANCEIRO
   =============================== */
CREATE DATABASE IF NOT EXISTS Financeiro_1;
CREATE DATABASE IF NOT EXISTS Financeiro_2;
CREATE DATABASE IF NOT EXISTS Financeiro_3;
CREATE DATABASE IF NOT EXISTS Financeiro_4;
CREATE DATABASE IF NOT EXISTS Financeiro_5;
CREATE DATABASE IF NOT EXISTS Financeiro_6;
CREATE DATABASE IF NOT EXISTS Financeiro_7;

/* ===============================
   OPERACIONAL
   =============================== */
CREATE DATABASE IF NOT EXISTS Operacional_1;
CREATE DATABASE IF NOT EXISTS Operacional_2;
CREATE DATABASE IF NOT EXISTS Operacional_3;
CREATE DATABASE IF NOT EXISTS Operacional_4;
CREATE DATABASE IF NOT EXISTS Operacional_5;
CREATE DATABASE IF NOT EXISTS Operacional_6;
CREATE DATABASE IF NOT EXISTS Operacional_7;

CREATE DATABASE IF NOT EXISTS Atualizacoes;

CREATE TABLE IF NOT EXISTS Atualizacoes.financial_records
(
    id UInt64,
    account_code String,
    amount Decimal(18,2),
    currency FixedString(3),
    reference_date Date
)
ENGINE = MergeTree
ORDER BY id;


CREATE TABLE IF NOT EXISTS Contabil_1.financial_records
(
    id UInt64,
    account_code String,
    amount Decimal(18,2),
    currency FixedString(3),
    reference_date Date
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Contabil_1.audit_log
(
    id UInt64,
    action String,
    user_id String,
    executed_at DateTime
)
ENGINE = MergeTree
ORDER BY id;


CREATE TABLE IF NOT EXISTS Financeiro_1.financial_records
(
    id UInt64,
    cost_center String,
    amount Decimal(18,2),
    currency FixedString(3),
    transaction_date Date
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Financeiro_1.audit_log
(
    id UInt64,
    action String,
    user_id String,
    executed_at DateTime
)
ENGINE = MergeTree
ORDER BY id;


CREATE TABLE IF NOT EXISTS Operacional_1.operations
(
    id UInt64,
    operation String,
    status String,
    created_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Operacional_1.metrics
(
    id UInt64,
    metric_name String,
    metric_value Float64,
    measured_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

-- Inserting 100,000 random records into Contabil_1.financial_records
INSERT INTO Atualizacoes.financial_records
SELECT
    number + 4 AS id, -- Starting from 4 to avoid ID conflict with your manual inserts
    concat('3.1.03.', toString(100 + (number % 900))) AS account_code,
    round(exp(rand() % 10), 2) AS amount, -- Generates realistic varying amounts
    arrayElement(['BRL', 'USD', 'EUR'], (number % 3) + 1) AS currency,
    toDate('2025-01-01') + (number % 365) AS reference_date
FROM numbers(100000);


-- Inserting 100,000 random records into Contabil_1.financial_records
INSERT INTO Contabil_1.financial_records
SELECT
    number + 4 AS id, -- Starting from 4 to avoid ID conflict with your manual inserts
    concat('3.1.03.', toString(100 + (number % 900))) AS account_code,
    round(exp(rand() % 10), 2) AS amount, -- Generates realistic varying amounts
    arrayElement(['BRL', 'USD', 'EUR'], (number % 3) + 1) AS currency,
    toDate('2025-01-01') + (number % 365) AS reference_date
FROM numbers(100000);

-- Inserting 100,000 random records into Contabil_1.audit_log
INSERT INTO Contabil_1.audit_log
SELECT
    number + 3 AS id,
    arrayElement(['INSERT_FINANCIAL', 'EXPORT_EXCEL', 'DELETE_RECORD', 'UPDATE_TAX'], (number % 4) + 1) AS action,
    concat('user-aad-', toString(100 + (number % 50))) AS user_id,
    now() - (number * 10) AS executed_at
FROM numbers(100000);

-- Inserting 100,000 random records into Financeiro_1.financial_records
INSERT INTO Financeiro_1.financial_records
SELECT
    number + 4 AS id,
    arrayElement(['CC-LOG-01', 'CC-IMP-02', 'CC-VEN-03', 'CC-DIR-04'], (number % 4) + 1) AS cost_center,
    (rand() % 100000) / 100 AS amount,
    arrayElement(['BRL', 'USD'], (number % 2) + 1) AS currency,
    toDate('2025-01-01') + (number % 365) AS transaction_date
FROM numbers(100000);

-- Inserting 100,000 random records into Operacional_1.operations
INSERT INTO Operacional_1.operations
SELECT
    number + 4 AS id,
    arrayElement(['IMPORT_CONTAINER', 'WAREHOUSE_MOVE', 'DELIVERY_CLIENT', 'QUALITY_CHECK'], (number % 4) + 1) AS operation,
    arrayElement(['COMPLETED', 'PENDING', 'FAILED', 'IN_PROGRESS'], (number % 4) + 1) AS status,
    now() - (number * 60) AS created_at
FROM numbers(100000);

-- Inserting 100,000 random records into Operacional_1.metrics
INSERT INTO Operacional_1.metrics
SELECT
    number + 3 AS id,
    arrayElement(['containers_processed', 'avg_delivery_time_h', 'fuel_efficiency', 'staff_on_duty'], (number % 4) + 1) AS metric_name,
    (rand() % 500) + (rand() % 100 / 100.0) AS metric_value,
    now() - (number * 300) AS measured_at
FROM numbers(100000);
