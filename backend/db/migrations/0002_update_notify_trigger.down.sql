-- Revert to the original notify_score_change function (only notify on score improvements)
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
        -- Only notify if the score actually changed (improved)
        IF NEW.score > OLD.score THEN
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

-- Revert the comment
COMMENT ON FUNCTION notify_score_change() IS
'Sends notifications on channel scores_changes with JSON payload: {"player_name":"...", "score":12345, "op":"insert|update|delete"}. Only notifies on actual score improvements.';
