package securityplane

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	Type        string
	Severity    string
	UserID      string
	SessionID   string
	SourceIP    string
	SubjectType string
	SubjectID   string
	Metadata    map[string]any
}

type Recorder struct {
	pool *pgxpool.Pool
}

func NewRecorder(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool}
}

func (r *Recorder) Record(ctx context.Context, event Event) error {
	if r == nil || r.pool == nil {
		return nil
	}
	if !validSeverity(event.Severity) || strings.TrimSpace(event.Type) == "" {
		return nil
	}
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	ip := strings.TrimSpace(event.SourceIP)
	if parsed := net.ParseIP(ip); parsed == nil {
		ip = ""
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO security_events(
			event_type,severity,user_id,session_id,source_ip,subject_type,subject_id,metadata,created_at
		)
		VALUES(
			$1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::inet,$6,$7,$8::jsonb,$9
		)`,
		event.Type,
		event.Severity,
		strings.TrimSpace(event.UserID),
		strings.TrimSpace(event.SessionID),
		ip,
		strings.TrimSpace(event.SubjectType),
		strings.TrimSpace(event.SubjectID),
		string(encoded),
		time.Now().UTC(),
	)
	return err
}

func (r *Recorder) RateLimitExceeded(ctx context.Context, sourceIP, bucket, method, path string, retryAfter int) error {
	severity := "low"
	switch bucket {
	case "auth", "realtime-ticket":
		severity = "medium"
	case "upload", "message":
		severity = "low"
	}
	return r.Record(ctx, Event{
		Type:        "rate_limit_exceeded",
		Severity:    severity,
		SourceIP:    sourceIP,
		SubjectType: "http_rate_limit",
		SubjectID:   bucket,
		Metadata: map[string]any{
			"method":      method,
			"path":        path,
			"retry_after": retryAfter,
		},
	})
}

func validSeverity(value string) bool {
	switch value {
	case "info", "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}
