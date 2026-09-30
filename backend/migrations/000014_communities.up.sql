CREATE TABLE communities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    chat_id UUID NOT NULL UNIQUE REFERENCES chats(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9_-]{2,47}$'),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 2 AND 80),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    visibility TEXT NOT NULL CHECK (visibility IN ('private','public')),
    moderation_status TEXT NOT NULL CHECK (moderation_status IN ('not_required','pending','approved','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX communities_public_catalog_idx
    ON communities(moderation_status, updated_at DESC, id DESC)
    WHERE visibility='public';

CREATE TABLE community_members (
    community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','moderator','member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    PRIMARY KEY (community_id,user_id)
);

CREATE INDEX community_members_user_active_idx ON community_members(user_id,community_id) WHERE left_at IS NULL;

CREATE TABLE community_posts (
    community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    post_id UUID NOT NULL UNIQUE REFERENCES posts(id) ON DELETE CASCADE,
    pinned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (community_id,post_id)
);
