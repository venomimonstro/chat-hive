CREATE UNIQUE INDEX growth_events_user_milestone_unique
ON growth_events(user_id, event_name)
WHERE user_id IS NOT NULL
  AND event_name IN ('signup_completed','first_message','first_post');
