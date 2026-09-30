package groups

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type HTTPHandler struct {
	service *Service
	auth *identity.Service
	logger *slog.Logger
}

func NewHTTPHandler(service *Service, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{service: service, auth: auth, logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/groups", h.create)
	mux.HandleFunc("GET /api/v1/groups/{chat_id}", h.get)
	mux.HandleFunc("GET /api/v1/groups/{chat_id}/members", h.members)
	mux.HandleFunc("PUT /api/v1/groups/{chat_id}/members/{username}/role", h.setRole)
	mux.HandleFunc("DELETE /api/v1/groups/{chat_id}/members/{username}", h.removeMember)
	mux.HandleFunc("POST /api/v1/groups/{chat_id}/leave", h.leave)
	mux.HandleFunc("POST /api/v1/groups/{chat_id}/invites", h.createInvite)
	mux.HandleFunc("POST /api/v1/groups/join", h.join)
	mux.HandleFunc("DELETE /api/v1/groups/{chat_id}/invites", h.revokeInvite)
}

func (h *HTTPHandler) create(w http.ResponseWriter, r *http.Request) {
	s,ok:=h.authenticate(w,r);if !ok{return}
	var body struct{ Title string `json:"title"`; Description string `json:"description"` }
	if decodeJSON(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return}
	g,err:=h.service.Create(r.Context(),s.UserID,body.Title,body.Description);if err!=nil{h.domain(w,err);return};writeJSON(w,201,g)
}
func (h *HTTPHandler) get(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};g,err:=h.service.Get(r.Context(),s.UserID,r.PathValue("chat_id"));if err!=nil{h.domain(w,err);return};writeJSON(w,200,g)}
func (h *HTTPHandler) members(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));items,err:=h.service.ListMembers(r.Context(),s.UserID,r.PathValue("chat_id"),limit);if err!=nil{h.domain(w,err);return};writeJSON(w,200,map[string]any{"items":items})}
func (h *HTTPHandler) setRole(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};var body struct{Role string `json:"role"`};if decodeJSON(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return};if err:=h.service.SetRole(r.Context(),s.UserID,r.PathValue("chat_id"),r.PathValue("username"),body.Role);err!=nil{h.domain(w,err);return};w.WriteHeader(204)}
func (h *HTTPHandler) removeMember(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.RemoveMember(r.Context(),s.UserID,r.PathValue("chat_id"),r.PathValue("username"));err!=nil{h.domain(w,err);return};w.WriteHeader(204)}
func (h *HTTPHandler) leave(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.Leave(r.Context(),s.UserID,r.PathValue("chat_id"));err!=nil{h.domain(w,err);return};w.WriteHeader(204)}
func (h *HTTPHandler) createInvite(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};var body struct{TTLHours int `json:"ttl_hours"`;MaxUses int `json:"max_uses"`};if decodeJSON(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return};ttl:=time.Duration(body.TTLHours)*time.Hour;token,err:=h.service.CreateInvite(r.Context(),s.UserID,r.PathValue("chat_id"),ttl,body.MaxUses);if err!=nil{h.domain(w,err);return};writeJSON(w,201,map[string]string{"token":token})}
func (h *HTTPHandler) join(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};var body struct{Token string `json:"token"`};if decodeJSON(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return};g,err:=h.service.JoinByInvite(r.Context(),s.UserID,body.Token);if err!=nil{h.domain(w,err);return};writeJSON(w,200,g)}
func (h *HTTPHandler) revokeInvite(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};var body struct{Token string `json:"token"`};if decodeJSON(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return};if err:=h.service.RevokeInvite(r.Context(),s.UserID,r.PathValue("chat_id"),body.Token);err!=nil{h.domain(w,err);return};w.WriteHeader(204)}

func (h *HTTPHandler) authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};s,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return s,true}
func (h *HTTPHandler) domain(w http.ResponseWriter,err error){switch{case errors.Is(err,ErrInvalidGroup):writeError(w,400,"invalid_group","Check group data");case errors.Is(err,ErrNotFound):writeError(w,404,"group_not_found","Group not found");case errors.Is(err,ErrForbidden):writeError(w,403,"forbidden","Action is not allowed");case errors.Is(err,ErrInviteInvalid):writeError(w,404,"invite_invalid","Invite is invalid or expired");default:h.logger.Error("group request failed","error",err);writeError(w,503,"temporarily_unavailable","Try again later")}}
func decodeJSON(w http.ResponseWriter,r *http.Request,d any)error{r.Body=http.MaxBytesReader(w,r.Body,16<<10);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();return dec.Decode(d)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func writeJSON(w http.ResponseWriter,status int,body any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(body)}
