CREATE TABLE growth_events (
    id BIGSERIAL PRIMARY KEY,
    event_name TEXT NOT NULL CHECK (event_name IN (
        'public_view','login_started','signup_completed','follow',
        'community_join','channel_subscribe','invite_join','first_post','first_message'
    )),
    acquisition_id UUID,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    object_type TEXT NOT NULL DEFAULT '' CHECK (object_type IN ('','profile','post','community','channel','group_invite')),
    object_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '' CHECK (char_length(source) <= 64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX growth_events_name_created_idx ON growth_events(event_name, created_at DESC);
CREATE INDEX growth_events_acquisition_idx ON growth_events(acquisition_id, created_at) WHERE acquisition_id IS NOT NULL;
CREATE INDEX growth_events_user_created_idx ON growth_events(user_id, created_at DESC) WHERE user_id IS NOT NULL;
