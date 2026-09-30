DROP TRIGGER IF EXISTS channel_posts_notify_after_insert ON channel_posts;
DROP FUNCTION IF EXISTS notify_channel_post_insert();
DROP TRIGGER IF EXISTS post_replies_notify_after_insert ON post_replies;
DROP FUNCTION IF EXISTS notify_post_reply_insert();
DROP TRIGGER IF EXISTS follows_notify_after_insert ON follows;
DROP FUNCTION IF EXISTS notify_follow_insert();
DROP TRIGGER IF EXISTS messages_notify_after_insert ON messages;
DROP FUNCTION IF EXISTS notify_message_insert();

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (
    kind IN ('direct_message','group_message','channel_post','follow','moderation','security')
);
