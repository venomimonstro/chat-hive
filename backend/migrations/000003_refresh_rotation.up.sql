CREATE TABLE session_refresh_history (
    token_hash BYTEA PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    consumed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX session_refresh_history_session_idx
    ON session_refresh_history (session_id, consumed_at DESC);
