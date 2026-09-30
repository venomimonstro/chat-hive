CREATE OR REPLACE FUNCTION reject_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit records are append-only';
END;
$$;

CREATE TRIGGER admin_audit_events_append_only
BEFORE UPDATE OR DELETE ON admin_audit_events
FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();

CREATE TRIGGER moderation_actions_append_only
BEFORE UPDATE OR DELETE ON moderation_actions
FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();

CREATE TABLE security_events (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info','low','medium','high','critical')),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    source_ip INET,
    subject_type TEXT NOT NULL DEFAULT '',
    subject_id TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX security_events_severity_created_idx ON security_events(severity, created_at DESC);
CREATE INDEX security_events_user_created_idx ON security_events(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX security_events_type_created_idx ON security_events(event_type, created_at DESC);

CREATE TRIGGER security_events_append_only
BEFORE UPDATE OR DELETE ON security_events
FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();
