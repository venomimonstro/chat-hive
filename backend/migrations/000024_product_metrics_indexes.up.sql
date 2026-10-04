CREATE INDEX users_created_idx
ON users(created_at DESC);

CREATE INDEX profiles_onboarding_completed_idx
ON profiles(onboarding_completed_at DESC)
WHERE onboarding_completed_at IS NOT NULL;

CREATE INDEX sessions_last_seen_user_idx
ON sessions(last_seen_at DESC, user_id);

CREATE INDEX follows_created_follower_idx
ON follows(created_at DESC, follower_id);

CREATE INDEX messages_sender_created_idx
ON messages(sender_id, created_at DESC)
WHERE sender_id IS NOT NULL;

CREATE INDEX growth_events_created_name_idx
ON growth_events(created_at DESC, event_name);
