package securityevents

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) RateLimitExceeded(ctx context.Context, sourceIP, bucket, method, path string, retryAfter int) error {
	severity := "low"
	switch bucket {
	case "auth", "upload", "realtime-ticket":
		severity = "medium"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO security_events(event_type,severity,source_ip,subject_type,subject_id,metadata)
		VALUES(
			'rate_limit_exceeded',
			$1,
			CASE WHEN NULLIF($2,'') IS NULL THEN NULL ELSE $2::inet END,
			'rate_limit',
			$3,
			jsonb_build_object('method',$4,'path',$5,'retry_after_seconds',$6)
		)`, severity, sourceIP, bucket, method, path, retryAfter)
	return err
}
