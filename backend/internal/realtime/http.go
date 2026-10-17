package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

// A realtime ticket authenticates an upgrade, not an unlimited lifetime.
// Force fresh authentication regularly so revoked or expired browser sessions
// cannot retain a WebSocket forever (a dedicated session-revocation feed
// would be preferable for immediate server-side disconnects).
const maxRealtimeConnectionAge = 5 * time.Minute

type HTTPHandler struct {
	guard         *authhttp.Guard
	hub           *Hub
	tickets       *TicketManager
	logger        *slog.Logger
	originPattern string
	originScheme string
}

func NewHTTPHandler(auth *identity.Service, hub *Hub, logger *slog.Logger, allowedOrigin string) *HTTPHandler {
	pattern, scheme := "", ""
	if parsed, err := url.Parse(strings.TrimSpace(allowedOrigin)); err == nil && parsed.Host != "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http") {
		pattern, scheme = parsed.Host, parsed.Scheme
	}
	return &HTTPHandler{guard: authhttp.New(auth), hub: hub, tickets: NewTicketManager(), logger: logger,
		originPattern: pattern, originScheme: scheme}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/realtime/ticket", h.issueTicket)
	mux.HandleFunc("GET /api/v1/realtime", h.connect)
}

func (h *HTTPHandler) issueTicket(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	ticket, expires, err := h.tickets.Issue(session.UserID)
	if err != nil {
		h.logger.Error("realtime ticket issue failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ticket": ticket,
		"expires_at": expires,
		"expires_in": int(time.Until(expires).Seconds()),
	})
}

func (h *HTTPHandler) connect(w http.ResponseWriter, r *http.Request) {
	// Reject foreign browser origins before consuming one-use tickets.
    origin := strings.TrimSpace(r.Header.Get("Origin"))
    // CHAT core supports browser WebSocket sessions only. An absent Origin
    // must never bypass the allowlist via native/scripted clients possessing
    // a leaked or misdirected short-lived websocket ticket.
    parsed, err := url.Parse(origin)
    if origin == "" || err != nil || parsed.Host == "" ||
        h.originPattern == "" || h.originScheme == "" ||
        !strings.EqualFold(parsed.Host,h.originPattern) ||
        parsed.Scheme != h.originScheme || parsed.User != nil ||
        parsed.Path != "" || parsed.RawQuery != "" ||
        parsed.Fragment != "" || parsed.Opaque != "" {
        writeError(w,http.StatusForbidden,"invalid_origin","Origin is not allowed")
        return
    }
	userID, ok := h.tickets.Consume(strings.TrimSpace(r.URL.Query().Get("ticket")))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_ticket", "Realtime ticket is invalid or expired")
		return
	}

	options := &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled}
	if h.originPattern != "" {
		options.OriginPatterns = []string{h.originPattern}
	}
	conn, err := websocket.Accept(w, r, options)
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4 << 10)

    ctx, cancel := context.WithTimeout(context.Background(),maxRealtimeConnectionAge)
    defer cancel()
    client := &Client{UserID:userID,Send:make(chan Event,64)}
	h.hub.Register(client)
	connectedAt := time.Now()
	h.logger.Info("realtime_connected", "connections", h.hub.Metrics().Connections)
	defer func() {
		h.hub.Unregister(client)
		metrics := h.hub.Metrics()
		h.logger.Info(
			"realtime_disconnected",
			"duration_ms", time.Since(connectedAt).Milliseconds(),
			"connections", metrics.Connections,
			"published", metrics.Published,
			"dropped", metrics.Dropped,
		)
	}()

	if err := wsjson.Write(ctx, conn, Event{Type: "ready", OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		return
	}
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		h.writeLoop(ctx, conn, client)
	}()

	for {
		var command struct {
			Type string `json:"type"`
		}
		readCtx, readCancel := context.WithTimeout(ctx, 90*time.Second)
		err := wsjson.Read(readCtx, conn, &command)
		readCancel()
		if err != nil {
			return
		}
		switch strings.ToLower(strings.TrimSpace(command.Type)) {
		case "ping":
			select {
			case client.Send <- Event{Type: "pong", OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}:
			default:
				h.logger.Warn("realtime_client_queue_full", "connections", h.hub.Metrics().Connections)
			}
		default:
			_ = conn.Close(websocket.StatusPolicyViolation, "unsupported realtime command")
			return
		}
		select {
		case <-writerDone:
			return
		default:
		}
	}
}

func (h *HTTPHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client.Send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := wsjson.Write(writeCtx, conn, event)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
