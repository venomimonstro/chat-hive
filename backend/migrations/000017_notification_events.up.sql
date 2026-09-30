ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (
    kind IN ('direct_message','group_message','channel_post','post_reply','follow','moderation','security')
);

CREATE OR REPLACE FUNCTION notify_message_insert() RETURNS trigger AS $$
DECLARE
    chat_kind TEXT;
BEGIN
    SELECT kind INTO chat_kind FROM chats WHERE id=NEW.chat_id;
    INSERT INTO notifications(user_id,kind,actor_id,entity_type,entity_id,title,body,dedupe_key)
    SELECT cm.user_id,
           CASE WHEN chat_kind='direct' THEN 'direct_message' ELSE 'group_message' END,
           NEW.sender_id,
           'chat',
           NEW.chat_id::text,
           CASE WHEN chat_kind='direct' THEN 'Новое сообщение' ELSE 'Новое сообщение в группе' END,
           LEFT(NEW.body, 180),
           'message:' || NEW.id::text || ':' || cm.user_id::text
    FROM chat_members cm
    WHERE cm.chat_id=NEW.chat_id
      AND cm.left_at IS NULL
      AND cm.user_id IS DISTINCT FROM NEW.sender_id
    ON CONFLICT (user_id,dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER messages_notify_after_insert
AFTER INSERT ON messages
FOR EACH ROW EXECUTE FUNCTION notify_message_insert();

CREATE OR REPLACE FUNCTION notify_follow_insert() RETURNS trigger AS $$
BEGIN
    INSERT INTO notifications(user_id,kind,actor_id,entity_type,entity_id,title,body,dedupe_key)
    VALUES(
        NEW.followed_id,
        'follow',
        NEW.follower_id,
        'user',
        NEW.follower_id::text,
        'Новый подписчик',
        '',
        'follow:' || NEW.follower_id::text || ':' || NEW.followed_id::text
    )
    ON CONFLICT (user_id,dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER follows_notify_after_insert
AFTER INSERT ON follows
FOR EACH ROW EXECUTE FUNCTION notify_follow_insert();

CREATE OR REPLACE FUNCTION notify_post_reply_insert() RETURNS trigger AS $$
DECLARE
    target_author UUID;
BEGIN
    SELECT author_id INTO target_author FROM posts WHERE id=NEW.post_id;
    IF target_author IS NOT NULL AND target_author <> NEW.author_id THEN
        INSERT INTO notifications(user_id,kind,actor_id,entity_type,entity_id,title,body,dedupe_key)
        VALUES(
            target_author,
            'post_reply',
            NEW.author_id,
            'post',
            NEW.post_id::text,
            'Новый ответ на публикацию',
            LEFT(NEW.body, 180),
            'reply:' || NEW.id::text
        )
        ON CONFLICT (user_id,dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER post_replies_notify_after_insert
AFTER INSERT ON post_replies
FOR EACH ROW EXECUTE FUNCTION notify_post_reply_insert();

CREATE OR REPLACE FUNCTION notify_channel_post_insert() RETURNS trigger AS $$
DECLARE
    author UUID;
    channel_title TEXT;
BEGIN
    SELECT p.author_id,c.title INTO author,channel_title
    FROM posts p
    JOIN channels c ON c.id=NEW.channel_id
    WHERE p.id=NEW.post_id;

    INSERT INTO notifications(user_id,kind,actor_id,entity_type,entity_id,title,body,dedupe_key)
    SELECT cs.user_id,
           'channel_post',
           author,
           'channel',
           NEW.channel_id::text,
           channel_title,
           'Новая публикация',
           'channel-post:' || NEW.post_id::text || ':' || cs.user_id::text
    FROM channel_subscribers cs
    WHERE cs.channel_id=NEW.channel_id
      AND cs.user_id IS DISTINCT FROM author
    ON CONFLICT (user_id,dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER channel_posts_notify_after_insert
AFTER INSERT ON channel_posts
FOR EACH ROW EXECUTE FUNCTION notify_channel_post_insert();
