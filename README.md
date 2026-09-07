# Threadman Leaderboard API

A small, production-oriented leaderboard API for the Threadman game. It is written in Go with Fiber v3 and PostgreSQL (`pgx/v5`). Players do not need accounts: the client creates a local UUID, obtains a short-lived game session, and submits one encrypted score.

## What it provides

- Single-use sessions that expire after 10 minutes.
- AES-128-GCM score payloads with a per-session secret.
- Server-derived player identity; score submissions cannot choose another `player_id`.
- Atomic session consumption to prevent replay and concurrent double-submit races.
- Daily top-10 leaderboards with deterministic score, creation-time, and ID tie-breaking.
- Name sanitization, timestamp freshness checks, duration limits, and a score-rate sanity check.
- Per-route rate limits and a background worker for abandoned-session cleanup.

Client-authoritative scoring is not cheat-proof. A user controlling a game client can still instrument or modify it. This service limits replay, tampering, accidental abuse, and leaderboard presentation attacks; authoritative scoring requires trusted server-side gameplay state.

## Project layout

```text
.
├── cmd/server/                 # Process entry point and graceful shutdown
├── internal/app/               # Fiber app composition and background workers
├── internal/config/            # Environment configuration
├── internal/database/          # PostgreSQL pool setup
├── internal/httpapi/           # Routes, middleware, request/response mapping
├── internal/leaderboard/       # Domain models, service, and PostgreSQL repository
├── internal/security/          # Encryption, names, and gameplay validation
├── database/schema.sql         # Idempotent PostgreSQL schema
└── public/robots.txt
```

The dependency direction is intentionally one-way: HTTP handlers call the leaderboard service; the service depends on a repository interface; the PostgreSQL implementation stays behind that interface. This keeps business rules testable without a live database and prevents the executable entry point from accumulating application logic.

## Requirements

- Go 1.27 or newer (see `go.mod`)
- PostgreSQL 14 or newer
- PostgreSQL extension `pgcrypto` for `gen_random_uuid()`

## Local setup

1. Create a database and enable the UUID function:

   ```sql
   CREATE DATABASE game_db;
   \c game_db
   CREATE EXTENSION IF NOT EXISTS pgcrypto;
   ```

2. Copy and edit the environment file:

   ```bash
   cp .env.example .env
   ```

   Required setting:

   ```env
   DATABASE_URL=postgres://postgres:password@localhost:5432/game_db?sslmode=disable
   ```

3. Apply the schema after loading the local environment:

   ```bash
   set -a
   source .env
   set +a
   psql "$DATABASE_URL" -f database/schema.sql
   ```

4. Start the API:

   ```bash
   go run ./cmd/server
   ```

The server listens on `:8080` by default. Load `.env` is supported automatically when present.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | required | PostgreSQL connection URL |
| `LEADERBOARD_TIMEZONE` | `UTC` | IANA timezone used to determine the leaderboard date |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS origins; trailing slashes are removed |

Use a restricted origin list outside local development. The API does not use `TRUST_PROXY` or `PROXY_HEADER`; configure client IP handling at the reverse proxy and verify the Fiber deployment settings before relying on IP-based rate limiting in production.

## API

All endpoints are under `/v1/game/:gameCode`. The currently supported game code is `threadman`.

### Create a session

`POST /v1/game/threadman/session`

Rate limit: 20 requests/minute/IP. Requires `Content-Type: application/json`.

```json
{"player_id":"9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"}
```

Response: `201 Created`

```json
{
  "session_id": "a81bc81b-dead-4e5d-b234-a123456789ab",
  "session_secret": "4f8a3c9b2d1e0f7a6b5c4d3e2f1a0b9c",
  "expires_at": "2026-09-07T08:30:00Z"
}
```

A player may have at most five active sessions. The returned secret is needed to encrypt the score and should be treated as sensitive, short-lived client state.

### Submit a score

`POST /v1/game/threadman/score`

Rate limit: 10 requests/minute/IP. Requires `Content-Type: application/json`.

```json
{
  "session_id": "a81bc81b-dead-4e5d-b234-a123456789ab",
  "payload": "Base64(12-byte nonce + AES-GCM ciphertext + 16-byte tag)"
}
```

Before encryption, the payload is JSON:

```json
{
  "player_name": "Speedy",
  "score": 1450,
  "duration_ms": 35200,
  "timestamp": 1757232000
}
```

The AES key is the 16-byte value represented by `session_secret`. Use a random 12-byte nonce, encrypt with AES-GCM, concatenate `nonce || ciphertext_and_tag`, and Base64-encode the result. The timestamp must be within ±120 seconds of server time; durations must be 3 seconds to 1 hour; and scores are capped at 200 points per elapsed second.

Response: `200 OK`

```json
{
  "submitted_score": {
    "id": 84920,
    "player_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
    "name": "Speedy",
    "score": 1450,
    "rank": 3,
    "score_date": "2026-09-07"
  },
  "top10_today": [
    {"rank": 1, "player_name": "Ace", "score": 3200},
    {"rank": 2, "player_name": "Blitz", "score": 2100},
    {"rank": 3, "player_name": "Speedy", "score": 1450}
  ]
}
```

### Get the leaderboard

`GET /v1/game/threadman/leaderboard`

Rate limit: 60 requests/minute/IP.

```json
{
  "game_code": "threadman",
  "score_date": "2026-09-07",
  "top10": [
    {"rank": 1, "player_name": "Ace", "score": 3200},
    {"rank": 2, "player_name": "Blitz", "score": 2100}
  ]
}
```

Common errors use `{"error":"..."}` and include `400` validation failures, `404` unknown games/sessions, `409` consumed sessions, `415` missing or invalid JSON content type, and `429` rate-limit responses.

## Database

`database/schema.sql` is safe to run repeatedly. It creates:

- `game_sessions`: short-lived session secrets and consumption state.
- `game_scores`: permanent daily leaderboard entries.
- Partial and ranking indexes for active-session pruning, top-10 reads, and deterministic rank calculation.

The worker deletes unsubmitted sessions older than 24 hours. Submitted scores are never removed by the worker.

## Development commands

```bash
# Format and test all packages
gofmt -w cmd internal
go test ./...

# Run static analysis available in the Go toolchain
go vet ./...

# Start locally
go run ./cmd/server

# Build a deployable binary
go build -o bin/homielab-api ./cmd/server

# Build the container image
docker build -t homielab-api .
```

Run the container with the same environment variables described above:

```bash
docker run --rm -p 8080:8080 --env-file .env homielab-api
```

## Deployment checklist

- Run behind TLS termination (Nginx, Cloudflare, or a managed load balancer).
- Set an explicit `ALLOWED_ORIGINS` value; do not use `*` for a browser-facing production deployment.
- Apply `database/schema.sql` with a migration process before starting the service.
- Keep `DATABASE_URL` and session secrets out of logs and source control.
- Set PostgreSQL connection limits with the pool settings in `internal/database/postgres.go` in mind.
- Configure reverse-proxy client-IP forwarding deliberately because rate limiting is IP-based.
- Monitor database errors, rate-limit responses, and the session-pruning worker.
- Tune `ValidateScore` in `internal/security/validation.go` for the actual game mechanics.
