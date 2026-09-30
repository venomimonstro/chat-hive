package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetPublic(ctx context.Context, postID string) (Post, error) {
	const query = `
		SELECT p.id::text,p.author_id::text,pr.username,pr.display_name,
		       p.kind,p.body,p.visibility,p.replies_count,p.reactions_count,
		       p.created_at,p.updated_at,false,false
		FROM posts p
		JOIN profiles pr ON pr.user_id=p.author_id
		JOIN users u ON u.id=p.author_id AND u.status='active'
		WHERE p.id=$1::uuid AND p.deleted_at IS NULL AND p.visibility='public'`
	post, err := scanPost(s.pool.QueryRow(ctx, query, postID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, err
	}
	if err := s.loadMedia(ctx, &post); err != nil {
		return Post{}, err
	}
	return post, nil
}

func (s *PostgresStore) ListPublicByAuthor(ctx context.Context, username string, before time.Time, limit int) ([]Post, error) {
	var beforeValue any
	if !before.IsZero() {
		beforeValue = before
	}
	const query = `
		SELECT p.id::text,p.author_id::text,pr.username,pr.display_name,
		       p.kind,p.body,p.visibility,p.replies_count,p.reactions_count,
		       p.created_at,p.updated_at,false,false
		FROM posts p
		JOIN profiles pr ON pr.user_id=p.author_id
		JOIN users u ON u.id=p.author_id AND u.status='active'
		WHERE lower(pr.username)=lower($1)
		  AND p.deleted_at IS NULL
		  AND p.visibility='public'
		  AND ($2::timestamptz IS NULL OR p.created_at<$2)
		ORDER BY p.created_at DESC,p.id DESC
		LIMIT $3`
	rows, err := s.pool.Query(ctx, query, username, beforeValue, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Post, 0, limit)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.loadMedia(ctx, &items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *PostgresStore) ListPublicReplies(ctx context.Context, postID string, limit int) ([]Reply, error) {
	if _, err := s.GetPublic(ctx, postID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text,r.post_id::text,r.author_id::text,p.username,p.display_name,r.body,r.created_at,false
		FROM post_replies r
		JOIN profiles p ON p.user_id=r.author_id
		JOIN users u ON u.id=r.author_id AND u.status='active'
		WHERE r.post_id=$1::uuid AND r.deleted_at IS NULL
		ORDER BY r.created_at ASC,r.id ASC
		LIMIT $2`, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Reply, 0, limit)
	for rows.Next() {
		var reply Reply
		if err := rows.Scan(
			&reply.ID,&reply.PostID,&reply.AuthorID,&reply.AuthorUsername,
			&reply.AuthorName,&reply.Body,&reply.CreatedAt,&reply.Mine,
		); err != nil {
			return nil, err
		}
		items = append(items, reply)
	}
	return items, rows.Err()
}
