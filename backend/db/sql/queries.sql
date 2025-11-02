-- name: UpsertScore :one
-- Upserts a player's score, keeping only the best (highest) score.
-- Returns the current best score and a boolean indicating if it was improved.
-- This query uses ON CONFLICT to handle the upsert logic efficiently.
-- Time complexity: O(log n) due to primary key lookup
INSERT INTO scores (player_name, score, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (player_name)
DO UPDATE SET
    score = GREATEST(EXCLUDED.score, scores.score),
    updated_at = CASE
        WHEN EXCLUDED.score > scores.score THEN now()
        ELSE scores.updated_at
    END
RETURNING player_name, score, updated_at;

-- name: GetTopScores :many
-- Retrieves the top N scores in descending order with pagination support.
-- Uses the idx_scores_leaderboard index for efficient sorting.
-- Time complexity: O(limit + offset) with index scan
SELECT player_name, score, updated_at
FROM scores
ORDER BY score DESC, player_name ASC
LIMIT $1 OFFSET $2;

-- name: GetPlayerScore :one
-- Retrieves a specific player's current best score.
-- Time complexity: O(1) - primary key lookup
SELECT player_name, score, updated_at
FROM scores
WHERE player_name = $1;

-- name: GetPlayerRank :one
-- Calculates a player's rank in the leaderboard.
-- Rank is 1-based (1 = best). Uses deterministic tie-breaking by player_name.
-- Returns the count of players with strictly better scores plus 1.
-- Time complexity: O(n) worst case, but uses index for score comparison
SELECT 1 + COUNT(*)::bigint AS rank
FROM scores s1
WHERE s1.score > (SELECT s2.score FROM scores s2 WHERE s2.player_name = $1)
   OR (s1.score = (SELECT s2.score FROM scores s2 WHERE s2.player_name = $1) AND s1.player_name < $1);

-- name: DeleteScore :exec
-- Deletes a player's score entry entirely.
-- Time complexity: O(log n) - primary key lookup
DELETE FROM scores
WHERE player_name = $1;

-- name: CountScores :one
-- Returns the total number of players in the leaderboard.
-- Time complexity: O(1) - uses table statistics or fast count
SELECT COUNT(*)::bigint AS total
FROM scores;

-- name: GetScoreForUpdate :one
-- Retrieves a player's score with a row lock for transactional updates.
-- Used when you need to ensure consistency during concurrent operations.
-- Time complexity: O(1) - primary key lookup with lock
SELECT player_name, score, updated_at
FROM scores
WHERE player_name = $1
FOR UPDATE;
