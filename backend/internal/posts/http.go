package posts

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
	mux.HandleFunc("POST /api/v1/posts", h.create)
	mux.HandleFunc("GET /api/v1/posts/{post_id}", h.get)
	mux.HandleFunc("PUT /api/v1/posts/{post_id}", h.edit)
	mux.HandleFunc("DELETE /api/v1/posts/{post_id}", h.delete)
	mux.HandleFunc("GET /api/v1/profiles/{username}/posts", h.listByAuthor)
	mux.HandleFunc("GET /api/v1/posts/{post_id}/replies", h.listReplies)
	mux.HandleFunc("POST /api/v1/posts/{post_id}/replies", h.addReply)
	mux.HandleFunc("PUT /api/v1/posts/{post_id}/reactions/{reaction}", h.addReaction)
	mux.HandleFunc("DELETE /api/v1/posts/{post_id}/reactions/{reaction}", h.removeReaction)
	mux.HandleFunc("PUT /api/v1/posts/{post_id}/saved", h.save)
	mux.HandleFunc("DELETE /api/v1/posts/{post_id}/saved", h.unsave)
}

func (h *HTTPHandler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct {
		Kind       string   `json:"kind"`
		Body       string   `json:"body"`
		Visibility string   `json:"visibility"`
		MediaIDs   []string `json:"media_ids"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	post, err := h.service.Create(r.Context(), CreateInput{AuthorID: session.UserID, Kind: body.Kind, Body: body.Body, Visibility: body.Visibility, MediaIDs: body.MediaIDs})
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusCreated, post)
}

func (h *HTTPHandler) get(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	post, err := h.service.Get(r.Context(), viewerID, r.PathValue("post_id"))
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusOK, post)
}

func (h *HTTPHandler) listByAuthor(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	var before time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil { writeError(w, http.StatusBadRequest, "invalid_cursor", "Invalid cursor"); return }
		before = parsed
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.ListByAuthor(r.Context(), viewerID, r.PathValue("username"), before, limit)
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) edit(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Body string `json:"body"` }
	if err := decodeJSON(w, r, &body); err != nil { writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request"); return }
	post, err := h.service.Edit(r.Context(), session.UserID, r.PathValue("post_id"), body.Body)
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusOK, post)
}

func (h *HTTPHandler) delete(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	if err := h.service.Delete(r.Context(), session.UserID, r.PathValue("post_id")); err != nil { h.domainError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) addReply(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	var body struct { Body string `json:"body"` }
	if err := decodeJSON(w, r, &body); err != nil { writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request"); return }
	reply, err := h.service.AddReply(r.Context(), session.UserID, r.PathValue("post_id"), body.Body)
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusCreated, reply)
}

func (h *HTTPHandler) listReplies(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := h.optionalViewer(w, r)
	if !ok { return }
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.ListReplies(r.Context(), viewerID, r.PathValue("post_id"), limit)
	if err != nil { h.domainError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) addReaction(w http.ResponseWriter, r *http.Request) { h.setReaction(w, r, true) }
func (h *HTTPHandler) removeReaction(w http.ResponseWriter, r *http.Request) { h.setReaction(w, r, false) }

func (h *HTTPHandler) setReaction(w http.ResponseWriter, r *http.Request, enabled bool) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	if err := h.service.SetReaction(r.Context(), session.UserID, r.PathValue("post_id"), r.PathValue("reaction"), enabled); err != nil { h.domainError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) save(w http.ResponseWriter, r *http.Request) { h.setSaved(w, r, true) }
func (h *HTTPHandler) unsave(w http.ResponseWriter, r *http.Request) { h.setSaved(w, r, false) }

func (h *HTTPHandler) setSaved(w http.ResponseWriter, r *http.Request, saved bool) {
	session, ok := h.authenticate(w, r)
	if !ok { return }
	if err := h.service.SetSaved(r.Context(), session.UserID, r.PathValue("post_id"), saved); err != nil { h.domainError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) optionalViewer(w http.ResponseWriter, r *http.Request) (string, bool) {
	if strings.TrimSpace(r.Header.Get("Authorization")) == "" { return "", true }
	session, ok := h.authenticate(w, r)
	if !ok { return "", false }
	return session.UserID, true
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

func (h *HTTPHandler) domainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidPost):
		writeError(w, http.StatusBadRequest, "invalid_post", "Check post data")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "post_not_found", "Post not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Action is not allowed")
	default:
		h.logger.Error("post request failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
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
