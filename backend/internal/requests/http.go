package requests

import (
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
	mux.HandleFunc("GET /api/v1/message-requests", h.list)
	mux.HandleFunc("POST /api/v1/message-requests/{chat_id}/accept", h.accept)
	mux.HandleFunc("POST /api/v1/message-requests/{chat_id}/reject", h.reject)
}

func (h *HTTPHandler) list(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.List(r.Context(), session.UserID, limit)
	if err != nil {
		h.domain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) accept(w http.ResponseWriter, r *http.Request) { h.decide(w, r, true) }
func (h *HTTPHandler) reject(w http.ResponseWriter, r *http.Request) { h.decide(w, r, false) }

func (h *HTTPHandler) decide(w http.ResponseWriter, r *http.Request, accept bool) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var err error
	if accept {
		err = h.service.Accept(r.Context(), session.UserID, r.PathValue("chat_id"))
	} else {
		err = h.service.Reject(r.Context(), session.UserID, r.PathValue("chat_id"))
	}
	if err != nil {
		h.domain(w, err)
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

func (h *HTTPHandler) domain(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "request_not_found", "Message request not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Action is not allowed")
	default:
		h.logger.Error("message request failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
