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
	service         *Service
	guard           *authhttp.Guard
	logger          *slog.Logger
	runtimeSnapshot  func() RuntimeSnapshot
	clientIPResolver func(*http.Request) string
}

func NewHTTPHandler(service *Service, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{service: service, guard: authhttp.New(auth), logger: logger}
}

func (h *HTTPHandler) SetRuntimeSnapshot(provider func() RuntimeSnapshot) {
	h.runtimeSnapshot = provider
}
func (h *HTTPHandler) SetClientIPResolver(resolve func(*http.Request) string) {
	h.clientIPResolver = resolve
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/me", h.me)
	mux.HandleFunc("GET /api/v1/admin/moderation/cases", h.listCases)
	mux.HandleFunc("POST /api/v1/admin/moderation/cases/{case_id}/decision", h.decideCase)
	mux.HandleFunc("GET /api/v1/admin/security/events", h.listSecurityEvents)
	mux.HandleFunc("GET /api/v1/admin/security/alerts", h.listSecurityAlerts)
	mux.HandleFunc("POST /api/v1/admin/security/alerts/{event_id}/ack", h.ackSecurityAlert)
	mux.HandleFunc("GET /api/v1/admin/security/summary", h.securitySummary)
	mux.HandleFunc("GET /api/v1/admin/ops/flags", h.listPlatformFlags)
	mux.HandleFunc("PUT /api/v1/admin/ops/flags/{key}", h.setPlatformFlag)
	mux.HandleFunc("GET /api/v1/admin/ops/runtime", h.runtime)
	mux.HandleFunc("GET /api/v1/admin/product/metrics", h.productMetrics)
	mux.HandleFunc("GET /api/v1/admin/beta/readiness", h.betaReadiness)
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

func (h *HTTPHandler) listSecurityAlerts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "owner")
	if !ok { return }
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := h.service.ListSecurityAlerts(r.Context(), principal, status, limit)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "security_alert_queue_view", "security_alerts", "", status)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) ackSecurityAlert(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "owner")
	if !ok { return }
	eventID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("event_id")), 10, 64)
	if err != nil || eventID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_action", "Check action data")
		return
	}
	var body struct { Note string `json:"note"` }
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.AcknowledgeSecurityAlert(r.Context(), principal, eventID, body.Note); err != nil {
		h.domain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) securitySummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "owner")
	if !ok { return }
	summary, err := h.service.GetSecuritySummary(r.Context(), principal)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "security_summary_view", "security_summary", "", "")
	writeJSON(w, http.StatusOK, summary)
}

func (h *HTTPHandler) listPlatformFlags(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "system_admin", "owner")
	if !ok { return }
	items, err := h.service.ListPlatformFlags(r.Context(), principal)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "platform_feature_flags_view", "feature_flags", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTPHandler) setPlatformFlag(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "system_admin", "owner")
	if !ok { return }
	var body struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	if err := h.service.SetPlatformFlag(r.Context(), principal, r.PathValue("key"), body.Enabled, body.Reason); err != nil {
		h.domain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) runtime(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "security", "system_admin", "owner")
	if !ok { return }
	if h.runtimeSnapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_metrics_unavailable", "Runtime metrics unavailable")
		return
	}
	snapshot := h.runtimeSnapshot()
	h.audit(r, principal, "runtime_metrics_view", "runtime_metrics", "", "")
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *HTTPHandler) betaReadiness(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "owner")
	if !ok { return }
	readiness, err := h.service.GetBetaReadiness(r.Context(), principal)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "beta_readiness_view", "beta_readiness", "", "")
	writeJSON(w, http.StatusOK, readiness)
}

func (h *HTTPHandler) productMetrics(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principal(w, r, "owner")
	if !ok { return }
	days, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("days")))
	metrics, err := h.service.GetProductMetrics(r.Context(), principal, days)
	if err != nil { h.domain(w, err); return }
	h.audit(r, principal, "product_metrics_view", "product_metrics", "", strconv.Itoa(metrics.WindowDays))
	writeJSON(w, http.StatusOK, metrics)
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
	return h.authorizePrincipal(w, r, false, roles...)
}

func (h *HTTPHandler) privilegedPrincipal(w http.ResponseWriter, r *http.Request, roles ...string) (Principal, bool) {
	return h.authorizePrincipal(w, r, true, roles...)
}

func (h *HTTPHandler) authorizePrincipal(w http.ResponseWriter, r *http.Request, requirePasskeyForPrivileged bool, roles ...string) (Principal, bool) {
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
	if requirePasskeyForPrivileged && hasAnyRole(principal, "owner", "security") && session.AuthMethod != "passkey" {
		h.audit(r, principal, "privileged_step_up_required", "session", session.SessionID, session.AuthMethod)
		writeError(w, http.StatusForbidden, "passkey_required", "Passkey sign-in is required for privileged admin access")
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
		SourceIP: h.auditSourceIP(r),
	}
	if err := h.service.Audit(r.Context(), input); err != nil {
		h.logger.Error("admin audit write failed", "error", err, "action", action, "user_id", principal.UserID)
	}
}

func (h *HTTPHandler) auditSourceIP(r *http.Request) string {
	if h.clientIPResolver != nil {
		return h.clientIPResolver(r)
	}
	return remoteIP(r.RemoteAddr)
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
