CREATE TABLE platform_feature_flags (
    key TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    reason TEXT NOT NULL DEFAULT '',
    updated_by UUID REFERENCES admin_users(user_id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO platform_feature_flags(key, enabled) VALUES
    ('feed.enabled', TRUE),
    ('discovery.enabled', TRUE),
    ('posting.enabled', TRUE),
    ('community_creation.enabled', TRUE),
    ('channel_creation.enabled', TRUE),
    ('media_upload.enabled', TRUE)
ON CONFLICT (key) DO NOTHING;
