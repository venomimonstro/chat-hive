package notifications

import (
	"encoding/json"
	"errors"
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
}

func (h *HTTPHandler) list(w http.ResponseWriter,r *http.Request){
	session,ok:=h.authenticate(w,r);if !ok{return}
	limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));items,unread,err:=h.service.List(r.Context(),session.UserID,limit);if err!=nil{h.domain(w,err);return}
	writeJSON(w,200,map[string]any{"items":items,"unread_count":unread})
}

func (h *HTTPHandler) markRead(w http.ResponseWriter,r *http.Request){session,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.MarkRead(r.Context(),session.UserID,r.PathValue("notification_id"));err!=nil{h.domain(w,err);return};w.WriteHeader(204)}
func (h *HTTPHandler) markAllRead(w http.ResponseWriter,r *http.Request){session,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.MarkAllRead(r.Context(),session.UserID);err!=nil{h.domain(w,err);return};w.WriteHeader(204)}

func (h *HTTPHandler) authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};session,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return session,true}
func (h *HTTPHandler) domain(w http.ResponseWriter,err error){switch{case errors.Is(err,ErrNotFound):writeError(w,404,"notification_not_found","Notification not found");case errors.Is(err,ErrForbidden):writeError(w,403,"forbidden","Action is not allowed");default:h.logger.Error("notification request failed","error",err);writeError(w,503,"temporarily_unavailable","Try again later")}}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func writeJSON(w http.ResponseWriter,status int,body any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(body)}
