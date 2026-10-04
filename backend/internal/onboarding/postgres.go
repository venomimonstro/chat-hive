package onboarding

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) ListInterests(ctx context.Context) ([]Interest, error) {
	rows, err := s.pool.Query(ctx, `SELECT slug, label_ru, label_en FROM interests WHERE is_active = TRUE ORDER BY sort_order, slug`)
	if err != nil { return nil, err }
	defer rows.Close()

	items := make([]Interest, 0, 16)
	for rows.Next() {
		var item Interest
		if err := rows.Scan(&item.Slug, &item.LabelRU, &item.LabelEN); err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) GetProfile(ctx context.Context, userID string) (Profile, error) {
	var p Profile
	var completedAt *time.Time
	const query = `
		SELECT p.user_id::text, p.username, p.display_name, p.bio, p.onboarding_completed_at
		FROM profiles p WHERE p.user_id = $1::uuid`
	if err := s.pool.QueryRow(ctx, query, userID).Scan(&p.UserID, &p.Username, &p.DisplayName, &p.Bio, &completedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return Profile{UserID: userID}, nil }
		return Profile{}, err
	}
	p.Completed = completedAt != nil
	rows, err := s.pool.Query(ctx, `SELECT interest_slug FROM user_interests WHERE user_id = $1::uuid ORDER BY interest_slug`, userID)
	if err != nil { return Profile{}, err }
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil { return Profile{}, err }
		p.Interests = append(p.Interests, slug)
	}
	return p, rows.Err()
}

func (s *PostgresStore) Complete(ctx context.Context, input CompleteInput) (Profile, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Profile{}, err }
	defer func() { _ = tx.Rollback(ctx) }()

	var validCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM interests WHERE is_active = TRUE AND slug = ANY($1)`, input.Interests).Scan(&validCount); err != nil { return Profile{}, err }
	if validCount != len(input.Interests) { return Profile{}, ErrInvalidProfile }

	_, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id, username, display_name, bio, onboarding_completed_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, now(), now())
		ON CONFLICT (user_id) DO UPDATE
		SET username = EXCLUDED.username, display_name = EXCLUDED.display_name, bio = EXCLUDED.bio,
		    onboarding_completed_at = COALESCE(profiles.onboarding_completed_at, now()), updated_at = now()`,
		input.UserID, input.Username, input.DisplayName, input.Bio)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { return Profile{}, ErrUsernameTaken }
		return Profile{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO growth_events(event_name,user_id)
		VALUES('signup_completed',$1::uuid)
		ON CONFLICT DO NOTHING`, input.UserID); err != nil { return Profile{}, err }

	if _, err := tx.Exec(ctx, `DELETE FROM user_interests WHERE user_id = $1::uuid`, input.UserID); err != nil { return Profile{}, err }
	for _, slug := range input.Interests {
		if _, err := tx.Exec(ctx, `INSERT INTO user_interests (user_id, interest_slug) VALUES ($1::uuid, $2)`, input.UserID, slug); err != nil { return Profile{}, err }
	}
	if err := tx.Commit(ctx); err != nil { return Profile{}, err }
	return Profile{UserID: input.UserID, Username: input.Username, DisplayName: input.DisplayName, Bio: input.Bio, Interests: input.Interests, Completed: true}, nil
}
