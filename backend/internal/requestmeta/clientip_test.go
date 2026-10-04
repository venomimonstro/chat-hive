package requestmeta

import (
	"net/http/httptest"
	"testing"
)

func TestResolverIgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	resolver, err := NewClientIPResolver("172.31.238.2/32")
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest("GET", "https://chat.example/api/v1", nil)
	req.RemoteAddr = "203.0.113.9:43120"
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	if got := resolver.Resolve(req); got != "203.0.113.9" {
		t.Fatalf("expected direct peer IP, got %q", got)
	}
}

func TestResolverUsesRightmostUntrustedForwardedAddress(t *testing.T) {
	resolver, err := NewClientIPResolver("172.31.238.2/32")
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest("GET", "https://chat.example/api/v1", nil)
	req.RemoteAddr = "172.31.238.2:43120"
	req.Header.Set("X-Forwarded-For", "192.0.2.123, 203.0.113.44")
	if got := resolver.Resolve(req); got != "203.0.113.44" {
		t.Fatalf("expected actual client IP, got %q", got)
	}
}

func TestResolverDoesNotTrustAnotherEdgeContainer(t *testing.T) {
	resolver, err := NewClientIPResolver("172.31.238.2/32")
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest("GET", "https://chat.example/api/v1", nil)
	req.RemoteAddr = "172.31.238.4:43120"
	req.Header.Set("X-Forwarded-For", "198.51.100.5")
	if got := resolver.Resolve(req); got != "172.31.238.4" {
		t.Fatalf("expected untrusted web container IP, got %q", got)
	}
}
