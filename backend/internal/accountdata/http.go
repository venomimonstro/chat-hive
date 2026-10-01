package accountdata

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type DataStore interface {
	WriteExport(ctx context.Context, userID string, w io.Writer) error
	DeleteAccount(ctx context.Context, userID string) error
}

type HTTPHandler struct {
	store  DataStore
	guard  *authhttp.Guard
	logger *slog.Logger
}

func NewHTTPHandler(store DataStore, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{store: store, guard: authhttp.New(auth), logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me/data-export", h.export)
	mux.HandleFunc("DELETE /api/v1/me/account", h.deleteAccount)
}

func (h *HTTPHandler) export(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="chat-data-export.json"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := h.store.WriteExport(r.Context(), session.UserID, w); err != nil {
		// Headers may already be committed by a streaming export, so never append a second JSON body.
		h.logger.Error("account data export failed", "error", err, "user_id", session.UserID)
	}
}

func (h *HTTPHandler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Confirmation) != "DELETE MY CHAT ACCOUNT" {
		writeAccountError(w, http.StatusBadRequest, "confirmation_required", "Explicit account deletion confirmation is required")
		return
	}
	if err := h.store.DeleteAccount(r.Context(), session.UserID); err != nil {
		h.logger.Error("account deletion failed", "error", err, "user_id", session.UserID)
		writeAccountError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func writeAccountError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
