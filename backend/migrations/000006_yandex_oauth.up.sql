CREATE TABLE oauth_login_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL CHECK (provider IN ('yandex')),
    state_hash BYTEA NOT NULL UNIQUE,
    code_verifier TEXT NOT NULL,
    request_ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE INDEX oauth_login_states_active_idx
    ON oauth_login_states (provider, expires_at)
    WHERE consumed_at IS NULL;
