package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	logger        *slog.Logger
	mux           *http.ServeMux
	readiness     func(context.Context) error
	allowedOrigin string
	limiter       *RateLimiter
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func New(logger *slog.Logger) *Server {
	s := &Server{logger: logger, mux: http.NewServeMux(), limiter: NewRateLimiter()}
	s.routes()
	return s
}

func (s *Server) Register(register func(*http.ServeMux)) { register(s.mux) }
func (s *Server) SetReadiness(check func(context.Context) error) { s.readiness = check }
func (s *Server) SetAllowedOrigin(origin string) { s.allowedOrigin = strings.TrimRight(strings.TrimSpace(origin), "/") }

func (s *Server) Handler() http.Handler {
	var handler http.Handler = s.mux
	handler = bodyLimitMiddleware(handler)
	handler = rateLimitMiddleware(s.limiter, handler)
	handler = corsMiddleware(s.allowedOrigin, handler)
	handler = securityHeaders(handler)
	handler = requestTelemetryMiddleware(s.logger, handler)
	handler = requestIDMiddleware(handler)
	handler = recoverMiddleware(s.logger, handler)
	return handler
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "chat-api"})
	})
	s.mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if s.readiness != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := s.readiness(ctx); err != nil {
				s.logger.Warn("readiness check failed", "error", err)
				writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Service: "chat-api"})
				return
			}
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ready", Service: "chat-api"})
	})
	s.mux.HandleFunc("GET /api/v1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "chat-api", "version": "v1"})
	})
}

func corsMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin == allowedOrigin {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			// Feature handlers may enforce stricter limits. This is only the outer safety ceiling.
			r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = time.Now().UTC().Format("20060102T150405.000000000")
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 { w.status = status }
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int,error) {
	if w.status == 0 { w.status = http.StatusOK }
	n,err:=w.ResponseWriter.Write(body)
	w.bytes += n
	return n,err
}

func requestTelemetryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WebSocket libraries need the original ResponseWriter interfaces for the HTTP upgrade.
		// Log the attempt without wrapping the writer; connection-level telemetry lives in realtime.
		if r.URL.Path == "/api/v1/realtime" {
			started:=time.Now()
			next.ServeHTTP(w,r)
			logger.Info("websocket_request","method",r.Method,"path",r.URL.Path,"duration_ms",time.Since(started).Milliseconds(),"request_id",w.Header().Get("X-Request-ID"))
			return
		}
		started:=time.Now()
		wrapped:=&statusWriter{ResponseWriter:w}
		next.ServeHTTP(wrapped,r)
		status:=wrapped.status
		if status==0 { status=http.StatusOK }
		logger.Info("http_request",
			"method",r.Method,
			"path",r.URL.Path,
			"status",status,
			"bytes",wrapped.bytes,
			"duration_ms",time.Since(started).Milliseconds(),
			"request_id",w.Header().Get("X-Request-ID"),
		)
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "error", recovered, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]string{"code": "internal_error", "message": "Internal server error"},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
