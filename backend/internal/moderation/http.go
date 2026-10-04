package moderation

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type HTTPHandler struct {
	service *Service
	guard   *authhttp.Guard
	logger  *slog.Logger
}

func NewHTTPHandler(service *Service, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{service: service, guard: authhttp.New(auth), logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/reports", h.createReport)
}

func (h *HTTPHandler) createReport(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Category   string `json:"category"`
		Note       string `json:"note"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	report, err := h.service.CreateReport(r.Context(), CreateInput{
		ReporterID: session.UserID,
		TargetType: body.TargetType,
		TargetID: body.TargetID,
		Category: body.Category,
		Note: body.Note,
	})
	if err != nil {
		h.domain(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}

func (h *HTTPHandler) domain(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidReport):
		writeError(w, http.StatusBadRequest, "invalid_report", "Check report data")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "target_not_found", "Content is unavailable")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Action is not allowed")
	default:
		h.logger.Error("create report failed", "error", err)
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
