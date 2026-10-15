# Daily Challenge metrics

Two sources:

1. **`/metrics`** (Prometheus text, process lifetime, resets on deploy):
   - `game_sessions_created_total{game="threadman"|"threadman_daily"}`
   - `game_scores_submitted_total{game=...}`
   - `daily_replay_rejected_total`: second attempts refused (session or score). This should stay small; a spike means the client lets players retry.
   - `daily_lookups_total`: `GET /daily` calls (roughly mode-select opens).
2. **SQL** for per-day numbers (run with `psql "$DATABASE_URL"`, e.g. `dokku postgres:connect <db>` on the VPS).

## Daily players per day (last 14 days)

```sql
SELECT score_date, COUNT(*) AS players, ROUND(AVG(score)) AS avg_score, MAX(score) AS best
FROM game_scores
WHERE game_code = 'threadman_daily' AND score_date > CURRENT_DATE - 14
GROUP BY score_date ORDER BY score_date DESC;
```

## Started vs. finished daily runs (abandon rate)

```sql
SELECT created_at::date AS day,
       COUNT(*) AS sessions,
       COUNT(submitted_at) AS submitted,
       ROUND(100.0 * COUNT(submitted_at) / NULLIF(COUNT(*), 0), 1) AS pct_submitted
FROM game_sessions
WHERE game_code = 'threadman_daily' AND created_at > NOW() - INTERVAL '14 days'
GROUP BY 1 ORDER BY 1 DESC;
```

Unsubmitted sessions are pruned after they expire plus 24 h, so read this for recent days only.

## Share of daily players who also played the day before (D1 return)

```sql
SELECT t.score_date,
       COUNT(*) AS players,
       COUNT(y.player_id) AS returning,
       ROUND(100.0 * COUNT(y.player_id) / COUNT(*), 1) AS pct_returning
FROM game_scores t
LEFT JOIN game_scores y
  ON y.game_code = 'threadman_daily' AND y.player_id = t.player_id AND y.score_date = t.score_date - 1
WHERE t.game_code = 'threadman_daily' AND t.score_date > CURRENT_DATE - 14
GROUP BY t.score_date ORDER BY t.score_date DESC;
```

## Current streak distribution

```sql
WITH d AS (
  SELECT player_id, score_date,
         score_date - (ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY score_date))::int AS grp
  FROM game_scores WHERE game_code = 'threadman_daily'
), runs AS (
  SELECT player_id, MAX(score_date) AS last_day, COUNT(*) AS len FROM d GROUP BY player_id, grp
)
SELECT len AS streak, COUNT(*) AS players
FROM runs
WHERE last_day >= CURRENT_DATE - 1   -- still alive (today or yesterday)
GROUP BY len ORDER BY len;
```

`CURRENT_DATE` is the database's date. If the leaderboard timezone (`score_date`) differs from the DB timezone, use `(NOW() AT TIME ZONE '<leaderboard tz>')::date` instead.
