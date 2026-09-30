package feed

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalidMode = errors.New("invalid feed mode")

type Item struct {
	ID             string    `json:"id"`
	AuthorID       string    `json:"author_id"`
	AuthorUsername string    `json:"author_username"`
	AuthorName     string    `json:"author_name"`
	Kind           string    `json:"kind"`
	Body           string    `json:"body"`
	RepliesCount   int64     `json:"replies_count"`
	ReactionsCount int64     `json:"reactions_count"`
	CreatedAt      time.Time `json:"created_at"`
	Reason         string    `json:"reason"`
	Score          int       `json:"-"`
}

type Store interface {
	List(ctx context.Context, userID, mode string, before time.Time, limit int) ([]Item, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) List(ctx context.Context, userID, mode string, before time.Time, limit int) ([]Item, error) {
	userID = strings.TrimSpace(userID)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "for-you"
	}
	if mode != "for-you" && mode != "following" {
		return nil, ErrInvalidMode
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 50 {
		limit = 50
	}
	return s.store.List(ctx, userID, mode, before, limit)
}
