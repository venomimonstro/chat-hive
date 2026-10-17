package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type unavailableRefreshStore struct{ *fakeStore }

func (s *unavailableRefreshStore) RotateRefreshToken(_ context.Context, _ RotateSessionInput) (AuthenticatedSession, error) {
	return AuthenticatedSession{}, errors.New("database unavailable")
}

func TestRefreshOutageRetainsBrowserCookie(t *testing.T) {
	service := NewService(&unavailableRefreshStore{fakeStore: &fakeStore{}}, &fakeSender{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHTTPHandler(service, logger, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "chat_refresh", Value: "valid-but-db-down"})
	res := httptest.NewRecorder()
	handler.refreshSession(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("outage must be retryable 503, got %d: %s", res.Code, res.Body.String())
	}
	if res.Header().Get("Set-Cookie") != "" {
		t.Fatal("temporary infrastructure error must not clear the refresh cookie")
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("temporary outage must provide Retry-After")
	}
}

func TestInvalidRefreshStillClearsBrowserCookie(t *testing.T) {
	service := NewService(&fakeStore{sessionRevoked: true}, &fakeSender{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHTTPHandler(service, logger, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "chat_refresh", Value: "revoked"})
	res := httptest.NewRecorder()
	handler.refreshSession(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("invalid credentials must return 401, got %d", res.Code)
	}
	if res.Header().Get("Set-Cookie") == "" {
		t.Fatal("invalid session must clear the refresh cookie")
	}
}
