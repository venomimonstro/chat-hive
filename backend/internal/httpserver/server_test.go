package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(logger).Handler()

	for _, path := range []string{"/health/live", "/health/ready", "/api/v1"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", res.Code)
			}
			if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("security header missing: %q", got)
			}
			if got := res.Header().Get("X-Request-ID"); got == "" {
				t.Fatal("request id missing")
			}
		})
	}
}
