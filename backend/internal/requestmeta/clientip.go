package requestmeta

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type ClientIPResolver struct {
	trusted []netip.Prefix
}

func NewClientIPResolver(rawCIDRs string) (*ClientIPResolver, error) {
	resolver := &ClientIPResolver{}
	for _, raw := range strings.Split(rawCIDRs, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", raw, err)
		}
		resolver.trusted = append(resolver.trusted, prefix.Masked())
	}
	return resolver, nil
}

func (r *ClientIPResolver) Resolve(req *http.Request) string {
	peer := parseRemoteAddr(req.RemoteAddr)
	if !peer.IsValid() {
		return "unknown"
	}
	if !r.isTrusted(peer) {
		return peer.String()
	}

	forwarded := strings.Split(req.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(forwarded[i]))
		if err != nil {
			continue
		}
		candidate = candidate.Unmap()
		if !r.isTrusted(candidate) {
			return candidate.String()
		}
	}
	return peer.String()
}

func (r *ClientIPResolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range r.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseRemoteAddr(value string) netip.Addr {
	value = strings.TrimSpace(value)
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}
