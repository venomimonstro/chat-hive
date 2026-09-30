package messaging

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
	mux.HandleFunc("POST /api/v1/chats/direct", h.createDirect)
	mux.HandleFunc("GET /api/v1/chats", h.listChats)
	mux.HandleFunc("GET /api/v1/chats/{chat_id}/messages", h.listMessages)
	mux.HandleFunc("POST /api/v1/chats/{chat_id}/messages", h.sendMessage)
	mux.HandleFunc("POST /api/v1/chats/{chat_id}/read", h.markRead)
	mux.HandleFunc("PUT /api/v1/messages/{message_id}", h.editMessage)
	mux.HandleFunc("DELETE /api/v1/messages/{message_id}", h.deleteMessage)
	mux.HandleFunc("GET /api/v1/messages/{message_id}/reactions", h.listReactions)
	mux.HandleFunc("PUT /api/v1/messages/{message_id}/reactions/{reaction}", h.addReaction)
	mux.HandleFunc("DELETE /api/v1/messages/{message_id}/reactions/{reaction}", h.removeReaction)
}

func (h *HTTPHandler) createDirect(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	chat, err := h.service.EnsureDirectChat(r.Context(), session.UserID, body.Username)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func (h *HTTPHandler) listChats(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListChats(
		r.Context(), session.UserID, r.URL.Query().Get("before"), parseLimit(r.URL.Query().Get("limit")),
	)
	if err != nil {
		h.logger.Error("list chats failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) listMessages(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListMessages(
		r.Context(), session.UserID, r.PathValue("chat_id"),
		r.URL.Query().Get("before_sequence"), parseLimit(r.URL.Query().Get("limit")),
	)
	if err != nil {
		if errors.Is(err, ErrInvalidMessage) {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "Invalid message cursor")
			return
		}
		if errors.Is(err, ErrChatNotFound) {
			writeError(w, http.StatusNotFound, "chat_not_found", "Chat not found")
			return
		}
		h.logger.Error("list messages failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) sendMessage(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var body struct {
		ClientMessageID string `json:"client_message_id"`
		Text            string `json:"text"`
		ReplyToID       string `json:"reply_to_id"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	message, duplicate, err := h.service.SendText(r.Context(), SendInput{
		UserID: session.UserID, ChatID: r.PathValue("chat_id"),
		ClientMessageID: body.ClientMessageID, Body: body.Text, ReplyToID: body.ReplyToID,
	})
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"message": message, "duplicate": duplicate})
}

func (h *HTTPHandler) editMessage(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	message, err := h.service.EditMessage(r.Context(), session.UserID, r.PathValue("message_id"), body.Text)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, message)
}

func (h *HTTPHandler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteMessage(r.Context(), session.UserID, r.PathValue("message_id")); err != nil {
		h.writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) listReactions(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListReactions(r.Context(), session.UserID, r.PathValue("message_id"))
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) addReaction(w http.ResponseWriter, r *http.Request) {
	h.setReaction(w, r, true)
}

func (h *HTTPHandler) removeReaction(w http.ResponseWriter, r *http.Request) {
	h.setReaction(w, r, false)
}

func (h *HTTPHandler) setReaction(w http.ResponseWriter, r *http.Request, enabled bool) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if err := h.service.SetReaction(
		r.Context(), session.UserID, r.PathValue("message_id"), r.PathValue("reaction"), enabled,
	); err != nil {
		h.writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) markRead(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var body struct {
		Sequence int64 `json:"sequence"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.MarkRead(r.Context(), session.UserID, r.PathValue("chat_id"), body.Sequence); err != nil {
		h.writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
	case errors.Is(err, ErrChatNotFound):
		writeError(w, http.StatusNotFound, "chat_not_found", "Chat not found")
	case errors.Is(err, ErrMessageNotFound):
		writeError(w, http.StatusNotFound, "message_not_found", "Message not found")
	case errors.Is(err, ErrInteractionDenied):
		writeError(w, http.StatusConflict, "interaction_denied", "This action is not available")
	case errors.Is(err, ErrInvalidMessage):
		writeError(w, http.StatusBadRequest, "invalid_message", "Check message data")
	default:
		h.logger.Error("messaging request failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
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

func parseLimit(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	return value
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
