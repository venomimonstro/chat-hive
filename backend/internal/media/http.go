package media

import (
	"encoding/json"
	"errors"
	"io"
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
	mux.HandleFunc("POST /api/v1/media/images", h.uploadImage)
	mux.HandleFunc("GET /api/v1/media/{media_id}/content", h.content)
}

func (h *HTTPHandler) uploadImage(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+1024)
	if err := r.ParseMultipartForm(MaxUploadBytes + 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image", "Image is invalid or too large")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image", "Image file is required")
		return
	}
	defer file.Close()
	object, err := h.service.UploadImage(r.Context(), session.UserID, file)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidImage):
			writeError(w, http.StatusBadRequest, "invalid_image", "Only safe JPEG/PNG images within limits are accepted")
		case errors.Is(err, ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Upload is not allowed")
		default:
			h.logger.Error("image upload failed", "error", err, "user_id", session.UserID)
			writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		}
		return
	}
	writeJSON(w, http.StatusCreated, object)
}

func (h *HTTPHandler) content(w http.ResponseWriter, r *http.Request) {
	viewerID := ""
	session, authenticated, ok := h.guard.Optional(w, r)
	if !ok {
		return
	}
	if authenticated {
		viewerID = session.UserID
	}
	file, object, err := h.service.Open(r.Context(), viewerID, r.PathValue("media_id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrForbidden):
			http.NotFound(w, r)
		default:
			h.logger.Error("media read failed", "error", err, "media_id", r.PathValue("media_id"))
			writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		}
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", object.MimeType)
	w.Header().Set("Content-Length", int64String(object.ByteSize))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}

func int64String(value int64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
