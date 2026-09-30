package realtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	Type       string `json:"type"`
	ChatID     string `json:"chat_id,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	SenderID   string `json:"sender_id,omitempty"`
	Sequence   int64  `json:"sequence,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

type HubMetrics struct {
	Connections int64 `json:"connections"`
	Published   int64 `json:"published"`
	Dropped     int64 `json:"dropped"`
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[string]map[*Client]struct{}
	connections atomic.Int64
	published   atomic.Int64
	dropped     atomic.Int64
}

type Client struct {
	UserID string
	Send   chan Event
}

func NewHub() *Hub { return &Hub{clients: make(map[string]map[*Client]struct{})} }

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.UserID] == nil { h.clients[client.UserID] = make(map[*Client]struct{}) }
	if _, exists := h.clients[client.UserID][client]; !exists {
		h.clients[client.UserID][client] = struct{}{}
		h.connections.Add(1)
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	bucket := h.clients[client.UserID]
	if bucket == nil { return }
	if _, ok := bucket[client]; ok {
		delete(bucket, client)
		close(client.Send)
		h.connections.Add(-1)
	}
	if len(bucket) == 0 { delete(h.clients, client.UserID) }
}

func (h *Hub) Publish(userID string, event Event) {
	h.mu.RLock()
	bucket := h.clients[userID]
	for client := range bucket {
		select {
		case client.Send <- event:
			h.published.Add(1)
		default:
			// Never let a slow browser block fan-out. The HTTP sync path remains authoritative.
			h.dropped.Add(1)
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) Metrics() HubMetrics {
	return HubMetrics{
		Connections: h.connections.Load(),
		Published:   h.published.Load(),
		Dropped:     h.dropped.Load(),
	}
}

type ticket struct {
	UserID    string
	ExpiresAt time.Time
}

type TicketManager struct {
	mu      sync.Mutex
	tickets map[string]ticket
	ttl     time.Duration
}

func NewTicketManager() *TicketManager { return &TicketManager{tickets: make(map[string]ticket), ttl: 30 * time.Second} }

func (m *TicketManager) Issue(userID string) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { return "", time.Time{}, err }
	value := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(m.ttl)
	hash := sha256.Sum256([]byte(value))
	m.mu.Lock()
	m.cleanupLocked(time.Now().UTC())
	m.tickets[hex.EncodeToString(hash[:])] = ticket{UserID:userID,ExpiresAt:expires}
	m.mu.Unlock()
	return value, expires, nil
}

func (m *TicketManager) Consume(value string) (string, bool) {
	if len(value) < 32 || len(value) > 256 { return "", false }
	hash := sha256.Sum256([]byte(value))
	key := hex.EncodeToString(hash[:])
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.tickets[key]
	delete(m.tickets, key)
	m.cleanupLocked(now)
	if !ok || !entry.ExpiresAt.After(now) { return "", false }
	return entry.UserID, true
}

func (m *TicketManager) cleanupLocked(now time.Time) {
	for key, entry := range m.tickets { if !entry.ExpiresAt.After(now) { delete(m.tickets,key) } }
}

func waitForContext(ctx context.Context) { <-ctx.Done() }
