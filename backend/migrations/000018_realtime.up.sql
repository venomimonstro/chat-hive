CREATE OR REPLACE FUNCTION realtime_message_notify() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify(
        'chat_events',
        json_build_object(
            'type','message.created',
            'chat_id',NEW.chat_id,
            'message_id',NEW.id,
            'sender_id',NEW.sender_id,
            'sequence',NEW.sequence,
            'occurred_at',NEW.created_at
        )::text
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER messages_realtime_after_insert
AFTER INSERT ON messages
FOR EACH ROW EXECUTE FUNCTION realtime_message_notify();
