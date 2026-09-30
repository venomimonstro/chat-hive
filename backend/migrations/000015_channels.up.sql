CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9_-]{2,47}$'),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 2 AND 80),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    moderation_status TEXT NOT NULL DEFAULT 'pending' CHECK (moderation_status IN ('pending','approved','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX channels_catalog_idx ON channels(moderation_status, updated_at DESC, id DESC);

CREATE TABLE channel_subscribers (
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id,user_id)
);

CREATE INDEX channel_subscribers_user_idx ON channel_subscribers(user_id,channel_id);

CREATE TABLE channel_posts (
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    post_id UUID NOT NULL UNIQUE REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id,post_id)
);

CREATE INDEX channel_posts_channel_created_idx ON channel_posts(channel_id,created_at DESC,post_id DESC);
