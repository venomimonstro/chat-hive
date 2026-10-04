DROP INDEX IF EXISTS sessions_user_auth_method_active_idx;
ALTER TABLE sessions DROP COLUMN IF EXISTS auth_method;
