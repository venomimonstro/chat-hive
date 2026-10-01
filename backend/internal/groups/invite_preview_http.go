package groups

import (
	"errors"
	"log/slog"
	"net/http"
)

type InvitePreviewHTTPHandler struct {
	service *InvitePreviewService
	logger  *slog.Logger
}

func NewInvitePreviewHTTPHandler(service *InvitePreviewService, logger *slog.Logger) *InvitePreviewHTTPHandler {
	return &InvitePreviewHTTPHandler{service: service, logger: logger}
}

func (h *InvitePreviewHTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/groups/invites/{token}/preview", h.preview)
}

func (h *InvitePreviewHTTPHandler) preview(w http.ResponseWriter, r *http.Request) {
	preview, err := h.service.Preview(r.Context(), r.PathValue("token"))
	if err != nil {
		if errors.Is(err, ErrInviteInvalid) {
			writeError(w, http.StatusNotFound, "invite_invalid", "Invite is invalid or expired")
			return
		}
		h.logger.Error("invite preview failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=30")
	writeJSON(w, http.StatusOK, preview)
}
