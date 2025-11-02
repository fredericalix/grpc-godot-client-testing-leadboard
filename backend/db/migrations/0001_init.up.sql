-- Create the scores table with constraints
CREATE TABLE scores (
    player_name TEXT PRIMARY KEY,
    score BIGINT NOT NULL CHECK (score >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Enforce max 20 character player_name at database level
    CONSTRAINT player_name_length CHECK (char_length(player_name) <= 20 AND char_length(player_name) > 0)
);

-- Create index for efficient leaderboard queries
-- This index supports ORDER BY score DESC, player_name for pagination and ranking
CREATE INDEX idx_scores_leaderboard ON scores (score DESC, player_name);

-- Create trigger function to notify on score changes
-- Only emits NOTIFY when a player's best score actually improves or is created
CREATE OR REPLACE FUNCTION notify_score_change()
RETURNS TRIGGER AS $$
DECLARE
    payload JSON;
    operation TEXT;
BEGIN
    -- Determine the operation type
    IF TG_OP = 'DELETE' THEN
        operation := 'delete';
        payload := json_build_object(
            'player_name', OLD.player_name,
            'score', OLD.score,
            'op', operation
        );
        PERFORM pg_notify('scores_changes', payload::text);
        RETURN OLD;
    ELSIF TG_OP = 'INSERT' THEN
        operation := 'insert';
        payload := json_build_object(
            'player_name', NEW.player_name,
            'score', NEW.score,
            'op', operation
        );
        PERFORM pg_notify('scores_changes', payload::text);
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        -- Notify if the score actually changed (any change, not just improvements)
        IF NEW.score <> OLD.score THEN
            operation := 'update';
            payload := json_build_object(
                'player_name', NEW.player_name,
                'score', NEW.score,
                'op', operation
            );
            PERFORM pg_notify('scores_changes', payload::text);
        END IF;
        RETURN NEW;
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Create trigger that fires after INSERT, UPDATE, or DELETE
CREATE TRIGGER scores_change_trigger
AFTER INSERT OR UPDATE OR DELETE ON scores
FOR EACH ROW
EXECUTE FUNCTION notify_score_change();

-- Add a comment explaining the NOTIFY channel
COMMENT ON FUNCTION notify_score_change() IS
'Sends notifications on channel scores_changes with JSON payload: {"player_name":"...", "score":12345, "op":"insert|update|delete"}. Notifies on any score change (increase or decrease).';
