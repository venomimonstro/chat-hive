CREATE TABLE chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('direct', 'group')),
    title TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    next_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_sequence > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE direct_chat_pairs (
    chat_id UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    user_low UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT direct_pair_order CHECK (user_low::text < user_high::text),
    UNIQUE (user_low, user_high)
);

CREATE TABLE chat_members (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    last_read_sequence BIGINT NOT NULL DEFAULT 0 CHECK (last_read_sequence >= 0),
    muted_until TIMESTAMPTZ,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX chat_members_user_active_idx
    ON chat_members (user_id, joined_at DESC, chat_id)
    WHERE left_at IS NULL;

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID REFERENCES users(id) ON DELETE SET NULL,
    client_message_id UUID NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    type TEXT NOT NULL DEFAULT 'text' CHECK (type IN ('text', 'image', 'system')),
    body TEXT NOT NULL DEFAULT '',
    reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    UNIQUE (chat_id, sequence),
    UNIQUE (sender_id, client_message_id),
    CONSTRAINT text_message_body CHECK (type <> 'text' OR char_length(body) BETWEEN 1 AND 4096)
);

CREATE INDEX messages_chat_sequence_idx ON messages (chat_id, sequence DESC);
CREATE INDEX messages_sender_created_idx ON messages (sender_id, created_at DESC);

CREATE TABLE message_reactions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction TEXT NOT NULL CHECK (char_length(reaction) BETWEEN 1 AND 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, reaction)
);
