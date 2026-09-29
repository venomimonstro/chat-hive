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

	http.SetCookie(w, &http.Cookie{
		Name:     "chat_refresh",
		Value:    tokens.RefreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.RefreshExpiry,
		MaxAge:   int(time.Until(tokens.RefreshExpiry).Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":          tokens.UserID,
		"session_id":       tokens.SessionID,
		"access_token":     tokens.AccessToken,
		"access_expires_at": tokens.AccessExpiry,
	})
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
