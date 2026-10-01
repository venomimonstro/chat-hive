package groups

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type ModerationHTTPHandler struct {
	service *ModerationService
	guard   *authhttp.Guard
	logger  *slog.Logger
}

func NewModerationHTTPHandler(service *ModerationService, auth *identity.Service, logger *slog.Logger) *ModerationHTTPHandler {
	return &ModerationHTTPHandler{service: service, guard: authhttp.New(auth), logger: logger}
}

func (h *ModerationHTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /api/v1/groups/{chat_id}/messages/{message_id}", h.deleteMessage)
}

func (h *ModerationHTTPHandler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	err := h.service.DeleteMessage(r.Context(), session.UserID, r.PathValue("chat_id"), r.PathValue("message_id"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Moderator permission required")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "message_not_found", "Message not found")
	case errors.Is(err, ErrInvalidGroup):
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
	default:
		h.logger.Error("group moderator delete failed", "error", err, "chat_id", r.PathValue("chat_id"))
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}
