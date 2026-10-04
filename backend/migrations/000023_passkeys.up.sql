CREATE TABLE passkey_credentials (
    credential_id BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential JSONB NOT NULL,
    label TEXT NOT NULL DEFAULT '' CHECK (char_length(label) <= 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX passkey_credentials_user_idx
ON passkey_credentials(user_id, created_at);

CREATE TABLE passkey_ceremonies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('registration','login')),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_data JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX passkey_ceremonies_expiry_idx
ON passkey_ceremonies(expires_at)
WHERE consumed_at IS NULL;
