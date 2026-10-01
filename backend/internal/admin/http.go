package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

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
	mux.HandleFunc("GET /api/v1/admin/me", h.me)
	mux.HandleFunc("GET /api/v1/admin/moderation/cases", h.listCases)
	mux.HandleFunc("POST /api/v1/admin/moderation/cases/{case_id}/decision", h.decideCase)
	mux.HandleFunc("GET /api/v1/admin/security/events", h.listSecurityEvents)
	mux.HandleFunc("GET /api/v1/admin/security/summary", h.securitySummary)
}

func (h *HTTPHandler) me(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "support", "moderator", "senior_moderator", "security", "legal", "system_admin", "owner")
	if !ok { return }
	h.audit(r, principal, "admin_me_view", "admin", principal.UserID, "")
	writeJSON(w, http.StatusOK, principal)
}

func (h *HTTPHandler) listCases(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "moderator", "senior_moderator", "security", "legal", "owner")
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := h.service.ListCases(r.Context(), principal, limit)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "moderation_queue_view", "moderation_queue", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) listSecurityEvents(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "owner")
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	severity := strings.TrimSpace(r.URL.Query().Get("severity"))
	items, err := h.service.ListSecurityEvents(r.Context(), principal, severity, limit)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "security_event_queue_view", "security_events", "", severity)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) securitySummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "owner")
	if !ok { return }
	summary, err := h.service.GetSecuritySummary(r.Context(), principal)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "security_summary_view", "security_summary", "", "")
	writeJSON(w, http.StatusOK, summary)
}

func (h *HTTPHandler) decideCase(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "senior_moderator", "security", "legal", "owner")
	if !ok { return }
	var body struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.ResolveCase(r.Context(), principal, r.PathValue("case_id"), body.Decision, body.Reason); err != nil {
		h.domain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) principal(w http.ResponseWriter, r *http.Request, roles ...string) (Principal, bool) {
	session, ok := h.guard.Required(w, r)
	if !ok { return Principal{}, false }
	principal, err := h.service.Authorize(r.Context(), session.UserID, roles...)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeError(w, http.StatusNotFound, "not_found", "Not found")
			return Principal{}, false
		}
		h.logger.Error("admin authorization failed", "error", err, "user_id", session.UserID)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return Principal{}, false
	}
	return principal, true
}

func (h *HTTPHandler) audit(r *http.Request, principal Principal, action, targetType, targetID, reason string) {
	input := AuditInput{
		ActorUserID: principal.UserID,
		ActorRole: strongestRole(principal),
		Action: action,
		TargetType: targetType,
		TargetID: targetID,
		Reason: reason,
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
		SourceIP: remoteIP(r.RemoteAddr),
	}
	if err := h.service.Audit(r.Context(), input); err != nil {
		h.logger.Error("admin audit write failed", "error", err, "action", action, "user_id", principal.UserID)
	}
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil { return host }
	return strings.TrimSpace(remoteAddr)
}

func (h *HTTPHandler) domain(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusNotFound, "not_found", "Not found")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "case_not_found", "Case not found")
	case errors.Is(err, ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_action", "Check action data")
	default:
		h.logger.Error("admin request failed", "error", err)
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
