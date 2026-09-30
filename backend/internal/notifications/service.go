package notifications

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

var (
	ErrForbidden = errors.New("notification access denied")
	ErrNotFound  = errors.New("notification not found")
	ErrInvalid   = errors.New("invalid notification input")
)

type Notification struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	ActorID       *string    `json:"actor_id,omitempty"`
	ActorUsername string     `json:"actor_username,omitempty"`
	EntityType    string     `json:"entity_type"`
	EntityID      string     `json:"entity_id"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	ReadAt        *time.Time `json:"read_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type EmitInput struct {
	UserID     string
	Kind       string
	ActorID    string
	EntityType string
	EntityID   string
	Title      string
	Body       string
	DedupeKey  string
}

type PushSubscriptionInput struct {
	UserID    string
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent string
}

type Store interface {
	List(ctx context.Context, userID string, limit int) ([]Notification, int64, error)
	MarkRead(ctx context.Context, userID, notificationID string) error
	MarkAllRead(ctx context.Context, userID string) error
	Emit(ctx context.Context, input EmitInput) error
	UpsertPushSubscription(ctx context.Context, input PushSubscriptionInput) error
	RevokePushSubscription(ctx context.Context, userID, endpoint string) error
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
	userID = strings.TrimSpace(userID)
	notificationID = strings.TrimSpace(notificationID)
	if userID == "" || !looksLikeUUID(notificationID) { return ErrNotFound }
	return s.store.MarkRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" { return ErrForbidden }
	return s.store.MarkAllRead(ctx, userID)
}

// Emit is intentionally a small application boundary. Business modules depend on
// this behavior rather than notification persistence, so delivery can later move
// to NATS/outbox without changing their domain APIs.
func (s *Service) Emit(ctx context.Context, input EmitInput) error {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorID = strings.TrimSpace(input.ActorID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.EntityType = strings.ToLower(strings.TrimSpace(input.EntityType))
	input.EntityID = strings.TrimSpace(input.EntityID)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.DedupeKey = strings.TrimSpace(input.DedupeKey)
	if input.UserID == "" || input.Title == "" || len([]rune(input.Title)) > 140 || len([]rune(input.Body)) > 500 {
		return ErrInvalid
	}
	switch input.Kind {
	case "direct_message", "group_message", "channel_post", "post_reply", "follow", "moderation", "security":
	default:
		return ErrInvalid
	}
	return s.store.Emit(ctx, input)
}

func (s *Service) SubscribePush(ctx context.Context, input PushSubscriptionInput) error {
	input.UserID = strings.TrimSpace(input.UserID)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.P256DH = strings.TrimSpace(input.P256DH)
	input.Auth = strings.TrimSpace(input.Auth)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	parsed, err := url.Parse(input.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(input.Endpoint) > 2048 || input.P256DH == "" || input.Auth == "" || len(input.P256DH) > 512 || len(input.Auth) > 512 {
		return ErrInvalid
	}
	if input.UserID == "" { return ErrForbidden }
	if len(input.UserAgent) > 512 { input.UserAgent = input.UserAgent[:512] }
	return s.store.UpsertPushSubscription(ctx, input)
}

func (s *Service) UnsubscribePush(ctx context.Context, userID, endpoint string) error {
	userID = strings.TrimSpace(userID)
	endpoint = strings.TrimSpace(endpoint)
	if userID == "" { return ErrForbidden }
	if endpoint == "" || len(endpoint) > 2048 { return ErrInvalid }
	return s.store.RevokePushSubscription(ctx, userID, endpoint)
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 { return false }
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 { if char != '-' { return false }; continue }
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) { return false }
	}
	return true
}
