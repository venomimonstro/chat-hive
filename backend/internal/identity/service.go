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
	ErrInvalidSession   = errors.New("invalid or expired session")
	ErrRefreshReuse     = errors.New("refresh token reuse detected")
	ErrRateLimited      = errors.New("too many login attempts")
)

type Store interface {
	CountRecentChallenges(ctx context.Context, email, requestIP string, since time.Time) (int, error)
	CreateLoginChallenge(ctx context.Context, email string, tokenHash []byte, requestIP string, expiresAt time.Time) error
	ConsumeLoginChallenge(ctx context.Context, tokenHash []byte, now time.Time) (userID string, err error)
	CreateSession(ctx context.Context, input CreateSessionInput) (string, error)
	FindSessionByAccessTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (AuthenticatedSession, error)
	RotateRefreshToken(ctx context.Context, input RotateSessionInput) (AuthenticatedSession, error)
	ListSessions(ctx context.Context, userID string, now time.Time) ([]DeviceSession, error)
	RevokeSession(ctx context.Context, userID, sessionID string, now time.Time) (bool, error)
}

type MagicLinkSender interface {
	SendMagicLink(ctx context.Context, email, rawToken string) error
}

type CreateSessionInput struct {
	UserID           string
	AuthMethod       string
	AccessTokenHash  []byte
	RefreshTokenHash []byte
	UserAgent        string
	IP               string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type RotateSessionInput struct {
	OldRefreshTokenHash []byte
	NewRefreshTokenHash []byte
	NewAccessTokenHash  []byte
	AccessExpiresAt     time.Time
	RefreshExpiresAt    time.Time
	Now                 time.Time
}

type AuthenticatedSession struct {
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	AuthMethod string    `json:"auth_method"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type DeviceSession struct {
	SessionID  string    `json:"session_id"`
	AuthMethod string    `json:"auth_method"`
	UserAgent  string    `json:"user_agent"`
	LastIP     string    `json:"last_ip"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type SessionTokens struct {
	UserID        string
	AuthMethod    string
	SessionID     string
	AccessToken   string
	RefreshToken  string
	AccessExpiry  time.Time
	RefreshExpiry time.Time
}

type Service struct {
	store         Store
	sender        MagicLinkSender
	now           func() time.Time
	challengeTTL  time.Duration
	accessTTL     time.Duration
	refreshTTL    time.Duration
	attemptLimit  int
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
	userID, err := s.store.ConsumeLoginChallenge(ctx, hashToken(rawToken), now)
	if err != nil {
		if errors.Is(err, ErrInvalidChallenge) {
			return SessionTokens{}, ErrInvalidChallenge
		}
		return SessionTokens{}, fmt.Errorf("consume login challenge: %w", err)
	}

	tokens, accessHash, refreshHash, err := s.newSessionTokens(now)
	if err != nil {
		return SessionTokens{}, err
	}
	tokens.UserID = userID
	tokens.AuthMethod = "email"

	sessionID, err := s.store.CreateSession(ctx, CreateSessionInput{
		UserID:           userID,
		AuthMethod:       "email",
		AccessTokenHash:  accessHash,
		RefreshTokenHash: refreshHash,
		UserAgent:        truncate(userAgent, 512),
		IP:               requestIP,
		AccessExpiresAt:  tokens.AccessExpiry,
		RefreshExpiresAt: tokens.RefreshExpiry,
	})
	if err != nil {
		return SessionTokens{}, fmt.Errorf("create session: %w", err)
	}
	tokens.SessionID = sessionID
	return tokens, nil
}

func (s *Service) CreateSessionForUser(ctx context.Context, userID, userAgent, requestIP string) (SessionTokens, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return SessionTokens{}, ErrInvalidSession
	}
	now := s.now()
	tokens, accessHash, refreshHash, err := s.newSessionTokens(now)
	if err != nil {
		return SessionTokens{}, err
	}
	tokens.UserID = userID
	tokens.AuthMethod = "passkey"
	sessionID, err := s.store.CreateSession(ctx, CreateSessionInput{
		AuthMethod:       "passkey",
		UserID:           userID,
		AccessTokenHash:  accessHash,
		RefreshTokenHash: refreshHash,
		UserAgent:        truncate(userAgent, 512),
		IP:               requestIP,
		AccessExpiresAt:  tokens.AccessExpiry,
		RefreshExpiresAt: tokens.RefreshExpiry,
	})
	if err != nil {
		return SessionTokens{}, fmt.Errorf("create session: %w", err)
	}
	tokens.SessionID = sessionID
	return tokens, nil
}

func (s *Service) RefreshSession(ctx context.Context, rawRefreshToken string) (SessionTokens, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 512 {
		return SessionTokens{}, ErrInvalidSession
	}

	now := s.now()
	tokens, accessHash, refreshHash, err := s.newSessionTokens(now)
	if err != nil {
		return SessionTokens{}, err
	}

	session, err := s.store.RotateRefreshToken(ctx, RotateSessionInput{
		OldRefreshTokenHash: hashToken(rawRefreshToken),
		NewRefreshTokenHash: refreshHash,
		NewAccessTokenHash:  accessHash,
		AccessExpiresAt:     tokens.AccessExpiry,
		RefreshExpiresAt:    tokens.RefreshExpiry,
		Now:                 now,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidSession) || errors.Is(err, ErrRefreshReuse) {
			return SessionTokens{}, err
		}
		return SessionTokens{}, fmt.Errorf("rotate refresh token: %w", err)
	}

	tokens.UserID = session.UserID
	tokens.SessionID = session.SessionID
	tokens.AuthMethod = session.AuthMethod
	return tokens, nil
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, rawToken string) (AuthenticatedSession, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > 512 {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	session, err := s.store.FindSessionByAccessTokenHash(ctx, hashToken(rawToken), s.now())
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			return AuthenticatedSession{}, ErrInvalidSession
		}
		return AuthenticatedSession{}, fmt.Errorf("authenticate access token: %w", err)
	}
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, userID string) ([]DeviceSession, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidSession
	}
	sessions, err := s.store.ListSessions(ctx, userID, s.now())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" {
		return ErrInvalidSession
	}
	revoked, err := s.store.RevokeSession(ctx, userID, sessionID, s.now())
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if !revoked {
		return ErrInvalidSession
	}
	return nil
}

func (s *Service) newSessionTokens(now time.Time) (SessionTokens, []byte, []byte, error) {
	accessToken, accessHash, err := newToken()
	if err != nil {
		return SessionTokens{}, nil, nil, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, refreshHash, err := newToken()
	if err != nil {
		return SessionTokens{}, nil, nil, fmt.Errorf("generate refresh token: %w", err)
	}
	return SessionTokens{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		AccessExpiry:  now.Add(s.accessTTL),
		RefreshExpiry: now.Add(s.refreshTTL),
	}, accessHash, refreshHash, nil
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
