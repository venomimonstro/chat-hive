ALTER TABLE sessions
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'legacy'
    CHECK (auth_method IN ('legacy','email','yandex','passkey'));

CREATE INDEX sessions_user_auth_method_active_idx
ON sessions(user_id, auth_method, expires_at DESC)
WHERE revoked_at IS NULL;
