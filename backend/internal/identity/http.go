package identity

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPHandler struct {
	service      *Service
	logger       *slog.Logger
	secureCookie bool
}

func NewHTTPHandler(service *Service, logger *slog.Logger, secureCookie bool) *HTTPHandler {
	return &HTTPHandler{service: service, logger: logger, secureCookie: secureCookie}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/email/start", h.startEmail)
	mux.HandleFunc("POST /api/v1/auth/email/complete", h.completeEmail)
	mux.HandleFunc("GET /api/v1/auth/session", h.currentSession)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{session_id}", h.revokeSession)
}

func (h *HTTPHandler) startEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}

	err := h.service.StartEmailLogin(r.Context(), body.Email, clientIP(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, ErrRateLimited):
		w.Header().Set("Retry-After", "900")
		writeAPIError(w, http.StatusTooManyRequests, "rate_limited", "Try again later")
	case strings.Contains(err.Error(), "invalid email"):
		writeAPIError(w, http.StatusBadRequest, "invalid_email", "Enter a valid email")
	default:
		h.logger.Error("start email login failed", "error", err)
		// Deliberately avoid exposing whether an account exists or internal mail/storage state.
		writeAPIError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
	}
}

func (h *HTTPHandler) completeEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}

	tokens, err := h.service.CompleteEmailLogin(r.Context(), body.Token, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrInvalidChallenge) {
			writeAPIError(w, http.StatusUnauthorized, "invalid_login_link", "Login link is invalid or expired")
			return
		}
		h.logger.Error("complete email login failed", "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}

	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":           tokens.UserID,
		"session_id":        tokens.SessionID,
		"access_token":      tokens.AccessToken,
		"access_expires_at": tokens.AccessExpiry,
	})
}

func (h *HTTPHandler) currentSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *HTTPHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	current, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	targetSessionID := strings.TrimSpace(r.PathValue("session_id"))
	if targetSessionID == "" || len(targetSessionID) > 64 {
		writeAPIError(w, http.StatusBadRequest, "invalid_session", "Invalid session")
		return
	}
	if err := h.service.RevokeSession(r.Context(), current.UserID, targetSessionID); err != nil {
		if errors.Is(err, ErrInvalidSession) {
			// Do not reveal another user's session IDs.
			writeAPIError(w, http.StatusNotFound, "session_not_found", "Session not found")
			return
		}
		h.logger.Error("revoke session failed", "error", err, "user_id", current.UserID)
		writeAPIError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	if targetSessionID == current.SessionID {
		h.clearRefreshCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) authenticateRequest(w http.ResponseWriter, r *http.Request) (AuthenticatedSession, bool) {
	rawToken, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return AuthenticatedSession{}, false
	}
	session, err := h.service.AuthenticateAccessToken(r.Context(), rawToken)
	if err != nil {
		if !errors.Is(err, ErrInvalidSession) {
			h.logger.Error("access session validation failed", "error", err)
		}
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return AuthenticatedSession{}, false
	}
	return session, true
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 512 {
		return "", false
	}
	return parts[1], true
}

func (h *HTTPHandler) setRefreshCookie(w http.ResponseWriter, token string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chat_refresh",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiry,
		MaxAge:   maxAgeUntil(expiry),
	})
}

func (h *HTTPHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chat_refresh",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}

func maxAgeUntil(expiry time.Time) int {
	seconds := int(time.Until(expiry).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	// Do not trust X-Forwarded-For here. A trusted edge-proxy middleware will normalize it later.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
