package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

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

func (s *PostgresStore) ListSecurityEvents(ctx context.Context, severity string, limit int) ([]SecurityEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,event_type,severity,
		       COALESCE(user_id::text,''),COALESCE(session_id::text,''),COALESCE(source_ip::text,''),
		       subject_type,subject_id,metadata,created_at
		FROM security_events
		WHERE ($1='' OR severity=$1)
		ORDER BY created_at DESC,id DESC
		LIMIT $2`, severity, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]SecurityEvent, 0, limit)
	for rows.Next() {
		var item SecurityEvent
		var userID, sessionID, sourceIP string
		var metadata []byte
		if err := rows.Scan(&item.ID,&item.EventType,&item.Severity,&userID,&sessionID,&sourceIP,&item.SubjectType,&item.SubjectID,&metadata,&item.CreatedAt); err != nil { return nil, err }
		if userID != "" { value := userID; item.UserID = &value }
		if sessionID != "" { value := sessionID; item.SessionID = &value }
		if sourceIP != "" { value := sourceIP; item.SourceIP = &value }
		item.Metadata = map[string]any{}
		if len(metadata) > 0 { if err := json.Unmarshal(metadata, &item.Metadata); err != nil { return nil, err } }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) ListSecurityAlerts(ctx context.Context, status string, limit int) ([]SecurityAlert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.event_id,e.event_type,e.severity,a.status,COALESCE(e.source_ip::text,''),
		       e.subject_type,e.subject_id,e.metadata,e.created_at,
		       a.acknowledged_at,COALESCE(a.acknowledged_by::text,''),a.note
		FROM security_alerts a
		JOIN security_events e ON e.id=a.event_id
		WHERE ($1='all' OR a.status=$1)
		ORDER BY CASE e.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 ELSE 2 END,
		         e.created_at DESC,e.id DESC
		LIMIT $2`, status, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]SecurityAlert, 0, limit)
	for rows.Next() {
		var item SecurityAlert
		var sourceIP, acknowledgedBy string
		var metadata []byte
		if err := rows.Scan(
			&item.EventID,&item.EventType,&item.Severity,&item.Status,&sourceIP,
			&item.SubjectType,&item.SubjectID,&metadata,&item.CreatedAt,
			&item.AcknowledgedAt,&acknowledgedBy,&item.Note,
		); err != nil { return nil, err }
		if sourceIP != "" { value := sourceIP; item.SourceIP = &value }
		if acknowledgedBy != "" { value := acknowledgedBy; item.AcknowledgedBy = &value }
		item.Metadata = map[string]any{}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &item.Metadata); err != nil { return nil, err }
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AcknowledgeSecurityAlert(ctx context.Context, eventID int64, actor Principal, note string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		UPDATE security_alerts
		SET status='acknowledged',acknowledged_by=$2::uuid,acknowledged_at=now(),note=$3,updated_at=now()
		WHERE event_id=$1 AND status='open'`, eventID, actor.UserID, note)
	if err != nil { return err }
	if result.RowsAffected() != 1 { return ErrNotFound }

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_events(actor_user_id,actor_role,action,target_type,target_id,reason)
		VALUES($1::uuid,$2,'security_alert_acknowledged','security_event',$3,$4)`,
		actor.UserID,strongestRole(actor),eventID,note); err != nil { return err }

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListPlatformFlags(ctx context.Context) ([]PlatformFlag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT key,enabled,reason,COALESCE(updated_by::text,''),updated_at
		FROM platform_feature_flags
		ORDER BY key`)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]PlatformFlag, 0, 8)
	for rows.Next() {
		var item PlatformFlag
		var updatedBy string
		if err := rows.Scan(&item.Key,&item.Enabled,&item.Reason,&updatedBy,&item.UpdatedAt); err != nil { return nil, err }
		if updatedBy != "" { value := updatedBy; item.UpdatedBy = &value }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SetPlatformFlag(ctx context.Context, key string, enabled bool, actor Principal, reason string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()

	var previous bool
	if err := tx.QueryRow(ctx, `
		SELECT enabled FROM platform_feature_flags WHERE key=$1 FOR UPDATE`, key).Scan(&previous); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE platform_feature_flags
		SET enabled=$2,reason=$3,updated_by=$4::uuid,updated_at=now()
		WHERE key=$1`, key, enabled, reason, actor.UserID); err != nil { return err }

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_events(actor_user_id,actor_role,action,target_type,target_id,reason)
		VALUES($1::uuid,$2,'platform_feature_flag_changed','feature_flag',$3,$4)`,
		actor.UserID,strongestRole(actor),key,reason); err != nil { return err }

	severity := "medium"
	if !enabled { severity = "high" }
	if _, err := tx.Exec(ctx, `
		INSERT INTO security_events(event_type,severity,user_id,subject_type,subject_id,metadata)
		VALUES('platform_feature_flag_changed',$1,$2::uuid,'feature_flag',$3,
		       jsonb_build_object('previous',$4,'enabled',$5,'reason',$6,'actor_role',$7))`,
		severity,actor.UserID,key,previous,enabled,reason,strongestRole(actor)); err != nil { return err }

	return tx.Commit(ctx)
}

func (s *PostgresStore) SecuritySummary(ctx context.Context) (SecuritySummary, error) {
	var result SecuritySummary
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE severity='critical' AND created_at >= now()-interval '15 minutes'),
			count(*) FILTER (WHERE severity='high' AND created_at >= now()-interval '15 minutes'),
			count(*) FILTER (WHERE severity='medium' AND created_at >= now()-interval '15 minutes'),
			count(*) FILTER (WHERE severity='critical' AND created_at >= now()-interval '1 hour'),
			count(*) FILTER (WHERE severity='high' AND created_at >= now()-interval '1 hour'),
			count(*) FILTER (WHERE created_at >= now()-interval '24 hours')
		FROM security_events`).Scan(&result.Critical15m,&result.High15m,&result.Medium15m,&result.Critical1h,&result.High1h,&result.Events24h)
	if err != nil { return SecuritySummary{}, err }
	rows, err := s.pool.Query(ctx, `SELECT event_type,count(*)::bigint FROM security_events WHERE created_at>=now()-interval '1 hour' GROUP BY event_type ORDER BY count(*) DESC,event_type LIMIT 8`)
	if err != nil { return SecuritySummary{}, err }
	for rows.Next() { var item SecurityCounter; if err:=rows.Scan(&item.Key,&item.Count);err!=nil{rows.Close();return SecuritySummary{},err};result.TopTypes1h=append(result.TopTypes1h,item) }
	if err:=rows.Err();err!=nil{rows.Close();return SecuritySummary{},err};rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT source_ip::text,count(*)::bigint FROM security_events WHERE created_at>=now()-interval '1 hour' AND source_ip IS NOT NULL GROUP BY source_ip ORDER BY count(*) DESC,source_ip LIMIT 8`)
	if err != nil { return SecuritySummary{}, err }
	for rows.Next() { var item SecurityCounter; if err:=rows.Scan(&item.Key,&item.Count);err!=nil{rows.Close();return SecuritySummary{},err};result.TopIPs1h=append(result.TopIPs1h,item) }
	if err:=rows.Err();err!=nil{rows.Close();return SecuritySummary{},err};rows.Close()
	result.GeneratedAt=time.Now().UTC()
	return result,nil
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
