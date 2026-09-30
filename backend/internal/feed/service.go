package feed

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidMode = errors.New("invalid feed mode")
	ErrInvalidFeedback = errors.New("invalid feed feedback")
)

type MediaRef struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	URL      string `json:"url"`
}

type Item struct {
	ID             string    `json:"id"`
	AuthorID       string    `json:"author_id"`
	AuthorUsername string    `json:"author_username"`
	AuthorName     string    `json:"author_name"`
	Kind           string    `json:"kind"`
	Body           string    `json:"body"`
	Cover          *MediaRef `json:"cover,omitempty"`
	RepliesCount   int64     `json:"replies_count"`
	ReactionsCount int64     `json:"reactions_count"`
	CreatedAt      time.Time `json:"created_at"`
	Reason         string    `json:"reason"`
	Score          int       `json:"-"`
}

type Store interface {
	List(ctx context.Context, userID, mode string, before time.Time, limit int) ([]Item, error)
	SetFeedback(ctx context.Context, userID, postID, signal string) error
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

func (s *Service) SetFeedback(ctx context.Context, userID, postID, signal string) error {
	userID = strings.TrimSpace(userID)
	postID = strings.TrimSpace(postID)
	signal = strings.ToLower(strings.TrimSpace(signal))
	if userID == "" || !looksLikeUUID(postID) {
		return ErrInvalidFeedback
	}
	switch signal {
	case "more_like_this", "not_interested", "hide":
	default:
		return ErrInvalidFeedback
	}
	return s.store.SetFeedback(ctx, userID, postID, signal)
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
