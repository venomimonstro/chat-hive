package authhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type errAuthenticator struct{ err error }

func (f errAuthenticator) AuthenticateAccessToken(_ context.Context, _ string) (identity.AuthenticatedSession, error) {
	return identity.AuthenticatedSession{}, f.err
}

func TestAuthGuardSeparatesInvalidTokenFromOutage(t *testing.T) {
	for _, tc := range []struct {
		name string
		err error
		status int
	}{
		{"expired", identity.ErrInvalidSession, http.StatusUnauthorized},
		{"database_down", errors.New("db unavailable"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard := New(errAuthenticator{err: tc.err})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
			req.Header.Set("Authorization", "Bearer token")
			res := httptest.NewRecorder()
			_, ok := guard.Required(res, req)
			if ok || res.Code != tc.status {
				t.Fatalf("want rejected %d, got ok=%v status=%d", tc.status, ok, res.Code)
			}
			if res.Header().Get("Cache-Control") != "no-store" { t.Fatal("auth rejection must not be cached") }
			if tc.status == http.StatusServiceUnavailable && res.Header().Get("Retry-After") == "" {
				t.Fatal("outage must be retryable")
			}
		})
	}
}
