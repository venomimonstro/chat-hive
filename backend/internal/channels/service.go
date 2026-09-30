package channels

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid   = errors.New("invalid channel")
	ErrNotFound  = errors.New("channel not found")
	ErrForbidden = errors.New("channel action forbidden")
	ErrSlugTaken = errors.New("channel slug taken")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,47}$`)

type Channel struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id"`
	Slug             string    `json:"slug"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	ModerationStatus string    `json:"moderation_status"`
	SubscribersCount int64     `json:"subscribers_count"`
	Subscribed       bool      `json:"subscribed"`
	Mine             bool      `json:"mine"`
	CreatedAt        time.Time `json:"created_at"`
}

type Post struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateInput struct {
	OwnerID     string
	Slug        string
	Title       string
	Description string
}

type Store interface {
	Create(ctx context.Context, input CreateInput) (Channel, error)
	Get(ctx context.Context, viewerID, slug string) (Channel, error)
	Discover(ctx context.Context, viewerID string, limit int) ([]Channel, error)
	SetSubscription(ctx context.Context, userID, slug string, enabled bool) (Channel, error)
	CreatePost(ctx context.Context, userID, slug, body string) (Post, error)
	ListPosts(ctx context.Context, viewerID, slug string, limit int) ([]Post, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, input CreateInput) (Channel, error) {
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.Slug = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(input.Slug, "@")))
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.OwnerID == "" || !slugPattern.MatchString(input.Slug) || len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 1000 {
		return Channel{}, ErrInvalid
	}
	return s.store.Create(ctx, input)
}

func (s *Service) Get(ctx context.Context, viewerID, slug string) (Channel, error) {
	slug = normalizeSlug(slug)
	if strings.TrimSpace(viewerID) == "" || !slugPattern.MatchString(slug) { return Channel{}, ErrNotFound }
	return s.store.Get(ctx, viewerID, slug)
}

func (s *Service) Discover(ctx context.Context, viewerID string, limit int) ([]Channel, error) {
	if strings.TrimSpace(viewerID) == "" { return nil, ErrForbidden }
	if limit <= 0 { limit = 30 }
	if limit > 50 { limit = 50 }
	return s.store.Discover(ctx, viewerID, limit)
}

func (s *Service) SetSubscription(ctx context.Context, userID, slug string, enabled bool) (Channel, error) {
	userID = strings.TrimSpace(userID); slug = normalizeSlug(slug)
	if userID == "" || !slugPattern.MatchString(slug) { return Channel{}, ErrInvalid }
	return s.store.SetSubscription(ctx, userID, slug, enabled)
}

func (s *Service) CreatePost(ctx context.Context, userID, slug, body string) (Post, error) {
	userID = strings.TrimSpace(userID); slug = normalizeSlug(slug); body = strings.TrimSpace(body)
	if userID == "" || !slugPattern.MatchString(slug) || len([]rune(body)) < 1 || len([]rune(body)) > 8000 { return Post{}, ErrInvalid }
	return s.store.CreatePost(ctx, userID, slug, body)
}

func (s *Service) ListPosts(ctx context.Context, viewerID, slug string, limit int) ([]Post, error) {
	viewerID = strings.TrimSpace(viewerID); slug = normalizeSlug(slug)
	if viewerID == "" || !slugPattern.MatchString(slug) { return nil, ErrNotFound }
	if limit <= 0 { limit = 30 }; if limit > 50 { limit = 50 }
	return s.store.ListPosts(ctx, viewerID, slug, limit)
}

func normalizeSlug(value string) string { return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@"))) }
