# Threadman Leaderboard API

A development-stage leaderboard API for Threadman, built with Go 1.27, Fiber v3, and PostgreSQL (`pgx/v5`). Players do not need accounts: the client creates a local UUID, obtains a short-lived session, encrypts one score, and submits it.

> Client-authoritative scoring is not cheat-proof. A modified client can fabricate a score within the configured validation rules. Fully authoritative scoring requires trusted server-side gameplay state.

## Documentation

| Document                                                        | Audience                                                     |
| --------------------------------------------------------------- | ------------------------------------------------------------ |
| [`docs/client-integration.md`](docs/client-integration.md)      | Game developers integrating the API into a web or native app |
| [`docs/maintainer.md`](docs/maintainer.md)                      | Maintainers running, operating, and changing this API        |

The README is only an entry point: features, quick start, and an endpoint summary.

## Features

- Single-use sessions with configurable expiration.
- AES-256-GCM score payloads using a per-session 32-byte secret.
- Session secrets encrypted at rest with a separate 32-byte server key.
- Server-derived player identity and atomic session consumption.
- PostgreSQL advisory-lock protection for the active-session limit.
- Daily and all-time top-10 leaderboards with deterministic tie-breaking.
- Display-only profanity masking of player names, folded for case and Vietnamese diacritics.
- UTF-8, control-character, zero-width, timestamp, duration, and score-rate validation.
- Strict JSON decoding that rejects unknown fields and trailing values.
- Configurable rate limits, HTTP timeouts, database pool limits, and trusted proxies.
- `GET /healthz`, `GET /readyz`, and `GET /metrics` for operations.

## Quick start

```bash
cp .env.example .env
openssl rand -base64 32
# Put the generated value in SESSION_SECRET_ENCRYPTION_KEY in .env

docker compose up --build
```

Compose starts PostgreSQL with `database/schema.sql` and then the API. Verify with `http://localhost:8080/healthz`.

Native setup, the full configuration table, container usage, and the development checklist live in [`docs/maintainer.md`](docs/maintainer.md).

## API at a glance

All game endpoints are under `/v1/game/:gameCode`; the only supported game code is `threadman`.

| Endpoint                              | Purpose                          | Default rate limit |
| ------------------------------------- | -------------------------------- | ------------------ |
| `POST /v1/game/:gameCode/session`     | Create a single-use session      | 20/min/IP          |
| `POST /v1/game/:gameCode/score`       | Submit one encrypted score       | 10/min/IP          |
| `GET  /v1/game/:gameCode/leaderboard` | Daily (`?date=`) or all-time (`?period=all-time`) top 10 | 60/min/IP |
| `GET /healthz`                        | Process health; no database      | —                  |
| `GET /readyz`                         | PostgreSQL readiness             | —                  |
| `GET /metrics`                        | Prometheus-compatible metrics    | —                  |

```json
{
  "game_code": "threadman",
  "period": "day",
  "score_date": "2026-09-07",
  "top10": [
    { "rank": 1, "player_name": "Ace", "score": 3200 },
    { "rank": 2, "player_name": "Blitz", "score": 2100 }
  ]
}
```

```json
{
  "game_code": "threadman",
  "period": "all-time",
  "top10": [
    { "rank": 1, "player_name": "Ace", "score": 9900 },
    { "rank": 2, "player_name": "Blitz", "score": 7400 }
  ]
}
```

Errors share one shape:

```json
{ "error": "session expired", "request_id": "6b1c4e3b-..." }
```

Common statuses: `400` validation errors, `404` unknown game/session, `409` consumed session, `415` invalid content type, `429` rate limit or active-session limit, and `503` database readiness failure.

Request/response contracts, the encryption protocol, and client code samples are in [`docs/client-integration.md`](docs/client-integration.md).

## Development

```bash
make fmt
make test
make race
make vet
make build
```

Requirements, local setup, configuration, schema policy, security model, testing, and operations are documented in [`docs/maintainer.md`](docs/maintainer.md).

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
├── docs/                       # Maintainer and client integration guides
├── docker-compose.yml          # Local PostgreSQL + API environment
├── Makefile                    # Repeatable development commands
├── Dockerfile
└── README.md
```
