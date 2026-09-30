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
