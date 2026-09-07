# Threadman Leaderboard API

A development-stage leaderboard API for Threadman, built with Go 1.27, Fiber v3, and PostgreSQL (`pgx/v5`). Players do not need accounts: the client creates a local UUID, obtains a short-lived session, encrypts one score, and submits it.

> Client-authoritative scoring is not cheat-proof. A modified client can fabricate a score within the configured validation rules. Fully authoritative scoring requires trusted server-side gameplay state.

## Features

- Single-use sessions with configurable expiration.
- AES-256-GCM score payloads using a per-session 32-byte secret.
- Session secrets encrypted at rest with a separate 32-byte server key.
- Server-derived player identity and atomic session consumption.
- PostgreSQL advisory-lock protection for the active-session limit.
- Daily top-10 leaderboards with deterministic tie-breaking.
- UTF-8, control-character, zero-width, timestamp, duration, and score-rate validation.
- Strict JSON decoding that rejects unknown fields and trailing values.
- Configurable rate limits, HTTP timeouts, database pool limits, and trusted proxies.
- `GET /healthz`, `GET /readyz`, and `GET /metrics` for operations.
- Unmatched web paths are served from `public/` after API route matching.

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
├── docker-compose.yml          # Local PostgreSQL + API environment
├── Makefile                    # Repeatable development commands
├── Dockerfile
└── README.md
```

The HTTP layer calls the leaderboard service, the service depends on a repository interface, and PostgreSQL remains behind that interface. The repository owns transaction boundaries and concurrency guarantees.

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

psql "$DATABASE_URL" -f database/schema.sql
go run ./cmd/server
```

The API listens on `:8080` by default. `.env` is loaded automatically when present. `DATABASE_URL` and `SESSION_SECRET_ENCRYPTION_KEY` are required. The encryption key protects session secrets stored in PostgreSQL and is never sent to clients.

This project is not deployed yet, so `database/schema.sql` is intentionally the single source of truth for database setup. If the schema changes during development, update this file and reapply it to a fresh development database.

## Configuration

| Variable                        | Default           | Description                                            |
| ------------------------------- | ----------------- | ------------------------------------------------------ |
| `PORT`                          | `8080`            | HTTP listen port, `1`–`65535`                          |
| `DATABASE_URL`                  | required          | PostgreSQL connection URL                              |
| `SESSION_SECRET_ENCRYPTION_KEY` | required          | Base64-encoded 32-byte key for secrets at rest         |
| `LEADERBOARD_TIMEZONE`          | `UTC`             | IANA timezone for `score_date`                         |
| `ALLOWED_ORIGINS`               | `*`               | Comma-separated `http`/`https` origins                 |
| `SESSION_TTL`                   | `10m`             | Session lifetime                                       |
| `MAX_ACTIVE_SESSIONS`           | `5`               | Active sessions per player/game                        |
| `PRUNE_INTERVAL`                | `15m`             | Abandoned-session cleanup interval                     |
| `SESSION_RATE_LIMIT`            | `20`              | Session requests per rate window/IP                    |
| `SCORE_RATE_LIMIT`              | `10`              | Score requests per rate window/IP                      |
| `LEADERBOARD_RATE_LIMIT`        | `60`              | Leaderboard requests per rate window/IP                |
| `RATE_LIMIT_WINDOW`             | `1m`              | Rate-limit window                                      |
| `BODY_LIMIT_BYTES`              | `4096`            | Maximum request body size                              |
| `REQUEST_TIMEOUT`               | `3s`              | Database-backed request timeout                        |
| `READINESS_TIMEOUT`             | `2s`              | Readiness probe timeout                                |
| `PRUNE_TIMEOUT`                 | `5s`              | Pruning query timeout                                  |
| `DB_CONNECT_TIMEOUT`            | `5s`              | PostgreSQL connection initialization timeout           |
| `DB_PING_TIMEOUT`               | `5s`              | PostgreSQL pool ping timeout                           |
| `READ_TIMEOUT`                  | `10s`             | HTTP read timeout                                      |
| `WRITE_TIMEOUT`                 | `10s`             | HTTP write timeout                                     |
| `IDLE_TIMEOUT`                  | `30s`             | Keep-alive idle timeout                                |
| `SHUTDOWN_TIMEOUT`              | `10s`             | Graceful shutdown timeout                              |
| `DB_MAX_CONNS`                  | `15`              | PostgreSQL pool maximum                                |
| `DB_MIN_CONNS`                  | `3`               | PostgreSQL pool minimum                                |
| `TRUST_PROXY`                   | `false`           | Enable trusted-proxy client IP extraction              |
| `PROXY_HEADER`                  | `X-Forwarded-For` | Proxy client-IP header                                 |
| `TRUSTED_PROXIES`               | empty             | Trusted IPs/CIDRs; required when proxy mode is enabled |

Never enable `TRUST_PROXY` without a restricted `TRUSTED_PROXIES` list. Set explicit CORS origins outside local development.

## API

All game endpoints are under `/v1/game/:gameCode`. The only supported game code is `threadman`. POST requests require `Content-Type: application/json`; unknown fields and trailing JSON values are rejected.

### Probes and metrics

```text
GET /healthz   # process health; no database required
GET /readyz    # PostgreSQL readiness
GET /metrics   # Prometheus-compatible metrics
```

Every request receives `X-Request-ID`. JSON error responses include the same ID.

### Create a session

`POST /v1/game/threadman/session` — default rate limit: 20 requests/minute/IP.

```bash
curl -X POST http://localhost:8080/v1/game/threadman/session \
  -H 'Content-Type: application/json' \
  -d '{"player_id":"9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"}'
```

```json
{
  "session_id": "a81bc81b-dead-4e5d-b234-a123456789ab",
  "session_secret": "64-lowercase-hex-characters-for-aes-256",
  "expires_at": "2026-09-07T08:30:00Z"
}
```

### Submit a score

`POST /v1/game/threadman/score` — default rate limit: 10 requests/minute/IP.

```json
{
  "session_id": "a81bc81b-dead-4e5d-b234-a123456789ab",
  "payload": "Base64(12-byte nonce + AES-256-GCM ciphertext + 16-byte tag)"
}
```

The plaintext payload before encryption is:

```json
{
  "player_name": "Speedy",
  "score": 1450,
  "duration_ms": 35200,
  "timestamp": 1757232000
}
```

The AES key is the 32-byte value represented by `session_secret`. Generate a random 12-byte nonce, encrypt with AES-256-GCM, concatenate `nonce || ciphertext_and_tag`, and Base64-encode it. Timestamp drift is limited to ±120 seconds, duration to 3 seconds–1 hour, and score rate to 200 points per elapsed second.

A successful score is committed before rank/top-10 enrichment. The response reports `rank_available` and `leaderboard_available`; clients must not retry a session just because enrichment is temporarily unavailable.

### Leaderboard

`GET /v1/game/threadman/leaderboard` — default rate limit: 60 requests/minute/IP.

```json
{
  "game_code": "threadman",
  "score_date": "2026-09-07",
  "top10": [
    { "rank": 1, "player_name": "Ace", "score": 3200 },
    { "rank": 2, "player_name": "Blitz", "score": 2100 }
  ]
}
```

### Errors

```json
{ "error": "session expired", "request_id": "6b1c4e3b-..." }
```

Common statuses: `400` validation errors, `404` unknown game/session, `409` consumed session, `415` invalid content type, `429` rate limit or active-session limit, and `503` database readiness failure.

## Security model

- Client payloads use AES-256-GCM with a fresh nonce.
- PostgreSQL stores an encrypted envelope of each session secret, not the plaintext secret.
- The master key is `SESSION_SECRET_ENCRYPTION_KEY`; keep it in a secret manager and never log it.
- New sessions contain 32 random bytes and are represented as 64 lowercase hex characters.
- Session creation uses `pg_advisory_xact_lock` per game/player so concurrent requests cannot exceed the active-session limit.
- Session consumption and score insertion happen in one transaction.
- No legacy protocol compatibility is retained because this project has not been deployed.

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
go build -o bin/homielab-api ./cmd/server
```

The PostgreSQL contention test is opt-in:

```bash
TEST_DATABASE_URL='postgres://postgres:password@localhost:5432/game_db?sslmode=disable' \
  go test ./internal/leaderboard -run TestPostgresSessionLimitUnderContention -v
```

It applies `database/schema.sql` and skips when `TEST_DATABASE_URL` is absent.

## Container

```bash
docker build -t homielab-api .
docker run --rm --network host --env-file .env homielab-api
```

The image contains only the API binary, runs as a non-root user, and uses the same `database/schema.sql` setup described above. `.dockerignore` excludes `.env`, Git metadata, and local artifacts.

## Development checklist

- Reapply `database/schema.sql` to a fresh database after schema changes.
- Keep `.env` and `SESSION_SECRET_ENCRYPTION_KEY` out of source control.
- Run the race detector before merging concurrency changes.
- Run the PostgreSQL contention test when PostgreSQL is available.
- Tune score validation in `internal/security/validation.go` for actual game mechanics.
- Add a migration system later if this project becomes deployed or multiple environments need independent schema history.
