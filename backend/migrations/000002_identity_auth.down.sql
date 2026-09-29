DROP INDEX IF EXISTS sessions_refresh_token_unique;
DROP INDEX IF EXISTS sessions_access_token_unique;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS access_expires_at,
    DROP COLUMN IF EXISTS access_token_hash;
DROP TABLE IF EXISTS login_challenges;
