package identity

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	challengeHash []byte
	email         string
	challengeUsed bool
	count         int
	session       CreateSessionInput
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
	return "00000000-0000-0000-0000-000000000002", nil
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
