package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) CreateOAuthState(ctx context.Context, provider string, stateHash []byte, verifier, requestIP string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_login_states (provider, state_hash, code_verifier, request_ip, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::inet, $5)`, provider, stateHash, verifier, requestIP, expiresAt)
	return err
}

func (s *PostgresStore) ConsumeOAuthState(ctx context.Context, provider string, stateHash []byte, now time.Time) (string, error) {
	var verifier string
	const query = `
		UPDATE oauth_login_states
		SET consumed_at = $4
		WHERE provider = $1 AND state_hash = $2 AND consumed_at IS NULL AND expires_at > $3
		RETURNING code_verifier`
	if err := s.pool.QueryRow(ctx, query, provider, stateHash, now, now).Scan(&verifier); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return "", ErrOAuthState }
		return "", err
	}
	return verifier, nil
}

func (s *PostgresStore) ResolveYandexIdentity(ctx context.Context, subject, email string, now time.Time) (string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return "", err }
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM user_identities WHERE provider = 'yandex' AND provider_subject = $1`, subject).Scan(&userID)
	if err == nil {
		if err := tx.Commit(ctx); err != nil { return "", err }
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) { return "", err }

	if err := tx.QueryRow(ctx, `INSERT INTO users DEFAULT VALUES RETURNING id::text`).Scan(&userID); err != nil {
		return "", fmt.Errorf("create yandex user: %w", err)
	}
	const insertIdentity = `
		INSERT INTO user_identities (user_id, provider, provider_subject, email_normalized, verified_at)
		VALUES ($1::uuid, 'yandex', $2, NULLIF($3, ''), $4)`
	if _, err := tx.Exec(ctx, insertIdentity, userID, subject, email, now); err != nil {
		return "", fmt.Errorf("create yandex identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil { return "", err }
	return userID, nil
}
