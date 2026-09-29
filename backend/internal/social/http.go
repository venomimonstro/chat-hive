package social

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type HTTPHandler struct {
	service *Service
	auth    *identity.Service
	logger  *slog.Logger
}

func NewHTTPHandler(service *Service, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{service: service, auth: auth, logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/profiles/{username}", h.getProfile)
	mux.HandleFunc("POST /api/v1/profiles/{username}/follow", h.follow)
	mux.HandleFunc("DELETE /api/v1/profiles/{username}/follow", h.unfollow)
	mux.HandleFunc("POST /api/v1/profiles/{username}/block", h.block)
	mux.HandleFunc("DELETE /api/v1/profiles/{username}/block", h.unblock)
}

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	viewerID := ""
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		session, ok := h.authenticate(w, r)
		if !ok { return }
		viewerID = session.UserID
	}
	profile, err := h.service.GetProfile(r.Context(), viewerID, r.PathValue("username"))
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
			return
		}
		h.logger.Error("get public profile failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *HTTPHandler) follow(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.Follow, http.StatusNoContent)
}
func (h *HTTPHandler) unfollow(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.Unfollow, http.StatusNoContent)
}
func (h *HTTPHandler) block(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.Block, http.StatusNoContent)
}
func (h *HTTPHandler) unblock(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.Unblock, http.StatusNoContent)
}

func (h *HTTPHandler) mutate(w http.ResponseWriter, r *http.Request, action func(r.Context, string, string) error, success int) {
	// Kept as separate explicit handlers until shared authenticated HTTP middleware lands.
}

func (h *HTTPHandler) authenticate(w http.ResponseWriter, r *http.Request) (identity.AuthenticatedSession, bool) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 512 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return identity.AuthenticatedSession{}, false
	}
	session, err := h.auth.AuthenticateAccessToken(r.Context(), parts[1])
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return identity.AuthenticatedSession{}, false
	}
	return session, true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
