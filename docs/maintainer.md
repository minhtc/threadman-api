# Maintainer Guide

This document is for people who run, operate, and change the Threadman Leaderboard API. Game client authors should read [`client-integration.md`](client-integration.md) instead; it describes the public contract without implementation detail.

## Architecture

The HTTP layer calls the leaderboard service, the service depends on a repository interface, and PostgreSQL remains behind that interface. The repository owns transaction boundaries and concurrency guarantees.

```text
internal/httpapi     routes, middleware, handlers, probes, metrics
    ↓
internal/leaderboard domain service (validation, enrichment, date policy)
    ↓
internal/leaderboard Repository interface
    ↓
PostgreSQL via pgx/v5
```

Rules that keep this boundary honest:

- Handlers translate HTTP to service calls and never issue SQL.
- The service never imports Fiber or `net/http`.
- Session consumption and score insertion stay inside one repository transaction.
- Rank and top-10 enrichment run after commit and must never fail a committed score.

## Project layout

```text
.
├── cmd/server/                 # API process entry point
├── internal/app/               # Fiber composition and background worker
├── internal/config/            # Validated environment configuration
├── internal/database/          # PostgreSQL pool setup
├── internal/httpapi/           # Routes, middleware, handlers, probes, metrics
├── internal/leaderboard/       # Domain service and PostgreSQL repository
├── internal/security/          # AES-256-GCM and validation
├── public/                     # Static web assets served by the API
├── database/schema.sql         # Single canonical development schema
├── docs/client-integration.md  # Public client integration contract
├── docs/maintainer.md          # This document
├── docker-compose.yml          # Local PostgreSQL + API environment
├── Makefile                    # Repeatable development commands
├── Dockerfile
└── README.md
```

## Requirements

- Go 1.27 or newer
- PostgreSQL 14 or newer
- Docker, optionally
- Docker Compose, optionally

## Local setup

### Docker Compose (recommended)

Generate the required AES-256 master key, place it in `.env`, and start PostgreSQL plus the API:

```bash
cp .env.example .env
openssl rand -base64 32
# Put the generated value in SESSION_SECRET_ENCRYPTION_KEY in .env

docker compose up --build
```

Compose initializes a named PostgreSQL volume with `database/schema.sql`, waits for PostgreSQL readiness, and then starts the API. Open `http://localhost:8080/healthz` after startup. Stop the services with `docker compose down`; remove the development database with `docker compose down -v`.

### Native Go setup

Create a database, copy the environment file, generate the encryption key, apply the schema, and start the API:

```bash
createdb game_db
cp .env.example .env
openssl rand -base64 32
# Put the generated value in SESSION_SECRET_ENCRYPTION_KEY in .env

set -a
source .env
set +a

go run ./cmd/server
```

The server applies `database/schema.sql` automatically during startup, so the `psql` command is optional. The API listens on `:8080` by default. `.env` is loaded automatically when present. `DATABASE_URL` and `SESSION_SECRET_ENCRYPTION_KEY` are required. The encryption key protects session secrets stored in PostgreSQL and is never sent to clients.

## Configuration

| Variable                        | Default               | Description                                            |
| ------------------------------- | --------------------- | ------------------------------------------------------ |
| `PORT`                          | `8080`                | HTTP listen port, `1`–`65535`                          |
| `DATABASE_URL`                  | required              | PostgreSQL connection URL                              |
| `SCHEMA_PATH`                   | `database/schema.sql` | Canonical schema applied during startup                |
| `SESSION_SECRET_ENCRYPTION_KEY` | required              | Base64-encoded 32-byte key for secrets at rest         |
| `LEADERBOARD_TIMEZONE`          | `UTC`                 | IANA timezone for `score_date`                         |
| `ALLOWED_ORIGINS`               | `*`                   | Comma-separated `http`/`https` origins                 |
| `SESSION_TTL`                   | `10m`                 | Session lifetime                                       |
| `MAX_ACTIVE_SESSIONS`           | `5`                   | Active sessions per player/game                        |
| `PRUNE_INTERVAL`                | `15m`                 | Abandoned-session cleanup interval                     |
| `SESSION_RATE_LIMIT`            | `20`                  | Session requests per rate window/IP                    |
| `SCORE_RATE_LIMIT`              | `10`                  | Score requests per rate window/IP                      |
| `LEADERBOARD_RATE_LIMIT`        | `60`                  | Leaderboard requests per rate window/IP                |
| `RATE_LIMIT_WINDOW`             | `1m`                  | Rate-limit window                                      |
| `BODY_LIMIT_BYTES`              | `4096`                | Maximum request body size                              |
| `REQUEST_TIMEOUT`               | `3s`                  | Database-backed request timeout                        |
| `READINESS_TIMEOUT`             | `2s`                  | Readiness probe timeout                                |
| `PRUNE_TIMEOUT`                 | `5s`                  | Pruning query timeout                                  |
| `DB_CONNECT_TIMEOUT`            | `5s`                  | PostgreSQL connection initialization timeout           |
| `DB_PING_TIMEOUT`               | `5s`                  | PostgreSQL pool ping timeout                           |
| `READ_TIMEOUT`                  | `10s`                 | HTTP read timeout                                      |
| `WRITE_TIMEOUT`                 | `10s`                 | HTTP write timeout                                     |
| `IDLE_TIMEOUT`                  | `30s`                 | Keep-alive idle timeout                                |
| `SHUTDOWN_TIMEOUT`              | `10s`                 | Graceful shutdown timeout                              |
| `DB_MAX_CONNS`                  | `15`                  | PostgreSQL pool maximum                                |
| `DB_MIN_CONNS`                  | `3`                   | PostgreSQL pool minimum                                |
| `TRUST_PROXY`                   | `false`               | Enable trusted-proxy client IP extraction              |
| `PROXY_HEADER`                  | `CF-Connecting-IP`    | Proxy client-IP header                                 |
| `TRUSTED_PROXIES`               | empty                 | Trusted IPs/CIDRs; required when proxy mode is enabled |

Configuration is validated at startup. A missing or invalid value stops the process instead of producing a partially working API.

## Schema and migrations

The API applies the canonical `SCHEMA_PATH` file during startup before serving requests. Schema setup is idempotent; a missing or invalid schema stops startup instead of allowing a partially working API to return database errors. Compose also mounts the same schema into PostgreSQL for first-time database initialization.

This project is not deployed yet, so `database/schema.sql` is intentionally the single source of truth for database setup. If the schema changes during development, update this file and reapply it to a fresh development database.

Add a migration system later if this project becomes deployed or multiple environments need independent schema history.

## API surface

All game endpoints are under `/v1/game/:gameCode`. The only supported game code is `threadman`. POST requests require `Content-Type: application/json`; unknown fields and trailing JSON values are rejected.

| Endpoint                       | Purpose                       | Default rate limit |
| ------------------------------ | ----------------------------- | ------------------ |
| `POST /v1/game/:gameCode/session` | Create a single-use session   | 20/min/IP          |
| `POST /v1/game/:gameCode/score`   | Submit one encrypted score    | 10/min/IP          |
| `GET  /v1/game/:gameCode/leaderboard` | Daily or all-time top 10   | 60/min/IP          |
| `GET /healthz`                 | Process health; no database   | —                  |
| `GET /readyz`                  | PostgreSQL readiness          | —                  |
| `GET /metrics`                 | Prometheus-compatible metrics | —                  |

Leaderboard query parameters:

- `date=YYYY-MM-DD` — inspect one calendar day in `LEADERBOARD_TIMEZONE`; omitted means the current day.
- `period=all-time` — top 10 across every recorded day. Cannot be combined with `date`; any other `period` value returns `400`.
- Day responses include `period: "day"` and `score_date`; all-time responses include `period: "all-time"` and omit `score_date`.

Every request receives `X-Request-ID`, and JSON error responses include the same ID:

```json
{ "error": "session expired", "request_id": "6b1c4e3b-..." }
```

Common statuses: `400` validation errors, `404` unknown game/session, `409` consumed session, `415` invalid content type, `429` rate limit or active-session limit, and `503` database readiness failure.

Behavioral details that matter when changing handlers:

- A successful score is committed before rank/top-10 enrichment. The response reports `rank_available` and `leaderboard_available`; clients must not retry a session just because enrichment is temporarily unavailable.
- Timestamp drift is limited to ±120 seconds, duration to 3 seconds–1 hour, and score rate to 200 points per elapsed second.
- The score payload is AES-256-GCM encrypted by the client with the per-session key; the plaintext before encryption is a JSON object with `player_name`, `score`, `duration_ms`, and `timestamp`.

## Security model

- Client payloads use AES-256-GCM with a fresh nonce.
- PostgreSQL stores an encrypted envelope of each session secret, not the plaintext secret.
- The master key is `SESSION_SECRET_ENCRYPTION_KEY`; keep it in a secret manager and never log it.
- New sessions contain 32 random bytes and are represented as 64 lowercase hex characters.
- Session creation uses `pg_advisory_xact_lock` per game/player so concurrent requests cannot exceed the active-session limit.
- Session consumption and score insertion happen in one transaction.
- No legacy protocol compatibility is retained because this project has not been deployed.
- Client-authoritative scoring is not cheat-proof. A modified client can fabricate a score within the configured validation rules. Fully authoritative scoring requires trusted server-side gameplay state.

## Development commands

```bash
make fmt
make test
make race
make vet
make build
```

Equivalent commands:

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
go build -o bin/threadman-api ./cmd/server
```

The PostgreSQL contention test is opt-in:

```bash
TEST_DATABASE_URL='postgres://postgres:password@localhost:5432/game_db?sslmode=disable' \
  go test ./internal/leaderboard -run TestPostgresSessionLimitUnderContention -v
```

It applies `database/schema.sql` and skips when `TEST_DATABASE_URL` is absent.

## Container

```bash
docker build -t threadman-api .
docker run --rm --network host --env-file .env threadman-api
```

The image contains only the API binary, runs as a non-root user, and uses the same `database/schema.sql` setup described above. `.dockerignore` excludes `.env`, Git metadata, and local artifacts.

## Operations

- `GET /healthz` reports process health and does not require the database; use it for container liveness.
- `GET /readyz` pings PostgreSQL within `READINESS_TIMEOUT`; use it for readiness/load-balancer checks.
- `GET /metrics` exposes `http_requests_total` plus PostgreSQL pool gauges (`db_pool_total_connections`, `db_pool_acquired_connections`, `db_pool_idle_connections`, `db_pool_max_connections`, `db_pool_empty_acquire_wait_seconds`).
- Logs are JSON on stdout via `slog`. Each request records request ID, method, path, status, duration, and IP. Failures also emit `create_session_failed`, `submit_score_failed`, `get_leaderboard_failed`, `get_all_time_leaderboard_failed`, and `session_pruning_failed`.
- Rate limits are per IP and per route; enable `TRUST_PROXY` with `TRUSTED_PROXIES` when running behind a CDN or load balancer so limits key on the real client IP.
- A background worker prunes abandoned sessions every `PRUNE_INTERVAL`.
- Shutdown is graceful up to `SHUTDOWN_TIMEOUT` on `SIGINT`/`SIGTERM`.
- Startup order: validate configuration, open the pool, apply `SCHEMA_PATH`, then listen. A failure at any of those steps exits the process.

## Development checklist

- Reapply `database/schema.sql` to a fresh database after schema changes.
- Keep `.env` and `SESSION_SECRET_ENCRYPTION_KEY` out of source control.
- Run the race detector before merging concurrency changes.
- Run the PostgreSQL contention test when PostgreSQL is available.
- Tune score validation in `internal/security/validation.go` for actual game mechanics.
- Keep [`client-integration.md`](client-integration.md) in sync with any change to routes, payloads, limits, or error codes.
- Add a migration system later if this project becomes deployed or multiple environments need independent schema history.
