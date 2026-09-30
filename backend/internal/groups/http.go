package groups

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
	mux.HandleFunc("POST /api/v1/groups", h.create)
	mux.HandleFunc("GET /api/v1/groups/{chat_id}", h.get)
	mux.HandleFunc("GET /api/v1/groups/{chat_id}/members", h.members)
	mux.HandleFunc("PUT /api/v1/groups/{chat_id}/members/{username}/role", h.setRole)
	mux.HandleFunc("POST /api/v1/groups/{chat_id}/owner", h.transferOwnership)
	mux.HandleFunc("DELETE /api/v1/groups/{chat_id}/members/{username}", h.removeMember)
	mux.HandleFunc("POST /api/v1/groups/{chat_id}/leave", h.leave)
	mux.HandleFunc("POST /api/v1/groups/{chat_id}/invites", h.createInvite)
	mux.HandleFunc("POST /api/v1/groups/join", h.join)
	mux.HandleFunc("DELETE /api/v1/groups/{chat_id}/invites", h.revokeInvite)
}

func (h *HTTPHandler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	group, err := h.service.Create(r.Context(), session.UserID, body.Title, body.Description)
	if err != nil { h.domain(w, err); return }
	writeJSON(w, http.StatusCreated, group)
}

func (h *HTTPHandler) get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	group, err := h.service.Get(r.Context(), session.UserID, r.PathValue("chat_id"))
	if err != nil { h.domain(w, err); return }
	writeJSON(w, http.StatusOK, group)
}

func (h *HTTPHandler) members(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.ListMembers(r.Context(), session.UserID, r.PathValue("chat_id"), limit)
	if err != nil { h.domain(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) setRole(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Role string `json:"role"` }
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.SetRole(r.Context(), session.UserID, r.PathValue("chat_id"), r.PathValue("username"), body.Role); err != nil {
		h.domain(w, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) transferOwnership(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Username string `json:"username"` }
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.TransferOwnership(r.Context(), session.UserID, r.PathValue("chat_id"), body.Username); err != nil {
		h.domain(w, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	if err := h.service.RemoveMember(r.Context(), session.UserID, r.PathValue("chat_id"), r.PathValue("username")); err != nil {
		h.domain(w, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) leave(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	if err := h.service.Leave(r.Context(), session.UserID, r.PathValue("chat_id")); err != nil {
		h.domain(w, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) createInvite(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct {
		TTLHours int `json:"ttl_hours"`
		MaxUses  int `json:"max_uses"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	token, err := h.service.CreateInvite(r.Context(), session.UserID, r.PathValue("chat_id"), time.Duration(body.TTLHours)*time.Hour, body.MaxUses)
	if err != nil { h.domain(w, err); return }
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

func (h *HTTPHandler) join(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Token string `json:"token"` }
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	group, err := h.service.JoinByInvite(r.Context(), session.UserID, body.Token)
	if err != nil { h.domain(w, err); return }
	writeJSON(w, http.StatusOK, group)
}

func (h *HTTPHandler) revokeInvite(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Token string `json:"token"` }
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.RevokeInvite(r.Context(), session.UserID, r.PathValue("chat_id"), body.Token); err != nil {
		h.domain(w, err); return
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
	case errors.Is(err, ErrInvalidGroup):
		writeError(w, http.StatusBadRequest, "invalid_group", "Check group data")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "group_not_found", "Group not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Action is not allowed")
	case errors.Is(err, ErrInviteInvalid):
		writeError(w, http.StatusNotFound, "invite_invalid", "Invite is invalid or expired")
	default:
		h.logger.Error("group request failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
