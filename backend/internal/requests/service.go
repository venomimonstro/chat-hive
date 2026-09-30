package requests

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("message request not found")
	ErrForbidden = errors.New("message request forbidden")
)

type Item struct {
	ChatID          string    `json:"chat_id"`
	RequesterID     string    `json:"requester_id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
	SharedInterests int64     `json:"shared_interests"`
	MutualFollowers int64     `json:"mutual_followers"`
}

type Store interface {
	List(ctx context.Context, userID string, limit int) ([]Item, error)
	Decide(ctx context.Context, userID, chatID string, accept bool) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) List(ctx context.Context, userID string, limit int) ([]Item, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrForbidden
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.List(ctx, userID, limit)
}

func (s *Service) Accept(ctx context.Context, userID, chatID string) error {
	return s.decide(ctx, userID, chatID, true)
}

func (s *Service) Reject(ctx context.Context, userID, chatID string) error {
	return s.decide(ctx, userID, chatID, false)
}

func (s *Service) decide(ctx context.Context, userID, chatID string, accept bool) error {
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if userID == "" || !looksLikeUUID(chatID) {
		return ErrNotFound
	}
	return s.store.Decide(ctx, userID, chatID, accept)
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
