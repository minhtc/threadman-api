-- Development schema for the Threadman Leaderboard API.
-- Apply with: psql "$DATABASE_URL" -f database/schema.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS game_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_code VARCHAR(32) NOT NULL,
    player_id UUID NOT NULL,
    -- Base64 AES-GCM envelope containing the AES-256 session secret.
    session_secret TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_active
ON game_sessions (game_code, player_id, expires_at)
WHERE submitted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_pruning
ON game_sessions (expires_at)
WHERE submitted_at IS NULL;

CREATE TABLE IF NOT EXISTS game_scores (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL UNIQUE,
    game_code VARCHAR(32) NOT NULL,
    player_id UUID NOT NULL,
    player_name VARCHAR(32) NOT NULL,
    score BIGINT NOT NULL,
    duration_ms INTEGER NOT NULL,
    score_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_scores_non_negative CHECK (score >= 0),
    CONSTRAINT chk_duration_bounds CHECK (duration_ms >= 3000 AND duration_ms <= 3600000),
    CONSTRAINT chk_name_length CHECK (char_length(player_name) >= 2 AND char_length(player_name) <= 24)
);

CREATE INDEX IF NOT EXISTS idx_game_scores_ranking
ON game_scores (game_code, score_date, score DESC, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_game_scores_all_time
ON game_scores (game_code, score DESC, created_at ASC, id ASC);
