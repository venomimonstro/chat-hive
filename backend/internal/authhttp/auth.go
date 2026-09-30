package authhttp

import (
	"context"
	"net/http"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type Authenticator interface {
	AuthenticateAccessToken(ctx context.Context, rawToken string) (identity.AuthenticatedSession, error)
}

type Guard struct{ auth Authenticator }

func New(auth Authenticator) *Guard { return &Guard{auth: auth} }

func (g *Guard) Required(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedSession, bool) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		WriteUnauthorized(w)
		return identity.AuthenticatedSession{}, false
	}
	session, err := g.auth.AuthenticateAccessToken(r.Context(), token)
	if err != nil {
		WriteUnauthorized(w)
		return identity.AuthenticatedSession{}, false
	}
	return session, true
}

// Optional returns an empty session when there is no Authorization header.
// If a malformed/expired Authorization header is supplied it fails closed with 401.
func (g *Guard) Optional(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedSession, bool, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return identity.AuthenticatedSession{}, false, true
	}
	session, ok := g.Required(w, r)
	if !ok {
		return identity.AuthenticatedSession{}, false, false
	}
	return session, true, true
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) == 0 || len(parts[1]) > 512 {
		return "", false
	}
	return parts[1], true
}

func WriteUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("{\"error\":{\"code\":\"unauthorized\",\"message\":\"Authentication required\"}}"))
}
