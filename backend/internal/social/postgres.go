package social

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) GetProfile(ctx context.Context, viewerID, username string) (PublicProfile, error) {
	const query = `
		SELECT p.user_id::text, p.username, p.display_name, p.bio,
		       (SELECT count(*) FROM follows f WHERE f.followed_id = p.user_id),
		       (SELECT count(*) FROM follows f WHERE f.follower_id = p.user_id),
		       CASE WHEN NULLIF($2, '') IS NULL THEN FALSE ELSE EXISTS(
		           SELECT 1 FROM follows f WHERE f.follower_id = NULLIF($2, '')::uuid AND f.followed_id = p.user_id
		       ) END,
		       CASE WHEN NULLIF($2, '') IS NULL THEN FALSE ELSE EXISTS(
		           SELECT 1 FROM user_blocks b WHERE b.blocker_id = NULLIF($2, '')::uuid AND b.blocked_id = p.user_id
		       ) END,
		       CASE WHEN NULLIF($2, '') IS NULL THEN FALSE ELSE p.user_id = NULLIF($2, '')::uuid END
		FROM profiles p
		JOIN users u ON u.id = p.user_id AND u.status = 'active'
		WHERE lower(p.username) = lower($1) AND p.onboarding_completed_at IS NOT NULL`
	var p PublicProfile
	if err := s.pool.QueryRow(ctx, query, username, viewerID).Scan(
		&p.UserID, &p.Username, &p.DisplayName, &p.Bio,
		&p.FollowersCount, &p.FollowingCount, &p.IsFollowing, &p.IsBlocked, &p.IsSelf,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return PublicProfile{}, ErrProfileNotFound }
		return PublicProfile{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT interest_slug FROM user_interests WHERE user_id = $1::uuid ORDER BY interest_slug`, p.UserID)
	if err != nil { return PublicProfile{}, err }
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil { return PublicProfile{}, err }
		p.Interests = append(p.Interests, slug)
	}
	return p, rows.Err()
}

func (s *PostgresStore) SetFollow(ctx context.Context, followerID, username string, follow bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func() { _ = tx.Rollback(ctx) }()

	var followedID string
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE lower(username) = lower($1) AND onboarding_completed_at IS NOT NULL`, username).Scan(&followedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrProfileNotFound }
		return err
	}
	if followerID == followedID { return ErrInteractionDenied }

	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id = $1::uuid AND blocked_id = $2::uuid) OR (blocker_id = $2::uuid AND blocked_id = $1::uuid))`, followerID, followedID).Scan(&blocked); err != nil { return err }
	if blocked { return ErrInteractionDenied }

	if follow {
		_, err = tx.Exec(ctx, `INSERT INTO follows (follower_id, followed_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`, followerID, followedID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1::uuid AND followed_id = $2::uuid`, followerID, followedID)
	}
	if err != nil { return err }
	return tx.Commit(ctx)
}

func (s *PostgresStore) SetBlock(ctx context.Context, blockerID, username string, block bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func() { _ = tx.Rollback(ctx) }()

	var blockedID string
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE lower(username) = lower($1) AND onboarding_completed_at IS NOT NULL`, username).Scan(&blockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrProfileNotFound }
		return err
	}
	if blockerID == blockedID { return ErrInteractionDenied }

	if block {
		if _, err := tx.Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`, blockerID, blockedID); err != nil { return err }
		if _, err := tx.Exec(ctx, `DELETE FROM follows WHERE (follower_id = $1::uuid AND followed_id = $2::uuid) OR (follower_id = $2::uuid AND followed_id = $1::uuid)`, blockerID, blockedID); err != nil { return err }
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM user_blocks WHERE blocker_id = $1::uuid AND blocked_id = $2::uuid`, blockerID, blockedID); err != nil { return err }
	}
	return tx.Commit(ctx)
}
