CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('thought','photo','post')),
    body TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','followers')),
    replies_count BIGINT NOT NULL DEFAULT 0 CHECK (replies_count >= 0),
    reactions_count BIGINT NOT NULL DEFAULT 0 CHECK (reactions_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX posts_author_created_idx ON posts(author_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX posts_public_created_idx ON posts(created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND visibility = 'public';

CREATE TABLE post_replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX post_replies_post_created_idx ON post_replies(post_id, created_at ASC, id ASC)
    WHERE deleted_at IS NULL;

CREATE TABLE post_reactions (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id, reaction)
);

CREATE INDEX post_reactions_post_idx ON post_reactions(post_id, reaction);

CREATE TABLE saved_posts (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);
