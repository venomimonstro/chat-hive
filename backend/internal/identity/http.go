package identity

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPHandler struct {
	service      *Service
	yandex       *YandexOAuth
	logger       *slog.Logger
	secureCookie bool
}

func NewHTTPHandler(service *Service, logger *slog.Logger, secureCookie bool) *HTTPHandler {
	return &HTTPHandler{service: service, logger: logger, secureCookie: secureCookie}
}

func (h *HTTPHandler) SetYandexOAuth(yandex *YandexOAuth) { h.yandex = yandex }

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/email/start", h.startEmail)
	mux.HandleFunc("POST /api/v1/auth/email/complete", h.completeEmail)
	mux.HandleFunc("GET /api/v1/auth/yandex/start", h.startYandex)
	mux.HandleFunc("GET /api/v1/auth/yandex/callback", h.completeYandex)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refreshSession)
	mux.HandleFunc("GET /api/v1/auth/session", h.currentSession)
	mux.HandleFunc("GET /api/v1/auth/sessions", h.listSessions)
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

	h.writeSessionTokens(w, tokens)
}

func (h *HTTPHandler) startYandex(w http.ResponseWriter, r *http.Request) {
	if h.yandex == nil || !h.yandex.Enabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "yandex_not_configured", "Yandex ID is not configured")
		return
	}
	authorizationURL, err := h.yandex.Start(r.Context(), clientIP(r))
	if err != nil {
		h.logger.Error("start yandex oauth failed", "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (h *HTTPHandler) completeYandex(w http.ResponseWriter, r *http.Request) {
	if h.yandex == nil || !h.yandex.Enabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "yandex_not_configured", "Yandex ID is not configured")
		return
	}
	base := h.yandex.WebCompleteURL()
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		h.redirectOAuthResult(w, r, base, "yandex_denied")
		return
	}
	tokens, err := h.yandex.Complete(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"), r.UserAgent(), clientIP(r))
	if err != nil {
		if !errors.Is(err, ErrOAuthState) {
			h.logger.Error("complete yandex oauth failed", "error", err)
		}
		h.redirectOAuthResult(w, r, base, "oauth_failed")
		return
	}
	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	h.redirectOAuthResult(w, r, base, "")
}

func (h *HTTPHandler) redirectOAuthResult(w http.ResponseWriter, r *http.Request, base, errorCode string) {
	target, err := url.Parse(base)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "oauth_redirect_invalid", "OAuth redirect is not configured")
		return
	}
	if errorCode != "" {
		query := target.Query()
		query.Set("error", errorCode)
		target.RawQuery = query.Encode()
	}
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

func (h *HTTPHandler) refreshSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("chat_refresh")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		h.clearRefreshCookie(w)
		writeAPIError(w, http.StatusUnauthorized, "session_expired", "Session expired")
		return
	}

	tokens, err := h.service.RefreshSession(r.Context(), cookie.Value)
	if err != nil {
		h.clearRefreshCookie(w)
		if errors.Is(err, ErrRefreshReuse) {
			h.logger.Warn("refresh token reuse detected", "ip", clientIP(r))
			writeAPIError(w, http.StatusUnauthorized, "session_revoked", "Session revoked")
			return
		}
		if !errors.Is(err, ErrInvalidSession) {
			h.logger.Error("refresh session failed", "error", err)
		}
		writeAPIError(w, http.StatusUnauthorized, "session_expired", "Session expired")
		return
	}

	h.writeSessionTokens(w, tokens)
}

func (h *HTTPHandler) currentSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *HTTPHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	current, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListSessions(r.Context(), current.UserID)
	if err != nil {
		h.logger.Error("list sessions failed", "error", err, "user_id", current.UserID)
		writeAPIError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "current_session_id": current.SessionID})
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

func (h *HTTPHandler) writeSessionTokens(w http.ResponseWriter, tokens SessionTokens) {
	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":           tokens.UserID,
		"session_id":        tokens.SessionID,
		"access_token":      tokens.AccessToken,
		"access_expires_at": tokens.AccessExpiry,
	})
}

func (h *HTTPHandler) setRefreshCookie(w http.ResponseWriter, token string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chat_refresh",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
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
		SameSite: http.SameSiteStrictMode,
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
