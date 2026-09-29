package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidChallenge = errors.New("invalid or expired login challenge")
	ErrRateLimited      = errors.New("too many login attempts")
)

type Store interface {
	CountRecentChallenges(ctx context.Context, email, requestIP string, since time.Time) (int, error)
	CreateLoginChallenge(ctx context.Context, email string, tokenHash []byte, requestIP string, expiresAt time.Time) error
	ConsumeLoginChallenge(ctx context.Context, tokenHash []byte, now time.Time) (userID string, err error)
	CreateSession(ctx context.Context, input CreateSessionInput) (string, error)
}

type MagicLinkSender interface {
	SendMagicLink(ctx context.Context, email, rawToken string) error
}

type CreateSessionInput struct {
	UserID           string
	AccessTokenHash  []byte
	RefreshTokenHash []byte
	UserAgent        string
	IP               string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type SessionTokens struct {
	UserID       string
	SessionID    string
	AccessToken  string
	RefreshToken string
	AccessExpiry time.Time
	RefreshExpiry time.Time
}

type Service struct {
	store        Store
	sender       MagicLinkSender
	now          func() time.Time
	challengeTTL time.Duration
	accessTTL    time.Duration
	refreshTTL   time.Duration
	attemptLimit int
	attemptWindow time.Duration
}

func NewService(store Store, sender MagicLinkSender) *Service {
	return &Service{
		store:         store,
		sender:        sender,
		now:           func() time.Time { return time.Now().UTC() },
		challengeTTL:  15 * time.Minute,
		accessTTL:     15 * time.Minute,
		refreshTTL:    30 * 24 * time.Hour,
		attemptLimit:  5,
		attemptWindow: 15 * time.Minute,
	}
}

func (s *Service) StartEmailLogin(ctx context.Context, email, requestIP string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}

	now := s.now()
	count, err := s.store.CountRecentChallenges(ctx, normalized, requestIP, now.Add(-s.attemptWindow))
	if err != nil {
		return fmt.Errorf("count recent challenges: %w", err)
	}
	if count >= s.attemptLimit {
		return ErrRateLimited
	}

	rawToken, tokenHash, err := newToken()
	if err != nil {
		return fmt.Errorf("generate login token: %w", err)
	}
	if err := s.store.CreateLoginChallenge(ctx, normalized, tokenHash, requestIP, now.Add(s.challengeTTL)); err != nil {
		return fmt.Errorf("persist login challenge: %w", err)
	}
	if err := s.sender.SendMagicLink(ctx, normalized, rawToken); err != nil {
		return fmt.Errorf("send magic link: %w", err)
	}
	return nil
}

func (s *Service) CompleteEmailLogin(ctx context.Context, rawToken, userAgent, requestIP string) (SessionTokens, error) {
	if strings.TrimSpace(rawToken) == "" || len(rawToken) > 512 {
		return SessionTokens{}, ErrInvalidChallenge
	}

	now := s.now()
	tokenHash := hashToken(rawToken)
	userID, err := s.store.ConsumeLoginChallenge(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, ErrInvalidChallenge) {
			return SessionTokens{}, ErrInvalidChallenge
		}
		return SessionTokens{}, fmt.Errorf("consume login challenge: %w", err)
	}

	accessToken, accessHash, err := newToken()
	if err != nil {
		return SessionTokens{}, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, refreshHash, err := newToken()
	if err != nil {
		return SessionTokens{}, fmt.Errorf("generate refresh token: %w", err)
	}

	accessExpiry := now.Add(s.accessTTL)
	refreshExpiry := now.Add(s.refreshTTL)
	sessionID, err := s.store.CreateSession(ctx, CreateSessionInput{
		UserID:           userID,
		AccessTokenHash:  accessHash,
		RefreshTokenHash: refreshHash,
		UserAgent:        truncate(userAgent, 512),
		IP:               requestIP,
		AccessExpiresAt:  accessExpiry,
		RefreshExpiresAt: refreshExpiry,
	})
	if err != nil {
		return SessionTokens{}, fmt.Errorf("create session: %w", err)
	}

	return SessionTokens{
		UserID:        userID,
		SessionID:     sessionID,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		AccessExpiry:  accessExpiry,
		RefreshExpiry: refreshExpiry,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || len(value) > 254 {
		return "", fmt.Errorf("invalid email")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || strings.ToLower(parsed.Address) != value {
		return "", fmt.Errorf("invalid email")
	}
	return value, nil
}

func newToken() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
