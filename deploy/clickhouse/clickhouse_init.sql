CREATE DATABASE IF NOT EXISTS Schema_1;
CREATE DATABASE IF NOT EXISTS Schema_2;
CREATE DATABASE IF NOT EXISTS Schema_3;

-- Public
CREATE DATABASE IF NOT EXISTS Schema_4;

CREATE TABLE IF NOT EXISTS Schema_1.financial_records
(
    id UInt64,
    account_code String,
    amount Decimal(18,2),
    currency FixedString(3),
    reference_date Date
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Schema_1.financial_ledger
(
    -- identifiers
    id UInt64,
    transaction_id UUID,
    document_number String,
    document_type LowCardinality(String),

    -- accounting structure
    account_code String,
    account_name String,
    cost_center LowCardinality(String),
    project_code LowCardinality(String),
    branch_id UInt16,
    branch_name LowCardinality(String),

    -- parties
    partner_code String,
    partner_name String,
    partner_type LowCardinality(String),

    -- monetary values
    debit Decimal(18,2),
    credit Decimal(18,2),
    balance Decimal(18,2),
    currency FixedString(3),
    exchange_rate Decimal(18,6),
    amount_local Decimal(18,2),

    -- taxes
    tax_code LowCardinality(String),
    tax_base Decimal(18,2),
    tax_amount Decimal(18,2),
    tax_type LowCardinality(String),

    -- dates
    reference_date Date,
    posting_date Date,
    document_date Date,
    created_at DateTime,
    updated_at DateTime,

    -- metadata / audit
    source_system LowCardinality(String),
    created_by String,
    updated_by String,
    is_reversal UInt8,
    status LowCardinality(String),

    -- free-form
    description String,
    notes String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(reference_date)
ORDER BY (
    reference_date,
    account_code,
    branch_id,
    project_code,
    transaction_id
)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS Schema_1.audit_log
(
    id UInt64,
    action String,
    user_id String,
    executed_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Schema_2.financial_records
(
    id UInt64,
    cost_center String,
    amount Decimal(18,2),
    currency FixedString(3),
    transaction_date Date
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Schema_2.audit_log
(
    id UInt64,
    action String,
    user_id String,
    executed_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Schema_3.operations
(
    id UInt64,
    operation String,
    status String,
    created_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

CREATE TABLE IF NOT EXISTS Schema_3.metrics
(
    id UInt64,
    metric_name String,
    metric_value Float64,
    measured_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

/* ===============================
   Schema_4 (Public)
   =============================== */
CREATE TABLE IF NOT EXISTS Schema_4.metrics
(
    id UInt64,
    metric_name String,
    metric_value Float64,
    measured_at DateTime
)
ENGINE = MergeTree
ORDER BY id;

/* ===============================
   SEED DATA
   =============================== */

-- Schema_1.financial_ledger (300k rows)
INSERT INTO Schema_1.financial_ledger
SELECT
    number + 1                                   AS id,
    generateUUIDv4()                             AS transaction_id,
    concat('DOC-', toString(number))             AS document_number,
    arrayElement(['INVOICE','PAYMENT','ADJUSTMENT','REVERSAL'], rand() % 4 + 1) AS document_type,
    concat('3.', toString(rand() % 9), '.01.', lpad(toString(rand() % 100), 2, '0')) AS account_code,
    concat('Account ', toString(rand() % 500))   AS account_name,
    arrayElement(['CC01','CC02','CC03','CC04'], rand() % 4 + 1) AS cost_center,
    arrayElement(['PRJ_A','PRJ_B','PRJ_C'], rand() % 3 + 1)     AS project_code,
    rand() % 10 + 1                               AS branch_id,
    arrayElement(['HQ','SP','RJ','MG','RS'], rand() % 5 + 1)    AS branch_name,
    concat('PN-', toString(rand() % 10000))       AS partner_code,
    concat('Partner ', toString(rand() % 10000)) AS partner_name,
    arrayElement(['CUSTOMER','SUPPLIER','INTERNAL'], rand() % 3 + 1) AS partner_type,
    toDecimal64(rand() % 100000 / 10.0, 2)        AS debit,
    toDecimal64(rand() % 100000 / 10.0, 2)        AS credit,
    toDecimal64(rand() % 500000 / 10.0, 2)        AS balance,
    arrayElement(['BRL','USD','EUR'], rand() % 3 + 1) AS currency,
    toDecimal64(1 + rand() % 3000 / 1000.0, 6)    AS exchange_rate,
    toDecimal64(rand() % 100000 / 10.0, 2)        AS amount_local,
    arrayElement(['TAX01','TAX02','TAX03'], rand() % 3 + 1) AS tax_code,
    toDecimal64(rand() % 50000 / 10.0, 2)         AS tax_base,
    toDecimal64(rand() % 10000 / 10.0, 2)         AS tax_amount,
    arrayElement(['ICMS','PIS','COFINS','ISS'], rand() % 4 + 1) AS tax_type,
    today() - rand() % 365                        AS reference_date,
    today() - rand() % 365                        AS posting_date,
    today() - rand() % 365                        AS document_date,
    now()                                        AS created_at,
    now()                                        AS updated_at,
    arrayElement(['SAP','MANUAL','API'], rand() % 3 + 1) AS source_system,
    concat('user-', toString(rand() % 500))       AS created_by,
    concat('user-', toString(rand() % 500))       AS updated_by,
    rand() % 2                                   AS is_reversal,
    arrayElement(['OPEN','POSTED','CANCELED'], rand() % 3 + 1) AS status,
    concat('Transaction description ', toString(number)) AS description,
    concat('Notes ', toString(rand() % 1000))     AS notes
FROM numbers(300000);

-- Schema_1.financial_records (100k rows)
INSERT INTO Schema_1.financial_records
SELECT
    number + 4 AS id,
    concat('3.1.03.', toString(100 + (number % 900))) AS account_code,
    round(exp(rand() % 10), 2) AS amount,
    arrayElement(['BRL', 'USD', 'EUR'], (number % 3) + 1) AS currency,
    toDate('2025-01-01') + (number % 365) AS reference_date
FROM numbers(100000);

-- Schema_1.audit_log (100k rows)
INSERT INTO Schema_1.audit_log
SELECT
    number + 3 AS id,
    arrayElement(['INSERT_FINANCIAL', 'EXPORT_EXCEL', 'DELETE_RECORD', 'UPDATE_TAX'], (number % 4) + 1) AS action,
    concat('user-aad-', toString(100 + (number % 50))) AS user_id,
    now() - (number * 10) AS executed_at
FROM numbers(100000);

-- Schema_2.financial_records (100k rows)
INSERT INTO Schema_2.financial_records
SELECT
    number + 4 AS id,
    arrayElement(['CC-LOG-01', 'CC-IMP-02', 'CC-VEN-03', 'CC-DIR-04'], (number % 4) + 1) AS cost_center,
    (rand() % 100000) / 100 AS amount,
    arrayElement(['BRL', 'USD'], (number % 2) + 1) AS currency,
    toDate('2025-01-01') + (number % 365) AS transaction_date
FROM numbers(100000);

-- Schema_3.operations (100k rows)
INSERT INTO Schema_3.operations
SELECT
    number + 4 AS id,
    arrayElement(['IMPORT_CONTAINER', 'WAREHOUSE_MOVE', 'DELIVERY_CLIENT', 'QUALITY_CHECK'], (number % 4) + 1) AS operation,
    arrayElement(['COMPLETED', 'PENDING', 'FAILED', 'IN_PROGRESS'], (number % 4) + 1) AS status,
    now() - (number * 60) AS created_at
FROM numbers(100000);

-- Schema_3.metrics (100k rows)
INSERT INTO Schema_3.metrics
SELECT
    number + 3 AS id,
    arrayElement(['containers_processed', 'avg_delivery_time_h', 'fuel_efficiency', 'staff_on_duty'], (number % 4) + 1) AS metric_name,
    (rand() % 500) + (rand() % 100 / 100.0) AS metric_value,
    now() - (number * 300) AS measured_at
FROM numbers(100000);

-- Schema_4.metrics (10k rows)
INSERT INTO Schema_4.metrics
SELECT
    number + 3 AS id,
    arrayElement(['containers_processed', 'avg_delivery_time_h', 'fuel_efficiency', 'staff_on_duty'], (number % 4) + 1) AS metric_name,
    (rand() % 500) + (rand() % 100 / 100.0) AS metric_value,
    now() - (number * 300) AS measured_at
FROM numbers(10000);
