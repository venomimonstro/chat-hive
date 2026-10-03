package growth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

var ErrInvalid = errors.New("invalid growth event")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Record(ctx context.Context, eventName, acquisitionID, userID, objectType, objectID, source string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO growth_events(event_name,acquisition_id,user_id,object_type,object_id,source)
		VALUES($1,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,$4,$5,$6)`,
		eventName, acquisitionID, userID, objectType, objectID, source)
	return err
}

type HTTPHandler struct {
	store  *Store
	guard  *authhttp.Guard
	logger *slog.Logger
}

func NewHTTPHandler(store *Store, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{store: store, guard: authhttp.New(auth), logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/growth/events", h.record)
}

func (h *HTTPHandler) record(w http.ResponseWriter, r *http.Request) {
	session, present, ok := h.guard.Optional(w, r)
	if !ok { return }
	userID := ""
	if present { userID = session.UserID }

	var body struct {
		EventName     string `json:"event_name"`
		AcquisitionID string `json:"acquisition_id"`
		ObjectType    string `json:"object_type"`
		ObjectID      string `json:"object_id"`
		Source        string `json:"source"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	body.EventName = strings.ToLower(strings.TrimSpace(body.EventName))
	body.AcquisitionID = strings.TrimSpace(body.AcquisitionID)
	body.ObjectType = strings.ToLower(strings.TrimSpace(body.ObjectType))
	body.ObjectID = strings.TrimSpace(body.ObjectID)
	body.Source = normalizeSource(body.Source)
	if !validEvent(body.EventName) || !validObjectType(body.ObjectType) || len(body.ObjectID) > 128 ||
		(body.AcquisitionID != "" && !looksLikeUUID(body.AcquisitionID)) {
		writeError(w, http.StatusBadRequest, "invalid_event", "Invalid analytics event")
		return
	}
	if err := h.store.Record(r.Context(), body.EventName, body.AcquisitionID, userID, body.ObjectType, body.ObjectID, body.Source); err != nil {
		h.logger.Error("growth event write failed", "error", err, "event_name", body.EventName)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validEvent(value string) bool {
	switch value {
	case "public_view", "login_started", "signup_completed", "follow", "community_join", "channel_subscribe", "invite_join", "first_post", "first_message":
		return true
	default:
		return false
	}
}

func validObjectType(value string) bool {
	switch value {
	case "", "profile", "post", "community", "channel", "group_invite":
		return true
	default:
		return false
	}
}

func normalizeSource(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" { return "" }
	if len(raw) > 256 { raw = raw[:256] }
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Hostname() != "" {
		host := strings.ToLower(parsed.Hostname())
		if len(host) > 64 { host = host[:64] }
		return host
	}
	raw = strings.ToLower(raw)
	if len(raw) > 64 { raw = raw[:64] }
	return raw
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 { return false }
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 { if char != '-' { return false }; continue }
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) { return false }
	}
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
