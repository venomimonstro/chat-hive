package identity

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	challengeHash  []byte
	email          string
	challengeUsed  bool
	count          int
	session        CreateSessionInput
	sessionID      string
	sessionRevoked bool
	refreshHistory [][]byte
}

func (f *fakeStore) CountRecentChallenges(context.Context, string, string, time.Time) (int, error) {
	return f.count, nil
}
func (f *fakeStore) CreateLoginChallenge(_ context.Context, email string, tokenHash []byte, _ string, _ time.Time) error {
	f.email = email
	f.challengeHash = append([]byte(nil), tokenHash...)
	return nil
}
func (f *fakeStore) ConsumeLoginChallenge(_ context.Context, tokenHash []byte, _ time.Time) (string, error) {
	if f.challengeUsed || !bytes.Equal(tokenHash, f.challengeHash) {
		return "", ErrInvalidChallenge
	}
	f.challengeUsed = true
	return "00000000-0000-0000-0000-000000000001", nil
}
func (f *fakeStore) CreateSession(_ context.Context, input CreateSessionInput) (string, error) {
	f.session = input
	f.sessionID = "00000000-0000-0000-0000-000000000002"
	return f.sessionID, nil
}
func (f *fakeStore) FindSessionByAccessTokenHash(_ context.Context, tokenHash []byte, _ time.Time) (AuthenticatedSession, error) {
	if f.sessionRevoked || len(f.session.AccessTokenHash) == 0 || !bytes.Equal(tokenHash, f.session.AccessTokenHash) {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	return AuthenticatedSession{
		UserID:    f.session.UserID,
		SessionID: f.sessionID,
		ExpiresAt: f.session.AccessExpiresAt,
	}, nil
}
func (f *fakeStore) RotateRefreshToken(_ context.Context, input RotateSessionInput) (AuthenticatedSession, error) {
	if f.sessionRevoked {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	if bytes.Equal(input.OldRefreshTokenHash, f.session.RefreshTokenHash) {
		f.refreshHistory = append(f.refreshHistory, append([]byte(nil), f.session.RefreshTokenHash...))
		f.session.RefreshTokenHash = append([]byte(nil), input.NewRefreshTokenHash...)
		f.session.AccessTokenHash = append([]byte(nil), input.NewAccessTokenHash...)
		f.session.AccessExpiresAt = input.AccessExpiresAt
		f.session.RefreshExpiresAt = input.RefreshExpiresAt
		return AuthenticatedSession{
			UserID:    f.session.UserID,
			SessionID: f.sessionID,
			ExpiresAt: input.AccessExpiresAt,
		}, nil
	}
	for _, consumed := range f.refreshHistory {
		if bytes.Equal(input.OldRefreshTokenHash, consumed) {
			f.sessionRevoked = true
			return AuthenticatedSession{}, ErrRefreshReuse
		}
	}
	return AuthenticatedSession{}, ErrInvalidSession
}
func (f *fakeStore) RevokeSession(_ context.Context, userID, sessionID string, _ time.Time) (bool, error) {
	if f.sessionRevoked || userID != f.session.UserID || sessionID != f.sessionID {
		return false, nil
	}
	f.sessionRevoked = true
	return true, nil
}

type fakeSender struct {
	email string
	token string
}

func (f *fakeSender) SendMagicLink(_ context.Context, email, token string) error {
	f.email = email
	f.token = token
	return nil
}

func TestMagicLinkIsHashedAndSingleUse(t *testing.T) {
	store := &fakeStore{}
	sender := &fakeSender{}
	service := NewService(store, sender)
	fixedNow := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if err := service.StartEmailLogin(context.Background(), "  USER@Example.com ", "127.0.0.1"); err != nil {
		t.Fatalf("start login: %v", err)
	}
	if store.email != "user@example.com" || sender.email != "user@example.com" {
		t.Fatalf("email was not normalized: store=%q sender=%q", store.email, sender.email)
	}
	if sender.token == "" || len(store.challengeHash) == 0 {
		t.Fatal("expected token and persisted hash")
	}
	if bytes.Equal([]byte(sender.token), store.challengeHash) {
		t.Fatal("raw magic-link token must not be stored")
	}

	tokens, err := service.CompleteEmailLogin(context.Background(), sender.token, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("complete login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.AccessToken == tokens.RefreshToken {
		t.Fatal("expected independent access and refresh tokens")
	}
	if len(store.session.AccessTokenHash) == 0 || len(store.session.RefreshTokenHash) == 0 {
		t.Fatal("session token hashes were not persisted")
	}

	if _, err := service.CompleteEmailLogin(context.Background(), sender.token, "test-agent", "127.0.0.1"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("expected one-time token rejection, got %v", err)
	}
}

func TestAccessSessionCanBeValidatedAndRevoked(t *testing.T) {
	store, service, tokens := authenticatedFixture(t)

	session, err := service.AuthenticateAccessToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}
	if session.UserID != tokens.UserID || session.SessionID != tokens.SessionID {
		t.Fatalf("unexpected authenticated session: %+v", session)
	}

	if err := service.RevokeSession(context.Background(), tokens.UserID, tokens.SessionID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := service.AuthenticateAccessToken(context.Background(), tokens.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("revoked session still authenticated: %v", err)
	}
	if !store.sessionRevoked {
		t.Fatal("store did not revoke session")
	}
}

func TestRefreshRotationRejectsReplayAndRevokesSession(t *testing.T) {
	store, service, original := authenticatedFixture(t)

	rotated, err := service.RefreshSession(context.Background(), original.RefreshToken)
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if rotated.RefreshToken == original.RefreshToken || rotated.AccessToken == original.AccessToken {
		t.Fatal("refresh must rotate both access and refresh credentials")
	}
	if _, err := service.AuthenticateAccessToken(context.Background(), original.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("old access token remained valid after rotation: %v", err)
	}
	if _, err := service.AuthenticateAccessToken(context.Background(), rotated.AccessToken); err != nil {
		t.Fatalf("new access token rejected: %v", err)
	}

	if _, err := service.RefreshSession(context.Background(), original.RefreshToken); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("expected replay detection, got %v", err)
	}
	if !store.sessionRevoked {
		t.Fatal("refresh replay must revoke the session")
	}
	if _, err := service.AuthenticateAccessToken(context.Background(), rotated.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("session remained valid after refresh replay: %v", err)
	}
}

func TestMagicLinkRateLimit(t *testing.T) {
	store := &fakeStore{count: 5}
	service := NewService(store, &fakeSender{})
	if err := service.StartEmailLogin(context.Background(), "user@example.com", "127.0.0.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestInvalidEmailRejected(t *testing.T) {
	service := NewService(&fakeStore{}, &fakeSender{})
	if err := service.StartEmailLogin(context.Background(), "not-an-email", "127.0.0.1"); err == nil {
		t.Fatal("expected invalid email error")
	}
}

func authenticatedFixture(t *testing.T) (*fakeStore, *Service, SessionTokens) {
	t.Helper()
	store := &fakeStore{}
	sender := &fakeSender{}
	service := NewService(store, sender)
	fixedNow := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if err := service.StartEmailLogin(context.Background(), "user@example.com", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	tokens, err := service.CompleteEmailLogin(context.Background(), sender.token, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	return store, service, tokens
}
