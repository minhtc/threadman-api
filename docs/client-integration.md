# Client Integration Guide

This guide explains how a web, mobile, Godot, Unity, or other game client integrates with the Threadman Leaderboard API. It is the complete contract for game developers: endpoints, limits, payloads, encryption, and error handling. Server setup, configuration, and internal architecture are out of scope here and live in [`maintainer.md`](maintainer.md).

The integration has three steps:

1. Create or load a persistent local `player_id`.
2. Create a short-lived single-use game session.
3. Encrypt and submit one score using the session secret.

The client never receives or needs the server's `SESSION_SECRET_ENCRYPTION_KEY`. That key protects secrets stored in PostgreSQL. The client only receives the per-session AES-256 key returned by the session endpoint.

## API base URL

Use the deployed API origin in production and the local origin during development:

```text
Development: http://localhost:8080
Production:  https://threadman-api.homielab.com
```

Do not hardcode a production URL into a reusable client library. Make the base URL configurable.

## Requirements and limits

- Supported game code: `threadman`
- `player_id`: UUID string
- Session lifetime: 10 minutes by default
- A session can submit exactly one score.
- A player can have at most five active sessions per game by default.
- Player name: 2–24 Unicode characters after trimming
- Score: non-negative integer
- Duration: `3000`–`3600000` milliseconds
- Timestamp drift: at most ±120 seconds from server time
- Score limit: at most 200 points per elapsed second
- Request bodies must use `Content-Type: application/json`.
- Unknown JSON fields are rejected.

The server configuration can change these operational limits, so clients should handle validation errors rather than relying only on the defaults.

## 1. Create a persistent player ID

The API does not provide accounts. Generate a UUID once and persist it locally. Do not generate a new UUID for every game; doing so makes one player appear as many players on the leaderboard.

Browser example:

```js
function getPlayerId() {
  const storageKey = "threadman.player_id";
  let playerId = localStorage.getItem(storageKey);

  if (!playerId) {
    playerId = crypto.randomUUID();
    localStorage.setItem(storageKey, playerId);
  }

  return playerId;
}
```

For native clients, persist the UUID in the platform's local preferences or secure app storage. The player ID is an anonymous client identifier, not an authentication credential.

## 2. Create a game session

Request:

```http
POST /v1/game/threadman/session
Content-Type: application/json
```

```json
{
  "player_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
}
```

Example response:

```http
201 Created
X-Request-ID: 7b9e...
Content-Type: application/json
```

```json
{
  "session_id": "a81bc81b-dead-4e5d-b234-a123456789ab",
  "session_secret": "64-lowercase-hex-characters-for-aes-256",
  "expires_at": "2026-09-07T08:30:00Z"
}
```

Store the returned values in memory for the current attempt. The session secret is short-lived sensitive data; do not log it, send it to analytics, or persist it longer than necessary.

If the request fails with `429`, wait before retrying. Do not create multiple sessions speculatively; the API limits concurrent active sessions.

## 3. Encrypt the score payload

The plaintext score object is:

```json
{
  "player_name": "Speedy",
  "score": 1450,
  "duration_ms": 35200,
  "timestamp": 1757232000
}
```

Encryption protocol:

1. Decode `session_secret` from lowercase hexadecimal into exactly 32 bytes.
2. Serialize the score object as UTF-8 JSON.
3. Generate a fresh random 12-byte nonce for this submission.
4. Encrypt with AES-256-GCM.
5. Concatenate `nonce || ciphertext_and_tag`.
6. Base64-encode the concatenated bytes.

Web Crypto's AES-GCM output already includes the 16-byte authentication tag at the end of the ciphertext. Do not append a second tag.

Wire format:

```text
Base64(12-byte nonce + ciphertext + 16-byte GCM authentication tag)
```

Never reuse a nonce with the same session secret. The helper below generates a new nonce every time.

## Complete browser implementation

This example creates a session, encrypts a score, submits it, and returns the API response.

```js
const API_BASE_URL = "http://localhost:8080";
const GAME_CODE = "threadman";

function getPlayerId() {
  const storageKey = "threadman.player_id";
  let playerId = localStorage.getItem(storageKey);
  if (!playerId) {
    playerId = crypto.randomUUID();
    localStorage.setItem(storageKey, playerId);
  }
  return playerId;
}

function hexToBytes(hex) {
  if (!/^[0-9a-fA-F]+$/.test(hex) || hex.length % 2 !== 0) {
    throw new Error("session_secret is not valid hexadecimal");
  }

  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < bytes.length; i += 1) {
    bytes[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

function bytesToBase64(bytes) {
  let binary = "";
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
  }
  return btoa(binary);
}

async function encryptScore(sessionSecret, scoreData) {
  const rawKey = hexToBytes(sessionSecret);
  if (rawKey.length !== 32) {
    throw new Error("session_secret must contain 32 bytes for AES-256-GCM");
  }

  const key = await crypto.subtle.importKey(
    "raw",
    rawKey,
    { name: "AES-GCM" },
    false,
    ["encrypt"],
  );

  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const plaintext = new TextEncoder().encode(JSON.stringify(scoreData));
  const ciphertextWithTag = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    plaintext,
  );

  const encrypted = new Uint8Array(nonce.length + ciphertextWithTag.byteLength);
  encrypted.set(nonce, 0);
  encrypted.set(new Uint8Array(ciphertextWithTag), nonce.length);
  return bytesToBase64(encrypted);
}

async function readError(response) {
  let body;
  try {
    body = await response.json();
  } catch {
    body = {};
  }

  const requestId = body.request_id || response.headers.get("X-Request-ID");
  const message = body.error || `Request failed with HTTP ${response.status}`;
  return new Error(
    requestId ? `${message} (request_id: ${requestId})` : message,
  );
}

async function createSession(playerId) {
  const response = await fetch(`${API_BASE_URL}/v1/game/${GAME_CODE}/session`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId }),
  });

  if (!response.ok) throw await readError(response);
  return response.json();
}

async function submitScore(playerName, score, durationMs) {
  const playerId = getPlayerId();
  const session = await createSession(playerId);

  const payload = await encryptScore(session.session_secret, {
    player_name: playerName,
    score: Math.trunc(score),
    duration_ms: Math.trunc(durationMs),
    timestamp: Math.trunc(Date.now() / 1000),
  });

  const response = await fetch(`${API_BASE_URL}/v1/game/${GAME_CODE}/score`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      session_id: session.session_id,
      payload,
    }),
  });

  if (!response.ok) throw await readError(response);
  return response.json();
}

// Example:
// const result = await submitScore("Speedy", 1450, 35200);
// console.log(result.submitted_score, result.top10_today);
```

## Submission lifecycle

Create the session immediately before the playable attempt, not when the game page first opens:

```text
open game
  └─ request session
       └─ play
            └─ build plaintext score
                 └─ encrypt with session secret
                      └─ submit once
```

If the game is abandoned, let the session expire. Do not submit placeholder scores.

The session is consumed atomically with score insertion. If the client receives a timeout after submission, it cannot know whether the server committed the score. Before retrying, treat the session as potentially consumed and query the leaderboard or show a recoverable “submission status unknown” state. A second submission may correctly return `409`.

## Read the leaderboard

Current day:

```http
GET /v1/game/threadman/leaderboard
```

Specific day:

```http
GET /v1/game/threadman/leaderboard?date=2026-09-07
```

The date must use exact `YYYY-MM-DD` format. The server interprets a missing date using its configured `LEADERBOARD_TIMEZONE`.

All-time top 10 (every recorded day):

```http
GET /v1/game/threadman/leaderboard?period=all-time
```

`period=all-time` cannot be combined with `date`; sending both returns `400`. Any other `period` value is also rejected with `400`.

Example:

```js
async function getLeaderboard({ date, period } = {}) {
  const params = new URLSearchParams();
  if (date) params.set("date", date);
  if (period) params.set("period", period);
  const query = params.size ? `?${params}` : "";
  const response = await fetch(
    `${API_BASE_URL}/v1/game/${GAME_CODE}/leaderboard${query}`,
  );
  if (!response.ok) throw await readError(response);
  return response.json();
}

// await getLeaderboard();                    // today
// await getLeaderboard({ date: "2026-09-07" });
// await getLeaderboard({ period: "all-time" });
```

Response:

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

All-time response, which has no `score_date`:

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

An empty `top10` array is a valid response when no scores exist for the selected date or period.

### Player name masking

`player_name` is stored exactly as submitted, but every name the API returns is masked for display. A listed word has its first two characters replaced with `**` and keeps the rest, so a name submitted as `fucker` comes back as `**cker` and `shit` as `**it`. Vietnamese diacritics and letter case are ignored when matching, but the returned name keeps the player's original spelling.

Masking is whole-word only, plus common word endings, so names that merely contain a listed short word are returned unchanged. Render `player_name` as returned and do not attempt to reverse or reconstruct it; note that masking changes the length of a name, so size UI elements to fit the masked form.

## Error handling

All JSON errors have this shape:

```json
{
  "error": "session expired",
  "request_id": "rBe2_s5B5xqVxnjtXussku03iYCBk2JgSp8sz6vevKg"
}
```

| Status | Meaning                                                                     | Client behavior                                              |
| ------ | --------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `400`  | Invalid payload, name, score, duration, timestamp, date, period, or expired session | Show validation feedback; do not blindly retry          |
| `404`  | Unsupported game or unknown session                                         | Check the game code/session state                            |
| `409`  | Session already used or unavailable                                         | Treat the attempt as finished; refresh leaderboard if needed |
| `415`  | Missing/incorrect JSON content type                                         | Fix the request headers                                      |
| `429`  | Rate limit or active-session limit                                          | Back off and retry later                                     |
| `500`  | Server/database failure                                                     | Retry safe reads with backoff; do not blindly replay a score |
| `503`  | Database readiness failure                                                  | Retry after a delay                                          |

Use exponential backoff for safe leaderboard reads. Avoid automatic score replay because score submission consumes a single-use session.

## Security and gameplay notes

- HTTPS is required in production. AES-GCM does not replace TLS.
- Do not log `session_secret`, encrypted payloads, or player data unnecessarily.
- The session secret is not an authentication token and does not identify the player by itself.
- Client-side encryption cannot prevent a modified client from creating a valid encrypted payload.
- Keep score and duration as integers. The server rejects negative scores and out-of-range durations.
- Send the actual gameplay timestamp at submission time; do not reuse a page-load timestamp.
- Player names may contain Unicode but cannot contain control, zero-width, or BiDi override characters.

## Native-client mapping

The protocol is engine-neutral:

```text
UUID generation/storage       → platform UUID + local preferences
POST JSON                     → HTTP client
64-char hex → 32 bytes       → hex decoder
AES-256-GCM                   → platform crypto library
12 random bytes               → secure random generator
nonce || ciphertext || tag   → byte array concatenation
Base64                       → standard Base64 encoder
```

Use the platform's authenticated AES-GCM API. Do not implement AES-GCM manually and do not use a non-cryptographic random generator for the nonce.
