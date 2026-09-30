package notifications

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type HTTPHandler struct{ service *Service; auth *identity.Service; logger *slog.Logger }

func NewHTTPHandler(service *Service, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{service: service, auth: auth, logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notifications", h.list)
	mux.HandleFunc("PUT /api/v1/notifications/{notification_id}/read", h.markRead)
	mux.HandleFunc("PUT /api/v1/notifications/read-all", h.markAllRead)
	mux.HandleFunc("PUT /api/v1/notifications/push-subscription", h.subscribePush)
	mux.HandleFunc("DELETE /api/v1/notifications/push-subscription", h.unsubscribePush)
}

func (h *HTTPHandler) list(w http.ResponseWriter,r *http.Request){
	session,ok:=h.authenticate(w,r);if !ok{return}
	limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));items,unread,err:=h.service.List(r.Context(),session.UserID,limit);if err!=nil{h.domain(w,err);return}
	writeJSON(w,http.StatusOK,map[string]any{"items":items,"unread_count":unread})
}

func (h *HTTPHandler) markRead(w http.ResponseWriter,r *http.Request){session,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.MarkRead(r.Context(),session.UserID,r.PathValue("notification_id"));err!=nil{h.domain(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *HTTPHandler) markAllRead(w http.ResponseWriter,r *http.Request){session,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.MarkAllRead(r.Context(),session.UserID);err!=nil{h.domain(w,err);return};w.WriteHeader(http.StatusNoContent)}

func (h *HTTPHandler) subscribePush(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w,r); if !ok { return }
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys struct { P256DH string `json:"p256dh"`; Auth string `json:"auth"` } `json:"keys"`
	}
	if err := decodeJSON(r,&body); err != nil { writeError(w,http.StatusBadRequest,"invalid_request","Invalid request"); return }
	if err := h.service.SubscribePush(r.Context(),PushSubscriptionInput{UserID:session.UserID,Endpoint:body.Endpoint,P256DH:body.Keys.P256DH,Auth:body.Keys.Auth,UserAgent:r.UserAgent()}); err != nil { h.domain(w,err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticate(w,r); if !ok { return }
	var body struct{ Endpoint string `json:"endpoint"` }
	if err := decodeJSON(r,&body); err != nil { writeError(w,http.StatusBadRequest,"invalid_request","Invalid request"); return }
	if err := h.service.UnsubscribePush(r.Context(),session.UserID,body.Endpoint); err != nil { h.domain(w,err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};session,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return session,true}
func (h *HTTPHandler) domain(w http.ResponseWriter,err error){switch{case errors.Is(err,ErrNotFound):writeError(w,http.StatusNotFound,"notification_not_found","Notification not found");case errors.Is(err,ErrForbidden):writeError(w,http.StatusForbidden,"forbidden","Action is not allowed");case errors.Is(err,ErrInvalid):writeError(w,http.StatusBadRequest,"invalid_request","Invalid notification request");default:h.logger.Error("notification request failed","error",err);writeError(w,http.StatusServiceUnavailable,"temporarily_unavailable","Try again later")}}
func decodeJSON(r *http.Request,dst any) error { dec:=json.NewDecoder(io.LimitReader(r.Body,16<<10)); dec.DisallowUnknownFields(); return dec.Decode(dst) }
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func writeJSON(w http.ResponseWriter,status int,body any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(body)}
