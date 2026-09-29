CREATE TABLE login_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_normalized TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    request_ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE INDEX login_challenges_email_created_idx
    ON login_challenges (email_normalized, created_at DESC);

ALTER TABLE sessions
    ADD COLUMN access_token_hash BYTEA,
    ADD COLUMN access_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX sessions_access_token_unique
    ON sessions (access_token_hash)
    WHERE access_token_hash IS NOT NULL;

CREATE UNIQUE INDEX sessions_refresh_token_unique
    ON sessions (refresh_token_hash);
