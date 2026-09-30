package admin

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Principal(ctx context.Context, userID string) (Principal, error) {
	var active bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_users WHERE user_id=$1::uuid AND status='active')`, userID).Scan(&active); err != nil { return Principal{}, err }
	if !active { return Principal{}, ErrForbidden }
	rows, err := s.pool.Query(ctx, `SELECT role FROM admin_user_roles WHERE user_id=$1::uuid ORDER BY role`, userID)
	if err != nil { return Principal{}, err }
	defer rows.Close()
	principal := Principal{UserID: userID}
	for rows.Next() { var role string; if err := rows.Scan(&role); err != nil { return Principal{}, err }; principal.Roles = append(principal.Roles, role) }
	if err := rows.Err(); err != nil { return Principal{}, err }
	if len(principal.Roles) == 0 { return Principal{}, ErrForbidden }
	return principal, nil
}

func (s *PostgresStore) ListCases(ctx context.Context, statuses []string, limit int) ([]Case, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text,c.target_type,c.target_id::text,c.severity,c.status,c.reason,
		       count(cr.report_id)::bigint,c.created_at,c.updated_at
		FROM moderation_cases c
		LEFT JOIN moderation_case_reports cr ON cr.case_id=c.id
		WHERE c.status = ANY($1)
		GROUP BY c.id
		ORDER BY CASE c.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,c.created_at ASC
		LIMIT $2`, statuses, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]Case, 0, limit)
	for rows.Next() {
		var item Case
		if err := rows.Scan(&item.ID,&item.TargetType,&item.TargetID,&item.Severity,&item.Status,&item.Reason,&item.ReportCount,&item.CreatedAt,&item.UpdatedAt); err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ResolveCase(ctx context.Context, actor Principal, caseID, decision, reason string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var targetType, targetID, status, caseReason string
	if err := tx.QueryRow(ctx, `SELECT target_type,target_id::text,status,reason FROM moderation_cases WHERE id=$1::uuid FOR UPDATE`, caseID).Scan(&targetType,&targetID,&status,&caseReason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
		return err
	}
	if status != "open" && status != "reviewing" { return ErrInvalid }
	nextStatus, action := "resolved", "case_resolved"
	if decision == "dismiss" { nextStatus, action = "dismissed", "case_dismissed" }

	// Review cases are approval workflows: dismissing the violation means public listing is approved;
	// confirming the case means the requested public listing is rejected.
	switch caseReason {
	case "public_community_review":
		entityStatus := "rejected"
		if decision == "dismiss" { entityStatus = "approved" }
		result, err := tx.Exec(ctx, `UPDATE communities SET moderation_status=$2,updated_at=now() WHERE chat_id=$1::uuid`, targetID, entityStatus)
		if err != nil { return err }
		if result.RowsAffected() != 1 { return ErrNotFound }
	case "public_channel_review":
		entityStatus := "rejected"
		if decision == "dismiss" { entityStatus = "approved" }
		result, err := tx.Exec(ctx, `UPDATE channels SET moderation_status=$2,updated_at=now() WHERE id=$1::uuid`, targetID, entityStatus)
		if err != nil { return err }
		if result.RowsAffected() != 1 { return ErrNotFound }
	}

	if _, err := tx.Exec(ctx, `UPDATE moderation_cases SET status=$2,reason=$3,updated_at=now(),resolved_at=now() WHERE id=$1::uuid`, caseID, nextStatus, reason); err != nil { return err }
	if _, err := tx.Exec(ctx, `UPDATE reports SET status=CASE WHEN $2='dismissed' THEN 'dismissed' ELSE 'resolved' END,updated_at=now() WHERE id IN (SELECT report_id FROM moderation_case_reports WHERE case_id=$1::uuid)`, caseID, nextStatus); err != nil { return err }
	if _, err := tx.Exec(ctx, `INSERT INTO moderation_actions(case_id,actor_id,action,target_type,target_id,reason) VALUES($1::uuid,$2::uuid,$3,$4,$5::uuid,$6)`, caseID, actor.UserID, action, targetType, targetID, reason); err != nil { return err }
	if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_events(actor_user_id,actor_role,action,target_type,target_id,reason) VALUES($1::uuid,$2,$3,$4,$5,$6)`, actor.UserID, strongestRole(actor), action, targetType, targetID, reason); err != nil { return err }
	return tx.Commit(ctx)
}

func (s *PostgresStore) WriteAudit(ctx context.Context, input AuditInput) error {
	var ip any
	if parsed := net.ParseIP(strings.TrimSpace(input.SourceIP)); parsed != nil { ip = parsed.String() }
	_, err := s.pool.Exec(ctx, `INSERT INTO admin_audit_events(actor_user_id,actor_role,action,target_type,target_id,reason,request_id,source_ip) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,$5,$6,$7,$8::inet)`, input.ActorUserID,input.ActorRole,input.Action,input.TargetType,input.TargetID,input.Reason,input.RequestID,ip)
	return err
}
