package communities

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
func NewHTTPHandler(service *Service,auth *identity.Service,logger *slog.Logger)*HTTPHandler{return &HTTPHandler{service:service,auth:auth,logger:logger}}
func(h *HTTPHandler)Register(mux *http.ServeMux){
	mux.HandleFunc("POST /api/v1/communities",h.create)
	mux.HandleFunc("GET /api/v1/communities",h.discover)
	mux.HandleFunc("GET /api/v1/communities/{slug}",h.get)
	mux.HandleFunc("POST /api/v1/communities/{slug}/join",h.join)
	mux.HandleFunc("POST /api/v1/communities/{slug}/leave",h.leave)
}
func(h *HTTPHandler)create(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};var body struct{Slug string `json:"slug"`;Title string `json:"title"`;Description string `json:"description"`;Visibility string `json:"visibility"`};if decode(w,r,&body)!=nil{writeError(w,400,"invalid_request","Invalid request");return};item,err:=h.service.Create(r.Context(),CreateInput{OwnerID:s.UserID,Slug:body.Slug,Title:body.Title,Description:body.Description,Visibility:body.Visibility});if err!=nil{h.domain(w,err);return};writeJSON(w,201,item)}
func(h *HTTPHandler)discover(w http.ResponseWriter,r *http.Request){viewerID,ok:=h.optionalViewer(w,r);if !ok{return};limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));items,err:=h.service.Discover(r.Context(),viewerID,limit);if err!=nil{h.domain(w,err);return};writeJSON(w,200,map[string]any{"items":items})}
func(h *HTTPHandler)get(w http.ResponseWriter,r *http.Request){viewerID,ok:=h.optionalViewer(w,r);if !ok{return};item,err:=h.service.Get(r.Context(),viewerID,r.PathValue("slug"));if err!=nil{h.domain(w,err);return};writeJSON(w,200,item)}
func(h *HTTPHandler)join(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};item,err:=h.service.Join(r.Context(),s.UserID,r.PathValue("slug"));if err!=nil{h.domain(w,err);return};writeJSON(w,200,item)}
func(h *HTTPHandler)leave(w http.ResponseWriter,r *http.Request){s,ok:=h.authenticate(w,r);if !ok{return};if err:=h.service.Leave(r.Context(),s.UserID,r.PathValue("slug"));err!=nil{h.domain(w,err);return};w.WriteHeader(204)}
func(h *HTTPHandler)optionalViewer(w http.ResponseWriter,r *http.Request)(string,bool){if strings.TrimSpace(r.Header.Get("Authorization"))==""{return "",true};s,ok:=h.authenticate(w,r);if !ok{return "",false};return s.UserID,true}
func(h *HTTPHandler)authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};s,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,401,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return s,true}
func(h *HTTPHandler)domain(w http.ResponseWriter,err error){switch{case errors.Is(err,ErrInvalid):writeError(w,400,"invalid_community","Check community data");case errors.Is(err,ErrSlugTaken):writeError(w,409,"slug_taken","Address is already used");case errors.Is(err,ErrNotFound):writeError(w,404,"community_not_found","Community not found");case errors.Is(err,ErrForbidden):writeError(w,403,"forbidden","Action is not allowed");default:h.logger.Error("community request failed","error",err);writeError(w,503,"temporarily_unavailable","Try again later")}}
func decode(w http.ResponseWriter,r *http.Request,d any)error{r.Body=http.MaxBytesReader(w,r.Body,32<<10);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();return dec.Decode(d)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func writeJSON(w http.ResponseWriter,status int,body any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(body)}
