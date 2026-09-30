CREATE TABLE media_objects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_type TEXT NOT NULL CHECK (media_type IN ('image')),
    storage_key TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size > 0),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    sha256 BYTEA NOT NULL,
    state TEXT NOT NULL DEFAULT 'ready' CHECK (state IN ('processing','ready','quarantined','deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX media_objects_owner_created_idx
    ON media_objects(owner_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE post_media (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES media_objects(id) ON DELETE RESTRICT,
    position SMALLINT NOT NULL CHECK (position >= 0 AND position < 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, media_id),
    UNIQUE (post_id, position)
);

CREATE INDEX post_media_media_idx ON post_media(media_id);

CREATE TABLE message_attachments (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES media_objects(id) ON DELETE RESTRICT,
    position SMALLINT NOT NULL CHECK (position >= 0 AND position < 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, media_id),
    UNIQUE (message_id, position)
);

CREATE INDEX message_attachments_media_idx ON message_attachments(media_id);
