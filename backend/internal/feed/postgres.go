package feed

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) List(ctx context.Context, userID, mode string, before time.Time, limit int) ([]Item, error) {
	var beforeValue any
	if !before.IsZero() {
		beforeValue = before
	}
	const query = `
		WITH viewer_interests AS (
			SELECT interest_slug FROM user_interests WHERE user_id=$1::uuid
		), candidates AS (
			SELECT p.id, p.author_id, p.kind, p.body, p.replies_count, p.reactions_count, p.created_at,
			       pr.username, pr.display_name,
			       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=$1::uuid AND f.followed_id=p.author_id) AS is_following,
			       (SELECT count(*) FROM user_interests ai JOIN viewer_interests vi ON vi.interest_slug=ai.interest_slug WHERE ai.user_id=p.author_id) AS shared_interests
			FROM posts p
			JOIN profiles pr ON pr.user_id=p.author_id
			JOIN users u ON u.id=p.author_id AND u.status='active'
			WHERE p.deleted_at IS NULL
			  AND p.visibility='public'
			  AND p.author_id<>$1::uuid
			  AND ($3::timestamptz IS NULL OR p.created_at<$3)
			  AND NOT EXISTS(
				SELECT 1 FROM user_blocks b
				WHERE (b.blocker_id=$1::uuid AND b.blocked_id=p.author_id)
				   OR (b.blocker_id=p.author_id AND b.blocked_id=$1::uuid)
			  )
		)
		SELECT id::text,author_id::text,username,display_name,kind,body,replies_count,reactions_count,created_at,
		       CASE
			WHEN is_following THEN 'Вы подписаны на автора'
			WHEN shared_interests>0 THEN 'У вас общие интересы'
			ELSE 'Новое в CHAT'
		   END AS reason,
		       (CASE WHEN is_following THEN 100 ELSE 0 END) + LEAST(shared_interests,5)*12 + LEAST(replies_count,20)::int AS score
		FROM candidates
		WHERE ($2='for-you' OR ($2='following' AND is_following))
		ORDER BY
			CASE WHEN $2='following' THEN extract(epoch from created_at)::bigint ELSE ((CASE WHEN is_following THEN 100 ELSE 0 END) + LEAST(shared_interests,5)*12 + LEAST(replies_count,20)::int) END DESC,
			created_at DESC,id DESC
		LIMIT $4`
	rows, err := s.pool.Query(ctx, query, userID, mode, beforeValue, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0, limit)
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.ID, &item.AuthorID, &item.AuthorUsername, &item.AuthorName,
			&item.Kind, &item.Body, &item.RepliesCount, &item.ReactionsCount,
			&item.CreatedAt, &item.Reason, &item.Score,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
