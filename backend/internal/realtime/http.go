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
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type HTTPHandler struct {
	auth          *identity.Service
	hub           *Hub
	tickets       *TicketManager
	logger        *slog.Logger
	originPattern string
}

func NewHTTPHandler(auth *identity.Service, hub *Hub, logger *slog.Logger, allowedOrigin string) *HTTPHandler {
	pattern := ""
	if parsed, err := url.Parse(strings.TrimSpace(allowedOrigin)); err == nil { pattern = parsed.Host }
	return &HTTPHandler{auth:auth,hub:hub,tickets:NewTicketManager(),logger:logger,originPattern:pattern}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/realtime/ticket", h.issueTicket)
	mux.HandleFunc("GET /api/v1/realtime", h.connect)
}

func (h *HTTPHandler) issueTicket(w http.ResponseWriter,r *http.Request) {
	session,ok:=h.authenticate(w,r);if !ok{return}
	ticket,expires,err:=h.tickets.Issue(session.UserID)
	if err!=nil{h.logger.Error("realtime ticket issue failed","error",err);writeError(w,http.StatusServiceUnavailable,"temporarily_unavailable","Try again later");return}
	writeJSON(w,http.StatusCreated,map[string]any{"ticket":ticket,"expires_at":expires,"expires_in":int(time.Until(expires).Seconds())})
}

func (h *HTTPHandler) connect(w http.ResponseWriter,r *http.Request) {
	userID,ok:=h.tickets.Consume(strings.TrimSpace(r.URL.Query().Get("ticket")))
	if !ok { writeError(w,http.StatusUnauthorized,"invalid_ticket","Realtime ticket is invalid or expired"); return }

	options:=&websocket.AcceptOptions{CompressionMode:websocket.CompressionDisabled}
	if h.originPattern!="" { options.OriginPatterns=[]string{h.originPattern} }
	conn,err:=websocket.Accept(w,r,options)
	if err!=nil { h.logger.Warn("websocket accept failed","error",err); return }
	defer conn.CloseNow()
	conn.SetReadLimit(4<<10)

	ctx,cancel:=context.WithCancel(context.Background())
	defer cancel()
	client:=&Client{UserID:userID,Send:make(chan Event,64)}
	h.hub.Register(client)
	connectedAt:=time.Now()
	h.logger.Info("realtime_connected","connections",h.hub.Metrics().Connections)
	defer func(){
		h.hub.Unregister(client)
		metrics:=h.hub.Metrics()
		h.logger.Info("realtime_disconnected","duration_ms",time.Since(connectedAt).Milliseconds(),"connections",metrics.Connections,"published",metrics.Published,"dropped",metrics.Dropped)
	}()

	if err:=wsjson.Write(ctx,conn,Event{Type:"ready",OccurredAt:time.Now().UTC().Format(time.RFC3339Nano)});err!=nil{return}
	writerDone:=make(chan struct{})
	go func(){ defer close(writerDone); h.writeLoop(ctx,conn,client) }()

	for {
		var command struct{ Type string `json:"type"` }
		readCtx,readCancel:=context.WithTimeout(ctx,90*time.Second)
		err:=wsjson.Read(readCtx,conn,&command)
		readCancel()
		if err!=nil{return}
		switch strings.ToLower(strings.TrimSpace(command.Type)) {
		case "ping":
			select{case client.Send<-Event{Type:"pong",OccurredAt:time.Now().UTC().Format(time.RFC3339Nano)}:default:{ h.logger.Warn("realtime_client_queue_full","connections",h.hub.Metrics().Connections) }}
		default:
			_ = conn.Close(websocket.StatusPolicyViolation,"unsupported realtime command")
			return
		}
		select{case <-writerDone:return;default:}
	}
}

func (h *HTTPHandler) writeLoop(ctx context.Context,conn *websocket.Conn,client *Client){
	for {
		select{
		case <-ctx.Done():return
		case event,ok:=<-client.Send:
			if !ok{return}
			writeCtx,cancel:=context.WithTimeout(ctx,5*time.Second)
			err:=wsjson.Write(writeCtx,conn,event)
			cancel()
			if err!=nil{return}
		}
	}
}

func (h *HTTPHandler) authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};session,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return session,true}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func writeJSON(w http.ResponseWriter,status int,body any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(body)}
