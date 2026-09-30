CREATE TABLE feed_feedback (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    signal TEXT NOT NULL CHECK (signal IN ('more_like_this','not_interested','hide')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, post_id)
);

CREATE INDEX feed_feedback_user_signal_idx ON feed_feedback(user_id, signal, updated_at DESC);
