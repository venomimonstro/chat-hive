package onboarding

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidProfile = errors.New("invalid profile")
	ErrUsernameTaken  = errors.New("username already taken")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

type Interest struct {
	Slug    string `json:"slug"`
	LabelRU string `json:"label_ru"`
	LabelEN string `json:"label_en"`
}

type Profile struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Bio         string   `json:"bio"`
	Interests   []string `json:"interests"`
	Completed   bool     `json:"completed"`
}

type CompleteInput struct {
	UserID      string
	Username    string
	DisplayName string
	Bio         string
	Interests   []string
}

type Store interface {
	ListInterests(ctx context.Context) ([]Interest, error)
	GetProfile(ctx context.Context, userID string) (Profile, error)
	Complete(ctx context.Context, input CompleteInput) (Profile, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) ListInterests(ctx context.Context) ([]Interest, error) {
	return s.store.ListInterests(ctx)
}

func (s *Service) GetProfile(ctx context.Context, userID string) (Profile, error) {
	return s.store.GetProfile(ctx, userID)
}

func (s *Service) Complete(ctx context.Context, input CompleteInput) (Profile, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Bio = strings.TrimSpace(input.Bio)
	if !usernamePattern.MatchString(input.Username) || len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 80 || len([]rune(input.Bio)) > 240 {
		return Profile{}, ErrInvalidProfile
	}

	seen := map[string]struct{}{}
	clean := make([]string, 0, len(input.Interests))
	for _, slug := range input.Interests {
		slug = strings.ToLower(strings.TrimSpace(slug))
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		clean = append(clean, slug)
	}
	if len(clean) < 3 || len(clean) > 12 {
		return Profile{}, fmt.Errorf("%w: choose 3 to 12 interests", ErrInvalidProfile)
	}
	input.Interests = clean
	return s.store.Complete(ctx, input)
}
