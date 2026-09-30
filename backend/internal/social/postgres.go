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
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicProfile{}, ErrProfileNotFound
		}
		return PublicProfile{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT interest_slug FROM user_interests WHERE user_id = $1::uuid ORDER BY interest_slug`, p.UserID)
	if err != nil {
		return PublicProfile{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return PublicProfile{}, err
		}
		p.Interests = append(p.Interests, slug)
	}
	return p, rows.Err()
}

func (s *PostgresStore) ListFollowers(ctx context.Context, viewerID, username string, limit int) ([]Connection, error) {
	return s.listConnections(ctx, viewerID, username, limit, true)
}

func (s *PostgresStore) ListFollowing(ctx context.Context, viewerID, username string, limit int) ([]Connection, error) {
	return s.listConnections(ctx, viewerID, username, limit, false)
}

func (s *PostgresStore) listConnections(ctx context.Context, viewerID, username string, limit int, followers bool) ([]Connection, error) {
	var ownerID string
	if err := s.pool.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE lower(username)=lower($1) AND onboarding_completed_at IS NOT NULL`, username).Scan(&ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}

	relation := "f.followed_id=$2::uuid AND p.user_id=f.follower_id"
	if !followers {
		relation = "f.follower_id=$2::uuid AND p.user_id=f.followed_id"
	}
	query := `
		WITH viewer_interests AS (
			SELECT interest_slug FROM user_interests WHERE user_id=NULLIF($1,'')::uuid
		)
		SELECT p.username,p.display_name,p.bio,
		       (SELECT count(*) FROM follows fx WHERE fx.followed_id=p.user_id),
		       (SELECT count(*) FROM user_interests ui JOIN viewer_interests vi ON vi.interest_slug=ui.interest_slug WHERE ui.user_id=p.user_id),
		       CASE WHEN NULLIF($1,'') IS NULL THEN FALSE ELSE EXISTS(
			SELECT 1 FROM follows mine WHERE mine.follower_id=NULLIF($1,'')::uuid AND mine.followed_id=p.user_id
		   ) END
		FROM follows f
		JOIN profiles p ON ` + relation + `
		JOIN users u ON u.id=p.user_id AND u.status='active'
		WHERE p.onboarding_completed_at IS NOT NULL
		  AND (NULLIF($1,'') IS NULL OR NOT EXISTS(
			SELECT 1 FROM user_blocks b
			WHERE (b.blocker_id=NULLIF($1,'')::uuid AND b.blocked_id=p.user_id)
			   OR (b.blocker_id=p.user_id AND b.blocked_id=NULLIF($1,'')::uuid)
		  ))
		ORDER BY 5 DESC,4 DESC,p.username ASC
		LIMIT $3`

	rows, err := s.pool.Query(ctx, query, viewerID, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Connection, 0, limit)
	for rows.Next() {
		var item Connection
		if err := rows.Scan(&item.Username, &item.DisplayName, &item.Bio, &item.FollowersCount, &item.SharedInterests, &item.IsFollowing); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SetFollow(ctx context.Context, followerID, username string, follow bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var followedID string
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE lower(username) = lower($1) AND onboarding_completed_at IS NOT NULL`, username).Scan(&followedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProfileNotFound
		}
		return err
	}
	if followerID == followedID {
		return ErrInteractionDenied
	}

	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id = $1::uuid AND blocked_id = $2::uuid) OR (blocker_id = $2::uuid AND blocked_id = $1::uuid))`, followerID, followedID).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return ErrInteractionDenied
	}

	if follow {
		_, err = tx.Exec(ctx, `INSERT INTO follows (follower_id, followed_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`, followerID, followedID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1::uuid AND followed_id = $2::uuid`, followerID, followedID)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SetBlock(ctx context.Context, blockerID, username string, block bool) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var blockedID string
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE lower(username) = lower($1) AND onboarding_completed_at IS NOT NULL`, username).Scan(&blockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProfileNotFound
		}
		return err
	}
	if blockerID == blockedID {
		return ErrInteractionDenied
	}

	if block {
		if _, err := tx.Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`, blockerID, blockedID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM follows WHERE (follower_id = $1::uuid AND followed_id = $2::uuid) OR (follower_id = $2::uuid AND followed_id = $1::uuid)`, blockerID, blockedID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM user_blocks WHERE blocker_id = $1::uuid AND blocked_id = $2::uuid`, blockerID, blockedID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
