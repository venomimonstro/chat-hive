package social

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrProfileNotFound   = errors.New("profile not found")
	ErrInteractionDenied = errors.New("interaction denied")
)

type PublicProfile struct {
	UserID         string   `json:"user_id"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"display_name"`
	Bio            string   `json:"bio"`
	Interests      []string `json:"interests"`
	FollowersCount int64    `json:"followers_count"`
	FollowingCount int64    `json:"following_count"`
	IsFollowing    bool     `json:"is_following"`
	IsBlocked      bool     `json:"is_blocked"`
	IsSelf         bool     `json:"is_self"`
}

type Connection struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Bio             string `json:"bio"`
	FollowersCount  int64  `json:"followers_count"`
	SharedInterests int64  `json:"shared_interests"`
	IsFollowing     bool   `json:"is_following"`
}

type Recommendation struct {
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	Bio               string `json:"bio"`
	FollowersCount    int64  `json:"followers_count"`
	SharedInterests   int64  `json:"shared_interests"`
	MutualConnections int64  `json:"mutual_connections"`
	Reason            string `json:"reason"`
}

type Store interface {
	GetProfile(ctx context.Context, viewerID, username string) (PublicProfile, error)
	ListFollowers(ctx context.Context, viewerID, username string, limit int) ([]Connection, error)
	ListFollowing(ctx context.Context, viewerID, username string, limit int) ([]Connection, error)
	RecommendPeople(ctx context.Context, viewerID string, limit int) ([]Recommendation, error)
	SetFollow(ctx context.Context, followerID, username string, follow bool) error
	SetBlock(ctx context.Context, blockerID, username string, block bool) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) GetProfile(ctx context.Context, viewerID, username string) (PublicProfile, error) {
	username = normalizeUsername(username)
	if username == "" { return PublicProfile{}, ErrProfileNotFound }
	return s.store.GetProfile(ctx, viewerID, username)
}

func (s *Service) ListFollowers(ctx context.Context, viewerID, username string, limit int) ([]Connection, error) {
	username = normalizeUsername(username)
	if username == "" { return nil, ErrProfileNotFound }
	return s.store.ListFollowers(ctx, viewerID, username, normalizeLimit(limit))
}

func (s *Service) ListFollowing(ctx context.Context, viewerID, username string, limit int) ([]Connection, error) {
	username = normalizeUsername(username)
	if username == "" { return nil, ErrProfileNotFound }
	return s.store.ListFollowing(ctx, viewerID, username, normalizeLimit(limit))
}

func (s *Service) RecommendPeople(ctx context.Context, viewerID string, limit int) ([]Recommendation, error) {
	viewerID = strings.TrimSpace(viewerID)
	if viewerID == "" { return nil, ErrInteractionDenied }
	return s.store.RecommendPeople(ctx, viewerID, normalizeLimit(limit))
}

func (s *Service) Follow(ctx context.Context, userID, username string) error {
	return s.setFollow(ctx, userID, username, true)
}

func (s *Service) Unfollow(ctx context.Context, userID, username string) error {
	return s.setFollow(ctx, userID, username, false)
}

func (s *Service) setFollow(ctx context.Context, userID, username string, follow bool) error {
	username = normalizeUsername(username)
	if strings.TrimSpace(userID) == "" || username == "" { return ErrInteractionDenied }
	if err := s.store.SetFollow(ctx, userID, username, follow); err != nil {
		if errors.Is(err, ErrProfileNotFound) || errors.Is(err, ErrInteractionDenied) { return err }
		return fmt.Errorf("set follow: %w", err)
	}
	return nil
}

func (s *Service) Block(ctx context.Context, userID, username string) error {
	return s.setBlock(ctx, userID, username, true)
}

func (s *Service) Unblock(ctx context.Context, userID, username string) error {
	return s.setBlock(ctx, userID, username, false)
}

func (s *Service) setBlock(ctx context.Context, userID, username string, block bool) error {
	username = normalizeUsername(username)
	if strings.TrimSpace(userID) == "" || username == "" { return ErrInteractionDenied }
	if err := s.store.SetBlock(ctx, userID, username, block); err != nil {
		if errors.Is(err, ErrProfileNotFound) || errors.Is(err, ErrInteractionDenied) { return err }
		return fmt.Errorf("set block: %w", err)
	}
	return nil
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@")))
}

func normalizeLimit(value int) int {
	if value <= 0 { return 50 }
	if value > 100 { return 100 }
	return value
}
