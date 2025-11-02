-- Drop trigger first
DROP TRIGGER IF EXISTS scores_change_trigger ON scores;

-- Drop the trigger function
DROP FUNCTION IF EXISTS notify_score_change();

-- Drop the index
DROP INDEX IF EXISTS idx_scores_leaderboard;

-- Drop the table
DROP TABLE IF EXISTS scores;
