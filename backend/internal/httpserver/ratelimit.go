package httpserver

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	tokens     float64
	last       time.Time
	lastAccess time.Time
}

type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*rateBucket
	lastSweep time.Time
}

func NewRateLimiter() *RateLimiter {
	now := time.Now()
	return &RateLimiter{buckets: make(map[string]*rateBucket), lastSweep: now}
}

func (l *RateLimiter) Allow(key string, ratePerSecond, burst float64) (bool, int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) > 5*time.Minute || len(l.buckets) > 50_000 {
		cutoff := now.Add(-15 * time.Minute)
		for candidate, bucket := range l.buckets {
			if bucket.lastAccess.Before(cutoff) { delete(l.buckets, candidate) }
		}
		l.lastSweep = now
	}
	bucket := l.buckets[key]
	if bucket == nil {
		bucket = &rateBucket{tokens: burst, last: now, lastAccess: now}
		l.buckets[key] = bucket
	}
	elapsed := now.Sub(bucket.last).Seconds()
	bucket.tokens += elapsed * ratePerSecond
	if bucket.tokens > burst { bucket.tokens = burst }
	bucket.last = now
	bucket.lastAccess = now
	if bucket.tokens >= 1 {
		bucket.tokens -= 1
		return true, 0
	}
	wait := int((1-bucket.tokens)/ratePerSecond) + 1
	if wait < 1 { wait = 1 }
	return false, wait
}

func rateLimitMiddleware(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" { next.ServeHTTP(w,r); return }
		ip := remoteIP(r)
		rate, burst, bucketName := 20.0, 40.0, "general"
		switch {
		case strings.Contains(r.URL.Path,"/auth/"):
			rate, burst, bucketName = 0.5, 8, "auth"
		case strings.Contains(r.URL.Path,"/media/images") && r.Method == http.MethodPost:
			rate, burst, bucketName = 0.25, 5, "upload"
		case strings.Contains(r.URL.Path,"/messages") && r.Method == http.MethodPost:
			rate, burst, bucketName = 3, 12, "message"
		case r.URL.Path == "/api/v1/realtime/ticket":
			rate, burst, bucketName = 1, 5, "realtime-ticket"
		}
		allowed,retryAfter:=limiter.Allow(bucketName+":"+ip,rate,burst)
		if !allowed {
			w.Header().Set("Retry-After",strconv.Itoa(retryAfter))
			writeJSON(w,http.StatusTooManyRequests,map[string]any{"error":map[string]string{"code":"rate_limited","message":"Too many requests"}})
			return
		}
		next.ServeHTTP(w,r)
	})
}

func remoteIP(r *http.Request) string {
	host,_,err:=net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err==nil && host!="" { return host }
	if parsed:=net.ParseIP(strings.TrimSpace(r.RemoteAddr));parsed!=nil { return parsed.String() }
	return "unknown"
}
