package moderation

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidReport = errors.New("invalid report")
	ErrNotFound      = errors.New("report target not found")
	ErrForbidden     = errors.New("report target forbidden")
)

type Report struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Category   string    `json:"category"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateInput struct {
	ReporterID string
	TargetType string
	TargetID   string
	Category   string
	Note       string
}

type Store interface {
	CreateReport(ctx context.Context, input CreateInput) (Report, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) CreateReport(ctx context.Context, input CreateInput) (Report, error) {
	input.ReporterID = strings.TrimSpace(input.ReporterID)
	input.TargetType = strings.ToLower(strings.TrimSpace(input.TargetType))
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Category = strings.ToLower(strings.TrimSpace(input.Category))
	input.Note = strings.TrimSpace(input.Note)
	if input.ReporterID == "" || !validTargetType(input.TargetType) || !looksLikeUUID(input.TargetID) || !validCategory(input.Category) || len([]rune(input.Note)) > 1000 {
		return Report{}, ErrInvalidReport
	}
	return s.store.CreateReport(ctx, input)
}

func validTargetType(value string) bool {
	switch value {
	case "user", "message", "post", "group":
		return true
	default:
		return false
	}
}

func validCategory(value string) bool {
	switch value {
	case "spam", "scam", "harassment", "threats", "illegal_content", "sexual_content", "impersonation", "malware", "other":
		return true
	default:
		return false
	}
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 { return false }
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if char != '-' { return false }
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) { return false }
	}
	return true
}
