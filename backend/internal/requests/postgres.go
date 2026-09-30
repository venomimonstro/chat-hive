package requests

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) List(ctx context.Context, userID string, limit int) ([]Item, error) {
	const query = `
		WITH viewer_interests AS (
			SELECT interest_slug FROM user_interests WHERE user_id=$1::uuid
		), requester AS (
			SELECT d.chat_id,d.requested_by
			FROM direct_chat_pairs d
			WHERE d.request_state='pending'
			  AND d.requested_by IS NOT NULL
			  AND d.requested_by<>$1::uuid
			  AND ($1::uuid=d.user_low OR $1::uuid=d.user_high)
		)
		SELECT r.chat_id::text,p.user_id::text,p.username,p.display_name,
		       COALESCE(m.body,''),COALESCE(m.created_at,c.created_at),
		       (SELECT count(*) FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=r.requested_by),
		       (
		         SELECT count(*) FROM follows a
		         JOIN follows b ON b.followed_id=a.followed_id
		         WHERE a.follower_id=$1::uuid AND b.follower_id=r.requested_by
		       )
		FROM requester r
		JOIN chats c ON c.id=r.chat_id
		JOIN profiles p ON p.user_id=r.requested_by
		LEFT JOIN LATERAL (
			SELECT body,created_at FROM messages WHERE chat_id=r.chat_id AND sender_id=r.requested_by AND deleted_at IS NULL ORDER BY sequence DESC LIMIT 1
		) m ON TRUE
		WHERE NOT EXISTS(
			SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id=$1::uuid AND ub.blocked_id=r.requested_by)
			   OR (ub.blocker_id=r.requested_by AND ub.blocked_id=$1::uuid)
		)
		ORDER BY COALESCE(m.created_at,c.created_at) DESC,r.chat_id DESC
		LIMIT $2`
	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]Item, 0, limit)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ChatID,&item.RequesterID,&item.Username,&item.DisplayName,&item.Body,&item.CreatedAt,&item.SharedInterests,&item.MutualFollowers); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) Decide(ctx context.Context, userID, chatID string, accept bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func() { _ = tx.Rollback(ctx) }()
	var requestedBy, state string
	const lock = `
		SELECT requested_by::text,request_state
		FROM direct_chat_pairs
		WHERE chat_id=$1::uuid
		  AND requested_by IS NOT NULL
		  AND requested_by<>$2::uuid
		  AND ($2::uuid=user_low OR $2::uuid=user_high)
		FOR UPDATE`
	if err := tx.QueryRow(ctx, lock, chatID, userID).Scan(&requestedBy, &state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
		return err
	}
	if state != "pending" { return ErrNotFound }
	nextState := "rejected"
	action := "rejected"
	if accept {
		nextState = "accepted"
		action = "accepted"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE direct_chat_pairs
		SET request_state=$3,accepted_at=CASE WHEN $3='accepted' THEN now() ELSE accepted_at END
		WHERE chat_id=$1::uuid AND requested_by=$2::uuid`, chatID, requestedBy, nextState); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO message_request_events(chat_id,actor_id,action) VALUES($1::uuid,$2::uuid,$3)`, chatID, userID, action); err != nil {
		return err
	}
	if !accept {
		if _, err := tx.Exec(ctx, `UPDATE chat_members SET last_read_sequence=(SELECT next_sequence-1 FROM chats WHERE id=$1::uuid) WHERE chat_id=$1::uuid AND user_id=$2::uuid`, chatID, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
