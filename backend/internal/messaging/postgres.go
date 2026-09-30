package messaging

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) EnsureDirectChat(ctx context.Context, userID, peerUsername string) (ChatSummary, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChatSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var peerID, displayName, username string
	const peerQuery = `
		SELECT p.user_id::text, p.username, p.display_name
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		WHERE lower(p.username) = lower($1)
		  AND p.onboarding_completed_at IS NOT NULL
		  AND u.status = 'active'`
	if err := tx.QueryRow(ctx, peerQuery, peerUsername).Scan(&peerID, &username, &displayName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChatSummary{}, ErrProfileNotFound
		}
		return ChatSummary{}, err
	}
	if peerID == userID {
		return ChatSummary{}, ErrInteractionDenied
	}

	var blocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id=$1::uuid AND blocked_id=$2::uuid)
			   OR (blocker_id=$2::uuid AND blocked_id=$1::uuid)
		)`, userID, peerID).Scan(&blocked); err != nil {
		return ChatSummary{}, err
	}
	if blocked {
		return ChatSummary{}, ErrInteractionDenied
	}

	low, high := userID, peerID
	if strings.Compare(low, high) > 0 {
		low, high = high, low
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, low+":"+high); err != nil {
		return ChatSummary{}, err
	}

	var chatID, requestState string
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT c.id::text, c.updated_at, d.request_state
		FROM direct_chat_pairs d
		JOIN chats c ON c.id=d.chat_id
		WHERE d.user_low=$1::uuid AND d.user_high=$2::uuid`, low, high).Scan(&chatID, &updatedAt, &requestState)
	if errors.Is(err, pgx.ErrNoRows) {
		var recipientAlreadyFollows bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id=$1::uuid AND followed_id=$2::uuid)`, peerID, userID).Scan(&recipientAlreadyFollows); err != nil {
			return ChatSummary{}, err
		}
		requestState = "pending"
		if recipientAlreadyFollows {
			requestState = "accepted"
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO chats (kind, created_by)
			VALUES ('direct', $1::uuid)
			RETURNING id::text, updated_at`, userID).Scan(&chatID, &updatedAt); err != nil {
			return ChatSummary{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO direct_chat_pairs (chat_id,user_low,user_high,request_state,requested_by,accepted_at)
			VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,CASE WHEN $4='accepted' THEN now() ELSE NULL END)`, chatID, low, high, requestState, userID); err != nil {
			return ChatSummary{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO chat_members (chat_id,user_id)
			VALUES ($1::uuid,$2::uuid),($1::uuid,$3::uuid)`, chatID, userID, peerID); err != nil {
			return ChatSummary{}, err
		}
		if requestState == "pending" {
			if _, err := tx.Exec(ctx, `INSERT INTO message_request_events(chat_id,actor_id,action) VALUES($1::uuid,$2::uuid,'created')`, chatID, userID); err != nil {
				return ChatSummary{}, err
			}
		}
	} else if err != nil {
		return ChatSummary{}, err
	} else if requestState == "rejected" {
		return ChatSummary{}, ErrInteractionDenied
	}

	if err := tx.Commit(ctx); err != nil {
		return ChatSummary{}, err
	}
	return ChatSummary{ChatID: chatID, Kind: "direct", PeerUsername: username, PeerDisplayName: displayName, UpdatedAt: updatedAt}, nil
}

func (s *PostgresStore) ListChats(ctx context.Context, userID string, limit int, beforeUpdatedAt string) ([]ChatSummary, error) {
	const query = `
		SELECT c.id::text, c.kind, c.title, c.updated_at, cm.last_read_sequence,
		       COALESCE(peer.username,''), COALESCE(peer.display_name,''),
		       COALESCE((
				SELECT count(*) FROM messages um
				WHERE um.chat_id=c.id AND um.sequence>cm.last_read_sequence
				  AND um.sender_id IS DISTINCT FROM $1::uuid AND um.deleted_at IS NULL
			),0),
		       lm.id::text, lm.chat_id::text, COALESCE(lm.sender_id::text,''), lm.client_message_id::text,
		       lm.sequence, lm.type, lm.body, lm.reply_to_id::text, lm.created_at, lm.edited_at, lm.deleted_at
		FROM chat_members cm
		JOIN chats c ON c.id=cm.chat_id
		LEFT JOIN direct_chat_pairs d ON d.chat_id=c.id
		LEFT JOIN profiles peer ON peer.user_id = CASE WHEN d.user_low=$1::uuid THEN d.user_high ELSE d.user_low END
		LEFT JOIN LATERAL (SELECT m.* FROM messages m WHERE m.chat_id=c.id ORDER BY m.sequence DESC LIMIT 1) lm ON TRUE
		WHERE cm.user_id=$1::uuid AND cm.left_at IS NULL
		  AND (c.kind<>'direct' OR d.request_state='accepted' OR d.requested_by=$1::uuid)
		  AND (NULLIF($2,'') IS NULL OR c.updated_at < NULLIF($2,'')::timestamptz)
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT $3`
	rows, err := s.pool.Query(ctx, query, userID, beforeUpdatedAt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ChatSummary, 0, limit)
	for rows.Next() {
		var item ChatSummary
		var lastID, lastChatID, lastSenderID, lastClientID *string
		var lastSequence *int64
		var lastType, lastBody, replyTo *string
		var createdAt *time.Time
		var editedAt, deletedAt *time.Time
		if err := rows.Scan(&item.ChatID, &item.Kind, &item.Title, &item.UpdatedAt, &item.LastReadSequence,
			&item.PeerUsername, &item.PeerDisplayName, &item.UnreadCount,
			&lastID, &lastChatID, &lastSenderID, &lastClientID, &lastSequence, &lastType,
			&lastBody, &replyTo, &createdAt, &editedAt, &deletedAt); err != nil {
			return nil, err
		}
		if lastID != nil && lastSequence != nil && createdAt != nil {
			item.LastMessage = &Message{ID: *lastID, ChatID: value(lastChatID), SenderID: value(lastSenderID), ClientMessageID: value(lastClientID), Sequence: *lastSequence, Type: value(lastType), Body: value(lastBody), ReplyToID: replyTo, CreatedAt: *createdAt, EditedAt: editedAt, DeletedAt: deletedAt}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListMessages(ctx context.Context, userID, chatID string, beforeSequence int64, limit int) ([]Message, error) {
	if ok, err := s.isActiveMember(ctx, userID, chatID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrChatNotFound
	}
	const query = `
		SELECT id::text, chat_id::text, COALESCE(sender_id::text,''), client_message_id::text,
		       sequence, type, body, reply_to_id::text, created_at, edited_at, deleted_at
		FROM messages
		WHERE chat_id=$1::uuid AND ($2=0 OR sequence<$2)
		ORDER BY sequence DESC LIMIT $3`
	rows, err := s.pool.Query(ctx, query, chatID, beforeSequence, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0, limit)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, message)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SendText(ctx context.Context, input SendInput) (Message, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Message{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if existing, found, err := findMessageByClientID(ctx, tx, input.UserID, input.ClientMessageID); err != nil {
		return Message{}, false, err
	} else if found {
		if existing.ChatID != input.ChatID {
			return Message{}, false, ErrInvalidMessage
		}
		return existing, true, nil
	}

	var kind string
	const memberLock = `SELECT c.kind FROM chats c JOIN chat_members cm ON cm.chat_id=c.id WHERE c.id=$1::uuid AND cm.user_id=$2::uuid AND cm.left_at IS NULL FOR UPDATE OF c`
	if err := tx.QueryRow(ctx, memberLock, input.ChatID, input.UserID).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Message{}, false, ErrChatNotFound
		}
		return Message{}, false, err
	}
	if kind == "direct" {
		var blocked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM direct_chat_pairs d JOIN user_blocks b ON (b.blocker_id=d.user_low AND b.blocked_id=d.user_high) OR (b.blocker_id=d.user_high AND b.blocked_id=d.user_low) WHERE d.chat_id=$1::uuid)`, input.ChatID).Scan(&blocked); err != nil {
			return Message{}, false, err
		}
		if blocked {
			return Message{}, false, ErrInteractionDenied
		}
		var state string
		var requestedBy *string
		if err := tx.QueryRow(ctx, `SELECT request_state,requested_by::text FROM direct_chat_pairs WHERE chat_id=$1::uuid FOR UPDATE`, input.ChatID).Scan(&state, &requestedBy); err != nil {
			return Message{}, false, err
		}
		if state == "rejected" {
			return Message{}, false, ErrInteractionDenied
		}
		if state == "pending" {
			if requestedBy == nil || *requestedBy != input.UserID {
				return Message{}, false, ErrInteractionDenied
			}
			var sent int
			if err := tx.QueryRow(ctx, `SELECT count(*)::int FROM messages WHERE chat_id=$1::uuid AND sender_id=$2::uuid AND deleted_at IS NULL`, input.ChatID, input.UserID).Scan(&sent); err != nil {
				return Message{}, false, err
			}
			if sent >= 1 {
				return Message{}, false, ErrInteractionDenied
			}
		}
	}

	if input.ReplyToID != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1::uuid AND chat_id=$2::uuid AND deleted_at IS NULL)`, input.ReplyToID, input.ChatID).Scan(&exists); err != nil {
			return Message{}, false, err
		}
		if !exists {
			return Message{}, false, ErrInvalidMessage
		}
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE chats SET next_sequence=next_sequence+1, updated_at=now() WHERE id=$1::uuid RETURNING next_sequence-1`, input.ChatID).Scan(&sequence); err != nil {
		return Message{}, false, err
	}
	const insert = `INSERT INTO messages (chat_id,sender_id,client_message_id,sequence,type,body,reply_to_id) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'text',$5,NULLIF($6,'')::uuid) RETURNING id::text,chat_id::text,COALESCE(sender_id::text,''),client_message_id::text,sequence,type,body,reply_to_id::text,created_at,edited_at,deleted_at`
	message, err := scanMessage(tx.QueryRow(ctx, insert, input.ChatID, input.UserID, input.ClientMessageID, sequence, input.Body, input.ReplyToID))
	if err != nil {
		return Message{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Message{}, false, err
	}
	return message, false, nil
}

func (s *PostgresStore) EditMessage(ctx context.Context, userID, messageID, body string) (Message, error) {
	const query = `UPDATE messages m SET body=$3,edited_at=now() FROM chat_members cm WHERE m.id=$1::uuid AND m.sender_id=$2::uuid AND m.deleted_at IS NULL AND cm.chat_id=m.chat_id AND cm.user_id=$2::uuid AND cm.left_at IS NULL RETURNING m.id::text,m.chat_id::text,COALESCE(m.sender_id::text,''),m.client_message_id::text,m.sequence,m.type,m.body,m.reply_to_id::text,m.created_at,m.edited_at,m.deleted_at`
	message, err := scanMessage(s.pool.QueryRow(ctx, query, messageID, userID, body))
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrMessageNotFound
	}
	return message, err
}

func (s *PostgresStore) DeleteMessage(ctx context.Context, userID, messageID string) error {
	const query = `UPDATE messages m SET body='',deleted_at=now() FROM chat_members cm WHERE m.id=$1::uuid AND m.sender_id=$2::uuid AND m.deleted_at IS NULL AND cm.chat_id=m.chat_id AND cm.user_id=$2::uuid AND cm.left_at IS NULL`
	result, err := s.pool.Exec(ctx, query, messageID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrMessageNotFound
	}
	return nil
}

func (s *PostgresStore) SetReaction(ctx context.Context, userID, messageID, reaction string, enabled bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var member bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN chat_members cm ON cm.chat_id=m.chat_id WHERE m.id=$1::uuid AND m.deleted_at IS NULL AND cm.user_id=$2::uuid AND cm.left_at IS NULL)`, messageID, userID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return ErrMessageNotFound
	}
	if enabled {
		_, err = tx.Exec(ctx, `INSERT INTO message_reactions(message_id,user_id,reaction) VALUES($1::uuid,$2::uuid,$3) ON CONFLICT DO NOTHING`, messageID, userID, reaction)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM message_reactions WHERE message_id=$1::uuid AND user_id=$2::uuid AND reaction=$3`, messageID, userID, reaction)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListReactions(ctx context.Context, userID, messageID string) ([]ReactionSummary, error) {
	var member bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN chat_members cm ON cm.chat_id=m.chat_id WHERE m.id=$1::uuid AND m.deleted_at IS NULL AND cm.user_id=$2::uuid AND cm.left_at IS NULL)`, messageID, userID).Scan(&member); err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrMessageNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT reaction,count(*)::bigint,bool_or(user_id=$2::uuid) FROM message_reactions WHERE message_id=$1::uuid GROUP BY reaction ORDER BY count(*) DESC,reaction ASC`, messageID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ReactionSummary, 0, 8)
	for rows.Next() {
		var item ReactionSummary
		if err := rows.Scan(&item.Reaction, &item.Count, &item.Mine); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) MarkRead(ctx context.Context, userID, chatID string, sequence int64) error {
	const query = `UPDATE chat_members cm SET last_read_sequence=GREATEST(cm.last_read_sequence,LEAST($3,c.next_sequence-1)) FROM chats c WHERE cm.chat_id=c.id AND cm.chat_id=$1::uuid AND cm.user_id=$2::uuid AND cm.left_at IS NULL`
	result, err := s.pool.Exec(ctx, query, chatID, userID, sequence)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrChatNotFound
	}
	return nil
}

func (s *PostgresStore) isActiveMember(ctx context.Context, userID, chatID string) (bool, error) {
	var member bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id=$1::uuid AND user_id=$2::uuid AND left_at IS NULL)`, chatID, userID).Scan(&member); err != nil {
		return false, err
	}
	return member, nil
}

func findMessageByClientID(ctx context.Context, tx pgx.Tx, userID, clientMessageID string) (Message, bool, error) {
	const query = `SELECT id::text,chat_id::text,COALESCE(sender_id::text,''),client_message_id::text,sequence,type,body,reply_to_id::text,created_at,edited_at,deleted_at FROM messages WHERE sender_id=$1::uuid AND client_message_id=$2::uuid`
	message, err := scanMessage(tx.QueryRow(ctx, query, userID, clientMessageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, nil
	}
	if err != nil {
		return Message{}, false, err
	}
	return message, true, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanMessage(row rowScanner) (Message, error) {
	var message Message
	err := row.Scan(&message.ID, &message.ChatID, &message.SenderID, &message.ClientMessageID, &message.Sequence, &message.Type, &message.Body, &message.ReplyToID, &message.CreatedAt, &message.EditedAt, &message.DeletedAt)
	return message, err
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
