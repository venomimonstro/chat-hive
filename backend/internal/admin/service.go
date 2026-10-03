package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrForbidden = errors.New("admin access denied")
	ErrNotFound  = errors.New("admin object not found")
	ErrInvalid   = errors.New("invalid admin action")
)

type Principal struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

type Case struct {
	ID          string    `json:"id"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason"`
	ReportCount int64     `json:"report_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SecurityEvent struct {
	ID          int64          `json:"id"`
	EventType   string         `json:"event_type"`
	Severity    string         `json:"severity"`
	UserID      *string        `json:"user_id,omitempty"`
	SessionID   *string        `json:"session_id,omitempty"`
	SourceIP    *string        `json:"source_ip,omitempty"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type SecurityCounter struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type SecurityAlert struct {
	EventID       int64          `json:"event_id"`
	EventType     string         `json:"event_type"`
	Severity      string         `json:"severity"`
	Status        string         `json:"status"`
	SourceIP      *string        `json:"source_ip,omitempty"`
	SubjectType   string         `json:"subject_type"`
	SubjectID     string         `json:"subject_id"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	AcknowledgedAt *time.Time    `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string       `json:"acknowledged_by,omitempty"`
	Note          string         `json:"note"`
}

type SecuritySummary struct {
	Critical15m int64             `json:"critical_15m"`
	High15m     int64             `json:"high_15m"`
	Medium15m   int64             `json:"medium_15m"`
	Critical1h  int64             `json:"critical_1h"`
	High1h      int64             `json:"high_1h"`
	Events24h   int64             `json:"events_24h"`
	TopTypes1h  []SecurityCounter `json:"top_event_types_1h"`
	TopIPs1h    []SecurityCounter `json:"top_source_ips_1h"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type PlatformFlag struct {
	Key       string     `json:"key"`
	Enabled   bool       `json:"enabled"`
	Reason    string     `json:"reason"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type RuntimeSnapshot struct {
	RealtimeConnections int64     `json:"realtime_connections"`
	RealtimePublished   int64     `json:"realtime_published"`
	RealtimeDropped     int64     `json:"realtime_dropped"`
	DBAcquired          int32     `json:"db_acquired"`
	DBIdle              int32     `json:"db_idle"`
	DBMax               int32     `json:"db_max"`
	DBAcquireCount      int64     `json:"db_acquire_count"`
	DBAcquireDurationMS int64     `json:"db_acquire_duration_ms"`
	GeneratedAt         time.Time `json:"generated_at"`
}

type AuditInput struct {
	ActorUserID string
	ActorRole   string
	Action      string
	TargetType  string
	TargetID    string
	Reason      string
	RequestID   string
	SourceIP    string
}

type Store interface {
	Principal(ctx context.Context, userID string) (Principal, error)
	ListCases(ctx context.Context, statuses []string, limit int) ([]Case, error)
	ListSecurityEvents(ctx context.Context, severity string, limit int) ([]SecurityEvent, error)
	ListSecurityAlerts(ctx context.Context, status string, limit int) ([]SecurityAlert, error)
	AcknowledgeSecurityAlert(ctx context.Context, eventID int64, actor Principal, note string) error
	ListPlatformFlags(ctx context.Context) ([]PlatformFlag, error)
	SetPlatformFlag(ctx context.Context, key string, enabled bool, actor Principal, reason string) error
	SecuritySummary(ctx context.Context) (SecuritySummary, error)
	ResolveCase(ctx context.Context, actor Principal, caseID, decision, reason string) error
	WriteAudit(ctx context.Context, input AuditInput) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Authorize(ctx context.Context, userID string, allowedRoles ...string) (Principal, error) {
	principal, err := s.store.Principal(ctx, strings.TrimSpace(userID))
	if err != nil { return Principal{}, err }
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles { allowed[role] = struct{}{} }
	for _, role := range principal.Roles { if _, ok := allowed[role]; ok { return principal, nil } }
	return Principal{}, ErrForbidden
}

func (s *Service) ListCases(ctx context.Context, principal Principal, limit int) ([]Case, error) {
	if !hasAnyRole(principal, "moderator", "senior_moderator", "security", "legal", "owner") { return nil, ErrForbidden }
	if limit <= 0 { limit = 50 }; if limit > 200 { limit = 200 }
	return s.store.ListCases(ctx, []string{"open", "reviewing"}, limit)
}

func (s *Service) ListSecurityEvents(ctx context.Context, principal Principal, severity string, limit int) ([]SecurityEvent, error) {
	if !hasAnyRole(principal, "security", "owner") { return nil, ErrForbidden }
	severity = strings.ToLower(strings.TrimSpace(severity))
	if severity != "" && severity != "info" && severity != "low" && severity != "medium" && severity != "high" && severity != "critical" { return nil, ErrInvalid }
	if limit <= 0 { limit = 100 }; if limit > 500 { limit = 500 }
	return s.store.ListSecurityEvents(ctx, severity, limit)
}

func (s *Service) ListSecurityAlerts(ctx context.Context, principal Principal, status string, limit int) ([]SecurityAlert, error) {
	if !hasAnyRole(principal, "security", "owner") { return nil, ErrForbidden }
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" { status = "open" }
	if status != "open" && status != "acknowledged" && status != "resolved" && status != "all" { return nil, ErrInvalid }
	if limit <= 0 { limit = 100 }; if limit > 500 { limit = 500 }
	return s.store.ListSecurityAlerts(ctx, status, limit)
}

func (s *Service) AcknowledgeSecurityAlert(ctx context.Context, principal Principal, eventID int64, note string) error {
	if !hasAnyRole(principal, "security", "owner") { return ErrForbidden }
	note = strings.TrimSpace(note)
	if eventID <= 0 || len([]rune(note)) < 3 || len([]rune(note)) > 1000 { return ErrInvalid }
	return s.store.AcknowledgeSecurityAlert(ctx, eventID, principal, note)
}

func (s *Service) ListPlatformFlags(ctx context.Context, principal Principal) ([]PlatformFlag, error) {
	if !hasAnyRole(principal, "security", "system_admin", "owner") { return nil, ErrForbidden }
	return s.store.ListPlatformFlags(ctx)
}

func (s *Service) SetPlatformFlag(ctx context.Context, principal Principal, key string, enabled bool, reason string) error {
	if !hasAnyRole(principal, "security", "system_admin", "owner") { return ErrForbidden }
	key = strings.TrimSpace(key)
	reason = strings.TrimSpace(reason)
	if !allowedPlatformFlag(key) || len([]rune(reason)) < 3 || len([]rune(reason)) > 1000 { return ErrInvalid }
	return s.store.SetPlatformFlag(ctx, key, enabled, principal, reason)
}

func allowedPlatformFlag(key string) bool {
	switch key {
	case "feed.enabled", "discovery.enabled", "posting.enabled", "community_creation.enabled", "channel_creation.enabled", "media_upload.enabled":
		return true
	default:
		return false
	}
}

func (s *Service) GetSecuritySummary(ctx context.Context, principal Principal) (SecuritySummary, error) {
	if !hasAnyRole(principal, "security", "owner") { return SecuritySummary{}, ErrForbidden }
	return s.store.SecuritySummary(ctx)
}

func (s *Service) ResolveCase(ctx context.Context, principal Principal, caseID, decision, reason string) error {
	if !hasAnyRole(principal, "senior_moderator", "security", "legal", "owner") { return ErrForbidden }
	caseID = strings.TrimSpace(caseID); decision = strings.ToLower(strings.TrimSpace(decision)); reason = strings.TrimSpace(reason)
	if !looksLikeUUID(caseID) || (decision != "resolve" && decision != "dismiss") || len([]rune(reason)) < 3 || len([]rune(reason)) > 1000 { return ErrInvalid }
	return s.store.ResolveCase(ctx, principal, caseID, decision, reason)
}

func (s *Service) Audit(ctx context.Context, input AuditInput) error { return s.store.WriteAudit(ctx, input) }

func hasAnyRole(principal Principal, roles ...string) bool {
	wanted := make(map[string]struct{}, len(roles))
	for _, role := range roles { wanted[role] = struct{}{} }
	for _, role := range principal.Roles { if _, ok := wanted[role]; ok { return true } }
	return false
}

func strongestRole(principal Principal) string {
	order := []string{"owner","security","legal","senior_moderator","moderator","system_admin","support"}
	for _, expected := range order { for _, role := range principal.Roles { if role == expected { return role } } }
	return "unknown"
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 { return false }
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 { if char != '-' { return false }; continue }
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) { return false }
	}
	return true
}
