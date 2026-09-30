package communities

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid   = errors.New("invalid community")
	ErrNotFound  = errors.New("community not found")
	ErrForbidden = errors.New("community action forbidden")
	ErrSlugTaken = errors.New("community slug taken")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,47}$`)

type Community struct {
	ID               string    `json:"id"`
	ChatID           string    `json:"chat_id"`
	Slug             string    `json:"slug"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Visibility       string    `json:"visibility"`
	ModerationStatus string    `json:"moderation_status"`
	Role             string    `json:"role"`
	MembersCount     int64     `json:"members_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateInput struct {
	OwnerID     string
	Slug        string
	Title       string
	Description string
	Visibility  string
}

type Store interface {
	Create(ctx context.Context, input CreateInput) (Community, error)
	Get(ctx context.Context, viewerID, slug string) (Community, error)
	Discover(ctx context.Context, viewerID string, limit int) ([]Community, error)
	Join(ctx context.Context, userID, slug string) (Community, error)
	Leave(ctx context.Context, userID, slug string) error
}

type PublicStore interface {
	GetPublic(ctx context.Context, slug string) (Community, error)
	DiscoverPublic(ctx context.Context, limit int) ([]Community, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) publicStore() (PublicStore, bool) { store, ok := s.store.(PublicStore); return store, ok }

func (s *Service) Create(ctx context.Context, input CreateInput) (Community, error) {
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.Slug = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(input.Slug, "@")))
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Visibility = strings.ToLower(strings.TrimSpace(input.Visibility))
	if input.Visibility == "" { input.Visibility = "private" }
	if input.OwnerID == "" || !slugPattern.MatchString(input.Slug) || len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 1000 || (input.Visibility != "private" && input.Visibility != "public") { return Community{}, ErrInvalid }
	return s.store.Create(ctx, input)
}

func (s *Service) Get(ctx context.Context, viewerID, slug string) (Community, error) {
	viewerID = strings.TrimSpace(viewerID)
	slug = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(slug, "@")))
	if !slugPattern.MatchString(slug) { return Community{}, ErrNotFound }
	if viewerID == "" {
		store, ok := s.publicStore(); if !ok { return Community{}, ErrNotFound }
		return store.GetPublic(ctx, slug)
	}
	return s.store.Get(ctx, viewerID, slug)
}

func (s *Service) Discover(ctx context.Context, viewerID string, limit int) ([]Community, error) {
	viewerID = strings.TrimSpace(viewerID)
	if limit <= 0 { limit = 30 }; if limit > 50 { limit = 50 }
	if viewerID == "" {
		store, ok := s.publicStore(); if !ok { return nil, ErrForbidden }
		return store.DiscoverPublic(ctx, limit)
	}
	return s.store.Discover(ctx, viewerID, limit)
}

func (s *Service) Join(ctx context.Context, userID, slug string) (Community, error) {
	userID = strings.TrimSpace(userID)
	slug = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(slug, "@")))
	if userID == "" || !slugPattern.MatchString(slug) { return Community{}, ErrInvalid }
	return s.store.Join(ctx, userID, slug)
}

func (s *Service) Leave(ctx context.Context, userID, slug string) error {
	userID = strings.TrimSpace(userID)
	slug = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(slug, "@")))
	if userID == "" || !slugPattern.MatchString(slug) { return ErrInvalid }
	return s.store.Leave(ctx, userID, slug)
}
