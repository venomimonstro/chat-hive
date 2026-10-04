package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHealthEndpoints(t *testing.T) {
	handler := New(testLogger()).Handler()
	for _, path := range []string{"/health/live", "/health/ready", "/api/v1"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusOK { t.Fatalf("expected 200, got %d", res.Code) }
			if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" { t.Fatalf("security header missing: %q", got) }
			if got := res.Header().Get("X-Frame-Options"); got != "DENY" { t.Fatalf("frame protection missing: %q", got) }
			if got := res.Header().Get("X-Request-ID"); got == "" { t.Fatal("request id missing") }
		}
	}
}

func TestAPIDisablesCaching(t *testing.T) {
	handler := New(testLogger()).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if got := res.Header().Get("Cache-Control"); got != "no-store" { t.Fatalf("expected no-store, got %q", got) }
	if got := res.Header().Get("Pragma"); got != "no-cache" { t.Fatalf("expected no-cache, got %q", got) }
}

func TestCORSPreflightOnlyAllowsConfiguredOrigin(t *testing.T) {
	server := New(testLogger())
	server.SetAllowedOrigin("https://chat.example")
	handler := server.Handler()

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1", nil)
	allowed.Header.Set("Origin", "https://chat.example")
	allowedRes := httptest.NewRecorder()
	handler.ServeHTTP(allowedRes, allowed)
	if allowedRes.Code != http.StatusNoContent { t.Fatalf("expected allowed preflight, got %d", allowedRes.Code) }
	if got := allowedRes.Header().Get("Access-Control-Allow-Origin"); got != "https://chat.example" { t.Fatalf("unexpected allow origin: %q", got) }

	denied := httptest.NewRequest(http.MethodOptions, "/api/v1", nil)
	denied.Header.Set("Origin", "https://evil.example")
	deniedRes := httptest.NewRecorder()
	handler.ServeHTTP(deniedRes, denied)
	if deniedRes.Code != http.StatusForbidden { t.Fatalf("expected denied preflight, got %d", deniedRes.Code) }
}

func TestRateLimiterRejectsBurst(t *testing.T) {
	limiter := NewRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := rateLimitMiddleware(limiter, next)

	var last *httptest.ResponseRecorder
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email/start", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		last = res
	}
	if last == nil || last.Code != http.StatusTooManyRequests { t.Fatalf("expected burst to be rate limited, got %v", last) }
	if last.Header().Get("Retry-After") == "" { t.Fatal("Retry-After header missing") }
}


func TestPressureGateHysteresis(t *testing.T) {
	gate := NewPressureGate(85, 70)
	if gate.Update(84, 100) { t.Fatal("must remain normal below high threshold") }
	if !gate.Update(85, 100) { t.Fatal("must enter degraded mode at high threshold") }
	if !gate.Update(80, 100) { t.Fatal("must stay degraded until low threshold") }
	if gate.Update(70, 100) { t.Fatal("must recover at low threshold") }
}

func TestDeferrableRequestsNeverIncludeMessaging(t *testing.T) {
	deferrable := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/feed"},
		{http.MethodGet, "/api/v1/discovery/people"},
		{http.MethodGet, "/api/v1/search"},
		{http.MethodPost, "/api/v1/media/images"},
		{http.MethodGet, "/api/v1/communities"},
		{http.MethodGet, "/api/v1/channels"},
	}
	for _, tc := range deferrable {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if !isDeferrableRequest(req) { t.Fatalf("expected deferrable: %s %s", tc.method, tc.path) }
	}

	messaging := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/chats"},
		{http.MethodGet, "/api/v1/chats/abc/messages"},
		{http.MethodPost, "/api/v1/chats/abc/messages"},
		{http.MethodPost, "/api/v1/chats/abc/read"},
		{http.MethodPost, "/api/v1/realtime/ticket"},
	}
	for _, tc := range messaging {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if isDeferrableRequest(req) { t.Fatalf("messaging path must never be shed: %s %s", tc.method, tc.path) }
	}
}

func TestLoadSheddingReturnsRetryable503(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := loadSheddingMiddleware(func() bool { return true }, next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/feed", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable { t.Fatalf("expected 503, got %d", res.Code) }
	if nextCalled { t.Fatal("deferrable request reached downstream handler") }
	if got := res.Header().Get("Retry-After"); got != "5" { t.Fatalf("expected Retry-After=5, got %q", got) }
}
