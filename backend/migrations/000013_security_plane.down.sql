DROP TRIGGER IF EXISTS security_events_append_only ON security_events;
DROP TABLE IF EXISTS security_events;
DROP TRIGGER IF EXISTS moderation_actions_append_only ON moderation_actions;
DROP TRIGGER IF EXISTS admin_audit_events_append_only ON admin_audit_events;
DROP FUNCTION IF EXISTS reject_audit_mutation();
