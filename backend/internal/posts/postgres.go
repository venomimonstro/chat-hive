package posts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Post, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Post{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		INSERT INTO posts (author_id, kind, body, visibility)
		VALUES ($1::uuid,$2,$3,$4)
		RETURNING id::text, author_id::text, kind, body, visibility,
		          replies_count, reactions_count, created_at, updated_at`
	var post Post
	if err := tx.QueryRow(ctx, query, input.AuthorID, input.Kind, input.Body, input.Visibility).Scan(
		&post.ID, &post.AuthorID, &post.Kind, &post.Body, &post.Visibility,
		&post.RepliesCount, &post.ReactionsCount, &post.CreatedAt, &post.UpdatedAt,
	); err != nil {
		return Post{}, err
	}
	for position, mediaID := range input.MediaIDs {
		var ref MediaRef
		const mediaQuery = `
			SELECT id::text,mime_type,width,height
			FROM media_objects
			WHERE id=$1::uuid AND owner_id=$2::uuid AND state='ready' AND deleted_at IS NULL
			FOR UPDATE`
		if err := tx.QueryRow(ctx, mediaQuery, mediaID, input.AuthorID).Scan(&ref.ID, &ref.MimeType, &ref.Width, &ref.Height); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Post{}, ErrForbidden
			}
			return Post{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO post_media(post_id,media_id,position) VALUES($1::uuid,$2::uuid,$3)`, post.ID, mediaID, position); err != nil {
			return Post{}, err
		}
		ref.URL = "/api/v1/media/" + ref.ID + "/content"
		post.Media = append(post.Media, ref)
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}
	post.Mine = true
	return post, nil
}

func (s *PostgresStore) Get(ctx context.Context, viewerID, postID string) (Post, error) {
	const query = `
		SELECT p.id::text, p.author_id::text, pr.username, pr.display_name,
		       p.kind, p.body, p.visibility, p.replies_count, p.reactions_count,
		       p.created_at, p.updated_at,
		       p.author_id=$1::uuid,
		       EXISTS(SELECT 1 FROM saved_posts sp WHERE sp.post_id=p.id AND sp.user_id=$1::uuid)
		FROM posts p
		JOIN profiles pr ON pr.user_id=p.author_id
		WHERE p.id=$2::uuid AND p.deleted_at IS NULL
		  AND (
			p.visibility='public'
			OR p.author_id=$1::uuid
			OR (p.visibility='followers' AND EXISTS(
				SELECT 1 FROM follows f WHERE f.follower_id=$1::uuid AND f.followed_id=p.author_id
			))
		  )`
	post, err := scanPost(s.pool.QueryRow(ctx, query, viewerID, postID))
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

func (s *PostgresStore) ListByAuthor(ctx context.Context, viewerID, username string, before time.Time, limit int) ([]Post, error) {
	const query = `
		SELECT p.id::text, p.author_id::text, pr.username, pr.display_name,
		       p.kind, p.body, p.visibility, p.replies_count, p.reactions_count,
		       p.created_at, p.updated_at,
		       p.author_id=$1::uuid,
		       EXISTS(SELECT 1 FROM saved_posts sp WHERE sp.post_id=p.id AND sp.user_id=$1::uuid)
		FROM posts p
		JOIN profiles pr ON pr.user_id=p.author_id
		WHERE lower(pr.username)=lower($2)
		  AND p.deleted_at IS NULL
		  AND ($3::timestamptz IS NULL OR p.created_at<$3)
		  AND (
			p.visibility='public'
			OR p.author_id=$1::uuid
			OR (p.visibility='followers' AND EXISTS(
				SELECT 1 FROM follows f WHERE f.follower_id=$1::uuid AND f.followed_id=p.author_id
			))
		  )
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $4`
	var beforeValue any
	if !before.IsZero() {
		beforeValue = before
	}
	rows, err := s.pool.Query(ctx, query, viewerID, username, beforeValue, limit)
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

func (s *PostgresStore) Edit(ctx context.Context, authorID, postID, body string) (Post, error) {
	const query = `
		UPDATE posts p
		SET body=$3, updated_at=now()
		FROM profiles pr
		WHERE p.id=$1::uuid AND p.author_id=$2::uuid AND p.deleted_at IS NULL AND pr.user_id=p.author_id
		RETURNING p.id::text, p.author_id::text, pr.username, pr.display_name, p.kind, p.body, p.visibility,
		          p.replies_count, p.reactions_count, p.created_at, p.updated_at, true,
		          EXISTS(SELECT 1 FROM saved_posts sp WHERE sp.post_id=p.id AND sp.user_id=$2::uuid)`
	post, err := scanPost(s.pool.QueryRow(ctx, query, postID, authorID, body))
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

func (s *PostgresStore) Delete(ctx context.Context, authorID, postID string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE posts SET body='', deleted_at=now(), updated_at=now()
		WHERE id=$1::uuid AND author_id=$2::uuid AND deleted_at IS NULL`, postID, authorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) AddReply(ctx context.Context, authorID, postID, body string) (Reply, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Reply{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var visible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM posts p
			WHERE p.id=$1::uuid AND p.deleted_at IS NULL
			  AND (
				p.visibility='public'
				OR p.author_id=$2::uuid
				OR (p.visibility='followers' AND EXISTS(
					SELECT 1 FROM follows f WHERE f.follower_id=$2::uuid AND f.followed_id=p.author_id
				))
			  )
		)`, postID, authorID).Scan(&visible); err != nil {
		return Reply{}, err
	}
	if !visible {
		return Reply{}, ErrNotFound
	}

	const insert = `
		WITH inserted AS (
			INSERT INTO post_replies (post_id,author_id,body)
			VALUES ($1::uuid,$2::uuid,$3)
			RETURNING id,post_id,author_id,body,created_at
		)
		SELECT i.id::text,i.post_id::text,i.author_id::text,p.username,p.display_name,i.body,i.created_at
		FROM inserted i JOIN profiles p ON p.user_id=i.author_id`
	var reply Reply
	if err := tx.QueryRow(ctx, insert, postID, authorID, body).Scan(
		&reply.ID, &reply.PostID, &reply.AuthorID, &reply.AuthorUsername,
		&reply.AuthorName, &reply.Body, &reply.CreatedAt,
	); err != nil {
		return Reply{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE posts SET replies_count=replies_count+1 WHERE id=$1::uuid`, postID); err != nil {
		return Reply{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Reply{}, err
	}
	reply.Mine = true
	return reply, nil
}

func (s *PostgresStore) ListReplies(ctx context.Context, viewerID, postID string, limit int) ([]Reply, error) {
	if _, err := s.Get(ctx, viewerID, postID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text,r.post_id::text,r.author_id::text,p.username,p.display_name,r.body,r.created_at,
		       r.author_id=$2::uuid
		FROM post_replies r
		JOIN profiles p ON p.user_id=r.author_id
		WHERE r.post_id=$1::uuid AND r.deleted_at IS NULL
		ORDER BY r.created_at ASC,r.id ASC
		LIMIT $3`, postID, viewerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Reply, 0, limit)
	for rows.Next() {
		var reply Reply
		if err := rows.Scan(
			&reply.ID, &reply.PostID, &reply.AuthorID, &reply.AuthorUsername,
			&reply.AuthorName, &reply.Body, &reply.CreatedAt, &reply.Mine,
		); err != nil {
			return nil, err
		}
		items = append(items, reply)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SetReaction(ctx context.Context, userID, postID, reaction string, enabled bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1::uuid AND deleted_at IS NULL)`, postID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if enabled {
		result, err := tx.Exec(ctx, `
			INSERT INTO post_reactions(post_id,user_id,reaction)
			VALUES($1::uuid,$2::uuid,$3)
			ON CONFLICT DO NOTHING`, postID, userID, reaction)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `UPDATE posts SET reactions_count=reactions_count+1 WHERE id=$1::uuid`, postID); err != nil {
				return err
			}
		}
	} else {
		result, err := tx.Exec(ctx, `DELETE FROM post_reactions WHERE post_id=$1::uuid AND user_id=$2::uuid AND reaction=$3`, postID, userID, reaction)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `UPDATE posts SET reactions_count=GREATEST(0,reactions_count-1) WHERE id=$1::uuid`, postID); err != nil {
				return err
			}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SetSaved(ctx context.Context, userID, postID string, saved bool) error {
	if saved {
		result, err := s.pool.Exec(ctx, `
			INSERT INTO saved_posts(post_id,user_id)
			SELECT id,$2::uuid FROM posts WHERE id=$1::uuid AND deleted_at IS NULL
			ON CONFLICT DO NOTHING`, postID, userID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			var exists bool
			if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1::uuid AND deleted_at IS NULL)`, postID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM saved_posts WHERE post_id=$1::uuid AND user_id=$2::uuid`, postID, userID)
	return err
}

func (s *PostgresStore) loadMedia(ctx context.Context, post *Post) error {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id::text,m.mime_type,m.width,m.height
		FROM post_media pm
		JOIN media_objects m ON m.id=pm.media_id AND m.state='ready' AND m.deleted_at IS NULL
		WHERE pm.post_id=$1::uuid
		ORDER BY pm.position ASC`, post.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	post.Media = nil
	for rows.Next() {
		var ref MediaRef
		if err := rows.Scan(&ref.ID, &ref.MimeType, &ref.Width, &ref.Height); err != nil {
			return err
		}
		ref.URL = "/api/v1/media/" + ref.ID + "/content"
		post.Media = append(post.Media, ref)
	}
	return rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanPost(row rowScanner) (Post, error) {
	var post Post
	err := row.Scan(
		&post.ID, &post.AuthorID, &post.AuthorUsername, &post.AuthorName,
		&post.Kind, &post.Body, &post.Visibility, &post.RepliesCount,
		&post.ReactionsCount, &post.CreatedAt, &post.UpdatedAt, &post.Mine, &post.Saved,
	)
	return post, err
}
