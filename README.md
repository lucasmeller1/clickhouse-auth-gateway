# ClickHouse Gateway API

A Go microservice that acts as an authenticated gateway between **Microsoft Excel** and **Power BI** and a ClickHouse analytical database. Clients connect via **Power Query's "From Web"** connector — the API handles authentication, authorization, and data delivery so that analysts can refresh ClickHouse data directly inside Excel or Power BI without any database credentials or direct network access to ClickHouse.

Users authenticate via Microsoft Entra ID (Azure AD), and the API enforces role-based access by mapping Entra ID group claims to ClickHouse schemas.

The service runs two HTTP servers: a **public server** (`:8080`) that handles authenticated requests from clients via Cloudflare Tunnel, and a **private server** (`:8081`) for internal cache invalidation, intended to be called by data pipelines (e.g., Airflow) after ETL updates. Port `:8081` should only be reachable from your internal network or pipeline infrastructure — never expose it publicly.

---

## Prerequisites

- Go 1.25.5+
- Docker & Docker Compose
- A Microsoft Entra ID tenant with an app registration and security groups assigned to users
- A Cloudflare Tunnel token (easier for local testing)

---

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/lucasmeller1/clickhouse-auth-gateway.git
cd clickhouse-auth-gateway

# 2. Create the Docker network
docker network create test_network

# 3. Configure environment
cp .env_template .env
# Edit .env with your values

# 4. Configure schema mappings
# Edit schema_guids.json (see Schema Configuration below)

# 5. Build and run
make deploy

# 6. Verify
curl http://localhost:8080/healthz
# Expected: ok
```

---

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TENANT_ID` | Yes | Microsoft Entra ID tenant ID |
| `AUDIENCE_JWT` | Yes | App registration client ID (JWT audience) |
| `CLICKHOUSE_HOSTNAME` | Yes | ClickHouse server hostname |
| `CLICKHOUSE_PORT` | Yes | ClickHouse HTTP port (default: `8123`) |
| `CLICKHOUSE_USER` | Yes | ClickHouse username |
| `CLICKHOUSE_PASSWORD` | Yes | ClickHouse password |
| `CLICKHOUSE_SCHEMA` | Yes | Default ClickHouse schema |
| `REDIS_HOSTNAME` | Yes | Redis server hostname |
| `REDIS_PORT` | Yes | Redis port (default: `6379`) |
| `REDIS_PASSWORD` | Yes | Redis password |
| `REDIS_DB` | Yes | Redis database number |
| `QUEUE_SIZE_LIMITER` | Yes | Max concurrent ClickHouse export queries (e.g., `50`) |
| `INVALIDATE_CACHE_TOKEN` | Yes | Bearer token for the private cache invalidation endpoint |
| `CLOUDFLARE_TUNNEL_TOKEN` | Yes | Cloudflare Tunnel token |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | OTel collector endpoint (default: `http://otel-collector:4317`) |

### Server Defaults

| Setting | Value |
|---------|-------|
| Public port | `:8080` |
| Private port | `:8081` |
| Read timeout | 10s |
| Write timeout | 60s |
| Idle timeout | 120s |
| Shutdown timeout | 15s |
| Export rate limit | 200 req/min per user |
| Tables rate limit | 30 req/min per user |
| Cache TTL | 1 min |
| Max export size | 100 MB (gzip-compressed) |

When a rate limit is exceeded, the API returns `429 Too Many Requests`. When an export exceeds the 100 MB gzip-compressed cap, the request is rejected before streaming begins.

---

## API Endpoints

### Public (`:8080`)

The `/healthz` endpoint is unauthenticated. All other endpoints require a valid Entra ID Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check (no auth required) |
| `GET` | `/v1/tabelas` | List all tables the user has access to, with download URLs |
| `GET` | `/v1/exportar?database={db}&table={tbl}` | Export a table as CSV (gzip if supported) |

### Private (`:8081`)

Requires the static bearer token set in `INVALIDATE_CACHE_TOKEN`. Keep this port isolated from public traffic.

| Method | Path | Description |
|--------|------|-------------|
| `DELETE` | `/deleteCache?database={db}&table={tbl}` | Invalidate a cached table export in Redis |

Successful exports return `text/csv; charset=utf-8` with optional `Content-Encoding: gzip`. Errors return a JSON body: `{"error": "description"}`.

### Example Usage

```bash
# List available tables
curl -H "Authorization: Bearer <ENTRA_ID_TOKEN>" \
  https://your-tunnel-domain.com/v1/tabelas

# Export a table
curl -H "Authorization: Bearer <ENTRA_ID_TOKEN>" \
  -H "Accept-Encoding: gzip" \
  https://your-tunnel-domain.com/v1/exportar?database=Financeiro_1&table=balances \
  -o export.csv.gz

# Invalidate cache (from Airflow or similar)
curl -X DELETE \
  -H "Authorization: Bearer <INVALIDATE_CACHE_TOKEN>" \
  http://localhost:8081/deleteCache?database=Financeiro_1&table=balances
```

---

## Authentication & Authorization

Clients obtain a JWT from Entra ID and send it as `Authorization: Bearer <token>`. The API validates the token by fetching JWKS keys from Entra ID (cached in Redis for 1 hour), verifying the RS256 signature, expiration, issuer, and audience. If signature verification fails due to key rotation, it retries with a forced JWKS refresh.

Authorization is claim-based. The user's `groups` claim (security group GUIDs) is matched against `schema_guids.json`. Public schemas are accessible to all authenticated users; restricted schemas require the user to belong to the corresponding Entra ID group. Rate limiting is applied per-user using the `oid` claim.

---

## Schema Configuration

Edit `schema_guids.json` to map Entra ID group GUIDs to ClickHouse database names:

```json
{
  "schemas_guid": {
    "Financeiro_1": "24bde539-9b6d-494e-b922-c71c6e43dc9b",
    "Operacional_1": "6138d620-bf80-4538-bb83-63e2e7aea359"
  },
  "public_schemas": ["Atualizacoes", "Consultas"]
}
```

`schemas_guid` maps schema names to group GUIDs. Schema names must match `[a-zA-Z0-9_]{1,255}` and GUIDs must be valid UUIDs — the API validates this at startup and will refuse to boot if anything is invalid or duplicated. `public_schemas` lists schemas accessible to any authenticated user without group membership.

---

## Observability

The stack ships with full OpenTelemetry instrumentation and is enabled by default when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Every request generates a distributed trace spanning authentication, Redis cache lookup, ClickHouse query, and response streaming. OTel metrics are exported alongside traces, covering request counts, export durations, and active export concurrency.

| Component | Purpose | URL |
|-----------|---------|-----|
| Grafana | Dashboards | `http://localhost:3000` |
| Prometheus | Metrics | `http://localhost:9090` |
| Tempo | Distributed traces | `http://localhost:3200` |
| Loki | Log aggregation | `http://localhost:3100` |

Custom key metrics: `api.request.count`, `table.export.processing.duration`, `active.export`.

---

## Concurrency & Caching Architecture

Export requests go through two layers of protection before hitting ClickHouse.

**Singleflight deduplication:** When multiple users request the same table simultaneously, only one ClickHouse query executes. All concurrent callers share the result, preventing redundant full-table scans and Redis writes under bursty load.

**Concurrency queue:** `QUEUE_SIZE_LIMITER` caps the number of ClickHouse export queries running at any given moment. Requests that arrive when the queue is full wait for a slot (respecting request context cancellation) rather than being rejected outright. This protects ClickHouse from overload while keeping the API responsive.

---

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make build` | Build the Docker image |
| `make up` | Start all services |
| `make down` | Stop all services |
| `make down-clean` | Stop all services and remove volumes |
| `make restart` | Restart all services |
| `make deploy` | Build image and restart all services |

---
