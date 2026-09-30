package search

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Search(ctx context.Context, userID, query string, limit int) (Results, error) {
	pattern := "%" + query + "%"
	var results Results

	peopleRows, err := s.pool.Query(ctx, `
		WITH viewer_interests AS (
			SELECT interest_slug FROM user_interests WHERE user_id=$1::uuid
		)
		SELECT p.username,p.display_name,p.bio,
		       (SELECT count(*) FROM follows f WHERE f.followed_id=p.user_id),
		       (SELECT count(*) FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=p.user_id)
		FROM profiles p
		JOIN users u ON u.id=p.user_id AND u.status='active'
		WHERE p.onboarding_completed_at IS NOT NULL
		  AND p.user_id<>$1::uuid
		  AND (p.username ILIKE $2 OR p.display_name ILIKE $2 OR p.bio ILIKE $2)
		  AND NOT EXISTS(
			SELECT 1 FROM user_blocks b
			WHERE (b.blocker_id=$1::uuid AND b.blocked_id=p.user_id)
			   OR (b.blocker_id=p.user_id AND b.blocked_id=$1::uuid)
		  )
		ORDER BY
		  CASE WHEN lower(p.username)=lower($3) THEN 0 WHEN p.username ILIKE $3 || '%' THEN 1 ELSE 2 END,
		  5 DESC,p.username ASC
		LIMIT $4`, userID, pattern, query, limit)
	if err != nil {
		return Results{}, err
	}
	for peopleRows.Next() {
		var item Person
		if err := peopleRows.Scan(&item.Username, &item.DisplayName, &item.Bio, &item.FollowersCount, &item.SharedInterests); err != nil {
			peopleRows.Close()
			return Results{}, err
		}
		results.People = append(results.People, item)
	}
	if err := peopleRows.Err(); err != nil {
		peopleRows.Close()
		return Results{}, err
	}
	peopleRows.Close()

	postRows, err := s.pool.Query(ctx, `
		SELECT p.id::text,pr.username,pr.display_name,p.kind,p.body
		FROM posts p
		JOIN profiles pr ON pr.user_id=p.author_id
		JOIN users u ON u.id=p.author_id AND u.status='active'
		WHERE p.deleted_at IS NULL AND p.visibility='public' AND p.body ILIKE $2
		  AND NOT EXISTS(
			SELECT 1 FROM user_blocks b
			WHERE (b.blocker_id=$1::uuid AND b.blocked_id=p.author_id)
			   OR (b.blocker_id=p.author_id AND b.blocked_id=$1::uuid)
		  )
		ORDER BY p.created_at DESC,p.id DESC
		LIMIT $3`, userID, pattern, limit)
	if err != nil {
		return Results{}, err
	}
	for postRows.Next() {
		var item Post
		if err := postRows.Scan(&item.ID, &item.AuthorUsername, &item.AuthorName, &item.Kind, &item.Body); err != nil {
			postRows.Close()
			return Results{}, err
		}
		results.Posts = append(results.Posts, item)
	}
	if err := postRows.Err(); err != nil {
		postRows.Close()
		return Results{}, err
	}
	postRows.Close()

	groupRows, err := s.pool.Query(ctx, `
		SELECT c.id::text,c.title,c.description,
		       (SELECT count(*) FROM chat_members cm2 WHERE cm2.chat_id=c.id AND cm2.left_at IS NULL),
		       EXISTS(SELECT 1 FROM chat_members cm3 WHERE cm3.chat_id=c.id AND cm3.user_id=$1::uuid AND cm3.left_at IS NULL)
		FROM chats c
		WHERE c.kind='group' AND (c.title ILIKE $2 OR c.description ILIKE $2)
		ORDER BY c.updated_at DESC,c.id DESC
		LIMIT $3`, userID, pattern, limit)
	if err != nil {
		return Results{}, err
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var item Group
		if err := groupRows.Scan(&item.ChatID, &item.Title, &item.Description, &item.MembersCount, &item.IsMember); err != nil {
			return Results{}, err
		}
		results.Groups = append(results.Groups, item)
	}
	return results, groupRows.Err()
}
