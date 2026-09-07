-- Operational table: Short-lived session state
CREATE TABLE IF NOT EXISTS game_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_code VARCHAR(32) NOT NULL,
    player_id UUID NOT NULL,
    session_secret VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ
);

-- Partial index for active session lookups & abuse checks
CREATE INDEX IF NOT EXISTS idx_sessions_active 
ON game_sessions (game_code, player_id, expires_at) 
WHERE submitted_at IS NULL;

-- Index for background cron pruning
CREATE INDEX IF NOT EXISTS idx_sessions_pruning 
ON game_sessions (expires_at) 
WHERE submitted_at IS NULL;

-- Permanent leaderboard table
CREATE TABLE IF NOT EXISTS game_scores (
    id BIGSERIAL PRIMARY KEY,
    -- Unique constraint guarantees one submission per session WITHOUT hard FK locking
    session_id UUID NOT NULL UNIQUE,
    game_code VARCHAR(32) NOT NULL,
    player_id UUID NOT NULL,
    player_name VARCHAR(32) NOT NULL,
    score BIGINT NOT NULL,
    duration_ms INTEGER NOT NULL,
    score_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Invariant defense-in-depth
    CONSTRAINT chk_scores_non_negative CHECK (score >= 0),
    CONSTRAINT chk_duration_bounds CHECK (duration_ms >= 3000 AND duration_ms <= 3600000),
    CONSTRAINT chk_name_length CHECK (char_length(player_name) >= 2 AND char_length(player_name) <= 24)
);

-- Fast Top-10 & deterministic Rank index (matches tie-breaker: score DESC, created_at ASC, id ASC)
CREATE INDEX IF NOT EXISTS idx_game_scores_ranking 
ON game_scores (game_code, score_date, score DESC, created_at ASC, id ASC);