package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CountRecentChallenges(ctx context.Context, email, requestIP string, since time.Time) (int, error) {
	const query = `
		SELECT count(*)
		FROM login_challenges
		WHERE created_at >= $3
		  AND (email_normalized = $1 OR (NULLIF($2, '') IS NOT NULL AND request_ip = NULLIF($2, '')::inet))`
	var count int
	if err := s.pool.QueryRow(ctx, query, email, requestIP, since).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostgresStore) CreateLoginChallenge(ctx context.Context, email string, tokenHash []byte, requestIP string, expiresAt time.Time) error {
	const query = `
		INSERT INTO login_challenges (email_normalized, token_hash, request_ip, expires_at)
		VALUES ($1, $2, NULLIF($3, '')::inet, $4)`
	_, err := s.pool.Exec(ctx, query, email, tokenHash, requestIP, expiresAt)
	return err
}

func (s *PostgresStore) ConsumeLoginChallenge(ctx context.Context, tokenHash []byte, now time.Time) (string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var email string
	var expiresAt time.Time
	var consumedAt *time.Time
	const selectChallenge = `
		SELECT email_normalized, expires_at, consumed_at
		FROM login_challenges
		WHERE token_hash = $1
		FOR UPDATE`
	if err := tx.QueryRow(ctx, selectChallenge, tokenHash).Scan(&email, &expiresAt, &consumedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidChallenge
		}
		return "", err
	}
	if consumedAt != nil || !now.Before(expiresAt) {
		return "", ErrInvalidChallenge
	}

	if _, err := tx.Exec(ctx, `UPDATE login_challenges SET consumed_at = $2 WHERE token_hash = $1`, tokenHash, now); err != nil {
		return "", err
	}

	var userID string
	const findUser = `
		SELECT user_id::text
		FROM user_identities
		WHERE provider = 'email' AND email_normalized = $1`
	err = tx.QueryRow(ctx, findUser, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `INSERT INTO users DEFAULT VALUES RETURNING id::text`).Scan(&userID); err != nil {
			return "", fmt.Errorf("create user: %w", err)
		}
		const createIdentity = `
			INSERT INTO user_identities (user_id, provider, provider_subject, email_normalized, verified_at)
			VALUES ($1::uuid, 'email', $2, $2, $3)`
		if _, err := tx.Exec(ctx, createIdentity, userID, email, now); err != nil {
			return "", fmt.Errorf("create email identity: %w", err)
		}
	} else if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, input CreateSessionInput) (string, error) {
	const query = `
		INSERT INTO sessions (
			user_id, refresh_token_hash, access_token_hash, user_agent, last_ip,
			access_expires_at, expires_at
		)
		VALUES ($1::uuid, $2, $3, $4, NULLIF($5, '')::inet, $6, $7)
		RETURNING id::text`
	var sessionID string
	if err := s.pool.QueryRow(ctx, query,
		input.UserID,
		input.RefreshTokenHash,
		input.AccessTokenHash,
		input.UserAgent,
		input.IP,
		input.AccessExpiresAt,
		input.RefreshExpiresAt,
	).Scan(&sessionID); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (s *PostgresStore) FindSessionByAccessTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (AuthenticatedSession, error) {
	const query = `
		SELECT user_id::text, id::text, access_expires_at
		FROM sessions
		WHERE access_token_hash = $1
		  AND revoked_at IS NULL
		  AND access_expires_at > $2
		  AND expires_at > $2`
	var session AuthenticatedSession
	if err := s.pool.QueryRow(ctx, query, tokenHash, now).Scan(&session.UserID, &session.SessionID, &session.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthenticatedSession{}, ErrInvalidSession
		}
		return AuthenticatedSession{}, err
	}
	return session, nil
}

func (s *PostgresStore) RevokeSession(ctx context.Context, userID, sessionID string, now time.Time) (bool, error) {
	const query = `
		UPDATE sessions
		SET revoked_at = $3, access_token_hash = NULL
		WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`
	result, err := s.pool.Exec(ctx, query, sessionID, userID, now)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
