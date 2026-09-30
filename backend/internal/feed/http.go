package feed

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	mux.HandleFunc("GET /api/v1/feed", h.list)
	mux.HandleFunc("PUT /api/v1/feed/{post_id}/feedback", h.feedback)
}

func (h *HTTPHandler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var before time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "Invalid cursor")
			return
		}
		before = parsed
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.List(r.Context(), session.UserID, r.URL.Query().Get("mode"), before, limit)
	if err != nil {
		if errors.Is(err, ErrInvalidMode) {
			writeError(w, http.StatusBadRequest, "invalid_mode", "Invalid feed mode")
			return
		}
		h.logger.Error("feed list failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) feedback(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct{ Signal string `json:"signal"` }
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.SetFeedback(r.Context(), session.UserID, r.PathValue("post_id"), body.Signal); err != nil {
		if errors.Is(err, ErrInvalidFeedback) {
			writeError(w, http.StatusBadRequest, "invalid_feedback", "Invalid feedback")
			return
		}
		h.logger.Error("feed feedback failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
