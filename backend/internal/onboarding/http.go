package onboarding

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
	mux.HandleFunc("GET /api/v1/onboarding/interests", h.listInterests)
	mux.HandleFunc("GET /api/v1/me/profile", h.getProfile)
	mux.HandleFunc("PUT /api/v1/me/onboarding", h.complete)
}

func (h *HTTPHandler) listInterests(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListInterests(r.Context())
	if err != nil {
		h.logger.Error("list interests failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	profile, err := h.service.GetProfile(r.Context(), session.UserID)
	if err != nil {
		h.logger.Error("get profile failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *HTTPHandler) complete(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	var body struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Bio         string   `json:"bio"`
		Interests   []string `json:"interests"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	profile, err := h.service.Complete(r.Context(), CompleteInput{
		UserID:      session.UserID,
		Username:    body.Username,
		DisplayName: body.DisplayName,
		Bio:         body.Bio,
		Interests:   body.Interests,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, profile)
	case errors.Is(err, ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username_taken", "Username is already taken")
	case errors.Is(err, ErrInvalidProfile):
		writeError(w, http.StatusBadRequest, "invalid_profile", "Check profile fields and choose 3 to 12 interests")
	default:
		h.logger.Error("complete onboarding failed", "error", err, "user_id", session.UserID)
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
