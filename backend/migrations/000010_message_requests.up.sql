ALTER TABLE direct_chat_pairs
    ADD COLUMN request_state TEXT NOT NULL DEFAULT 'accepted'
        CHECK (request_state IN ('pending','accepted','rejected')),
    ADD COLUMN requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN accepted_at TIMESTAMPTZ;

CREATE INDEX direct_chat_pairs_request_target_idx
    ON direct_chat_pairs(request_state, requested_by)
    WHERE request_state = 'pending';

CREATE TABLE message_request_events (
    id BIGSERIAL PRIMARY KEY,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('created','accepted','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX message_request_events_chat_idx ON message_request_events(chat_id, created_at DESC);
