package groups

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidGroup  = errors.New("invalid group")
	ErrNotFound      = errors.New("group not found")
	ErrForbidden     = errors.New("forbidden")
	ErrInviteInvalid = errors.New("invalid invite")
)

type Group struct {
	ChatID       string `json:"chat_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Role         string `json:"role"`
	MembersCount int64  `json:"members_count"`
}

type Member struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

type Store interface {
	Create(ctx context.Context, ownerID, title, description string) (Group, error)
	Get(ctx context.Context, userID, chatID string) (Group, error)
	ListMembers(ctx context.Context, userID, chatID string, limit int) ([]Member, error)
	SetMemberRole(ctx context.Context, actorID, chatID, targetUsername, role string) error
	TransferOwnership(ctx context.Context, actorID, chatID, targetUsername string) error
	RemoveMember(ctx context.Context, actorID, chatID, targetUsername string) error
	Leave(ctx context.Context, userID, chatID string) error
	CreateInvite(ctx context.Context, actorID, chatID string, tokenHash []byte, expiresAt *time.Time, maxUses *int) error
	JoinByInvite(ctx context.Context, userID string, tokenHash []byte, now time.Time) (Group, error)
	RevokeInvite(ctx context.Context, actorID, chatID string, tokenHash []byte) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Create(ctx context.Context, ownerID, title, description string) (Group, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if ownerID == "" || len([]rune(title)) < 2 || len([]rune(title)) > 80 || len([]rune(description)) > 500 {
		return Group{}, ErrInvalidGroup
	}
	return s.store.Create(ctx, ownerID, title, description)
}

func (s *Service) Get(ctx context.Context, userID, chatID string) (Group, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(chatID) == "" {
		return Group{}, ErrNotFound
	}
	return s.store.Get(ctx, userID, chatID)
}

func (s *Service) ListMembers(ctx context.Context, userID, chatID string, limit int) ([]Member, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListMembers(ctx, userID, chatID, limit)
}

func (s *Service) SetRole(ctx context.Context, actorID, chatID, username, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "admin" && role != "member" {
		return ErrInvalidGroup
	}
	return s.store.SetMemberRole(ctx, actorID, chatID, strings.TrimSpace(username), role)
}

func (s *Service) TransferOwnership(ctx context.Context, actorID, chatID, username string) error {
	actorID = strings.TrimSpace(actorID)
	chatID = strings.TrimSpace(chatID)
	username = strings.TrimSpace(username)
	if actorID == "" || chatID == "" || username == "" {
		return ErrInvalidGroup
	}
	return s.store.TransferOwnership(ctx, actorID, chatID, username)
}

func (s *Service) RemoveMember(ctx context.Context, actorID, chatID, username string) error {
	return s.store.RemoveMember(ctx, actorID, chatID, strings.TrimSpace(username))
}

func (s *Service) Leave(ctx context.Context, userID, chatID string) error {
	return s.store.Leave(ctx, userID, chatID)
}

func (s *Service) CreateInvite(ctx context.Context, actorID, chatID string, ttl time.Duration, maxUses int) (string, error) {
	if ttl <= 0 || ttl > 30*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	var max *int
	if maxUses > 0 {
		if maxUses > 10000 {
			maxUses = 10000
		}
		max = &maxUses
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	expires := s.now().Add(ttl)
	if err := s.store.CreateInvite(ctx, actorID, chatID, hash, &expires, max); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) JoinByInvite(ctx context.Context, userID, rawToken string) (Group, error) {
	rawToken = strings.TrimSpace(rawToken)
	if userID == "" || rawToken == "" || len(rawToken) > 512 {
		return Group{}, ErrInviteInvalid
	}
	return s.store.JoinByInvite(ctx, userID, hashToken(rawToken), s.now())
}

func (s *Service) RevokeInvite(ctx context.Context, actorID, chatID, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInviteInvalid
	}
	return s.store.RevokeInvite(ctx, actorID, chatID, hashToken(rawToken))
}

func newToken() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate invite token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
