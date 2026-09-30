DROP TABLE IF EXISTS message_request_events;
DROP INDEX IF EXISTS direct_chat_pairs_request_target_idx;
ALTER TABLE direct_chat_pairs
    DROP COLUMN IF EXISTS accepted_at,
    DROP COLUMN IF EXISTS requested_by,
    DROP COLUMN IF EXISTS request_state;
