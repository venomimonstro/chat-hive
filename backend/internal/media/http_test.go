package media

import (
	"net/http/httptest"
	"testing"
)

func TestMediaResponsesCannotLeakThroughBrowserCache(t *testing.T) {
	w := httptest.NewRecorder()
	setMediaResponseHeaders(w, StoredObject{MimeType: "image/png", ByteSize: 123})
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("private media must never be cached, got %q", got)
	}
	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected no-cache pragma, got %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "123" {
		t.Fatalf("incorrect content length: %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("missing nosniff: %q", got)
	}
}
