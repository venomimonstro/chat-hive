package social

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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
	mux.HandleFunc("GET /api/v1/profiles/{username}/followers", h.followers)
	mux.HandleFunc("GET /api/v1/profiles/{username}/following", h.following)
	mux.HandleFunc("GET /api/v1/discovery/people", h.recommendPeople)
	mux.HandleFunc("POST /api/v1/profiles/{username}/follow", h.follow)
	mux.HandleFunc("DELETE /api/v1/profiles/{username}/follow", h.unfollow)
	mux.HandleFunc("POST /api/v1/profiles/{username}/block", h.block)
	mux.HandleFunc("DELETE /api/v1/profiles/{username}/block", h.unblock)
}

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	profile, err := h.service.GetProfile(r.Context(), viewerID, r.PathValue("username"))
	if err != nil { h.profileError(w, err); return }
	writeJSON(w, http.StatusOK, profile)
}

func (h *HTTPHandler) followers(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.ListFollowers(r.Context(), viewerID, r.PathValue("username"), limit)
	if err != nil { h.profileError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) following(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.ListFollowing(r.Context(), viewerID, r.PathValue("username"), limit)
	if err != nil { h.profileError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) recommendPeople(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w,r)
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.RecommendPeople(r.Context(), session.UserID, limit)
	if err != nil {
		h.logger.Error("people recommendations failed", "error", err, "user_id", session.UserID)
		writeError(w,http.StatusServiceUnavailable,"temporarily_unavailable","Try again later")
		return
	}
	writeJSON(w,http.StatusOK,map[string]any{"items":items})
}

func (h *HTTPHandler) follow(w http.ResponseWriter, r *http.Request) { h.mutate(w, r, h.service.Follow) }
func (h *HTTPHandler) unfollow(w http.ResponseWriter, r *http.Request) { h.mutate(w, r, h.service.Unfollow) }
func (h *HTTPHandler) block(w http.ResponseWriter, r *http.Request) { h.mutate(w, r, h.service.Block) }
func (h *HTTPHandler) unblock(w http.ResponseWriter, r *http.Request) { h.mutate(w, r, h.service.Unblock) }

func (h *HTTPHandler) mutate(w http.ResponseWriter, r *http.Request, action func(context.Context, string, string) error) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	err := action(r.Context(), session.UserID, r.PathValue("username"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
	case errors.Is(err, ErrInteractionDenied):
		writeError(w, http.StatusConflict, "interaction_denied", "This action is not available")
	default:
		h.logger.Error("social graph mutation failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}

func (h *HTTPHandler) optionalViewer(w http.ResponseWriter, r *http.Request) (string, bool) {
	if strings.TrimSpace(r.Header.Get("Authorization")) == "" { return "", true }
	session, ok := h.authenticate(w, r)
	if !ok { return "", false }
	return session.UserID, true
}

func (h *HTTPHandler) profileError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
		return
	}
	h.logger.Error("profile request failed", "error", err)
	writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
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
