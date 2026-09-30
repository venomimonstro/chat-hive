package moderation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) CreateReport(ctx context.Context, input CreateInput) (Report, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Report{}, err }
	defer func() { _ = tx.Rollback(ctx) }()

	allowed, err := reportableTarget(ctx, tx, input.ReporterID, input.TargetType, input.TargetID)
	if err != nil { return Report{}, err }
	if !allowed { return Report{}, ErrNotFound }

	var report Report
	const insert = `
		INSERT INTO reports(reporter_id,target_type,target_id,category,note)
		VALUES($1::uuid,$2,$3::uuid,$4,$5)
		RETURNING id::text,target_type,target_id::text,category,status,created_at`
	if err := tx.QueryRow(ctx, insert, input.ReporterID, input.TargetType, input.TargetID, input.Category, input.Note).Scan(
		&report.ID,&report.TargetType,&report.TargetID,&report.Category,&report.Status,&report.CreatedAt,
	); err != nil { return Report{}, err }

	severity := severityFor(input.Category)
	var caseID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM moderation_cases
		WHERE target_type=$1 AND target_id=$2::uuid AND status IN ('open','reviewing')
		ORDER BY created_at ASC LIMIT 1 FOR UPDATE`, input.TargetType, input.TargetID).Scan(&caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			INSERT INTO moderation_cases(target_type,target_id,severity,reason)
			VALUES($1,$2::uuid,$3,$4)
			RETURNING id::text`, input.TargetType, input.TargetID, severity, input.Category).Scan(&caseID); err != nil {
			return Report{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO moderation_actions(case_id,action,target_type,target_id,reason) VALUES($1::uuid,'case_created',$2,$3::uuid,$4)`, caseID, input.TargetType, input.TargetID, input.Category); err != nil {
			return Report{}, err
		}
	} else if err != nil {
		return Report{}, err
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE moderation_cases
			SET severity=CASE
				WHEN severity='critical' OR $2='low' THEN severity
				WHEN $2='critical' THEN 'critical'
				WHEN $2='high' AND severity IN ('low','medium') THEN 'high'
				WHEN $2='medium' AND severity='low' THEN 'medium'
				ELSE severity END,
				updated_at=now()
			WHERE id=$1::uuid`, caseID, severity); err != nil {
			return Report{}, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO moderation_case_reports(case_id,report_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, caseID, report.ID); err != nil {
		return Report{}, err
	}
	return report, tx.Commit(ctx)
}

func reportableTarget(ctx context.Context, tx pgx.Tx, reporterID, targetType, targetID string) (bool, error) {
	var allowed bool
	var query string
	switch targetType {
	case "user":
		query = `SELECT EXISTS(SELECT 1 FROM users WHERE id=$2::uuid AND id<>$1::uuid AND status='active')`
	case "message":
		query = `SELECT EXISTS(SELECT 1 FROM messages m JOIN chat_members cm ON cm.chat_id=m.chat_id WHERE m.id=$2::uuid AND cm.user_id=$1::uuid AND cm.left_at IS NULL)`
	case "post":
		query = `SELECT EXISTS(SELECT 1 FROM posts p WHERE p.id=$2::uuid AND p.deleted_at IS NULL AND (p.visibility='public' OR p.author_id=$1::uuid OR (p.visibility='followers' AND EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=$1::uuid AND f.followed_id=p.author_id))))`
	case "group":
		query = `SELECT EXISTS(SELECT 1 FROM chats c JOIN chat_members cm ON cm.chat_id=c.id WHERE c.id=$2::uuid AND c.kind='group' AND cm.user_id=$1::uuid AND cm.left_at IS NULL)`
	default:
		return false, ErrInvalidReport
	}
	if err := tx.QueryRow(ctx, query, reporterID, targetID).Scan(&allowed); err != nil { return false, err }
	return allowed, nil
}

func severityFor(category string) string {
	switch category {
	case "threats", "illegal_content", "sexual_content", "malware":
		return "high"
	case "scam", "impersonation", "harassment":
		return "medium"
	default:
		return "low"
	}
}
