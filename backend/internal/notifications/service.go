package notifications

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrForbidden = errors.New("notification access denied")
	ErrNotFound  = errors.New("notification not found")
)

type Notification struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	ActorID    *string    `json:"actor_id,omitempty"`
	EntityType string     `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Store interface {
	List(ctx context.Context, userID string, limit int) ([]Notification, int64, error)
	MarkRead(ctx context.Context, userID, notificationID string) error
	MarkAllRead(ctx context.Context, userID string) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) List(ctx context.Context, userID string, limit int) ([]Notification, int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" { return nil, 0, ErrForbidden }
	if limit <= 0 { limit = 50 }
	if limit > 100 { limit = 100 }
	return s.store.List(ctx, userID, limit)
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID string) error {
	userID = strings.TrimSpace(userID); notificationID = strings.TrimSpace(notificationID)
	if userID == "" || !looksLikeUUID(notificationID) { return ErrNotFound }
	return s.store.MarkRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" { return ErrForbidden }
	return s.store.MarkAllRead(ctx, userID)
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 { return false }
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 { if char != '-' { return false }; continue }
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) { return false }
	}
	return true
}
