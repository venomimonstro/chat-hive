package messaging

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type ReactionBatchHTTPHandler struct {
	service *ReactionBatchService
	auth    *identity.Service
	logger  *slog.Logger
}

func NewReactionBatchHTTPHandler(service *ReactionBatchService, auth *identity.Service, logger *slog.Logger) *ReactionBatchHTTPHandler {
	return &ReactionBatchHTTPHandler{service:service,auth:auth,logger:logger}
}

func (h *ReactionBatchHTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/messages/reactions/batch", h.list)
}

func (h *ReactionBatchHTTPHandler) list(w http.ResponseWriter,r *http.Request) {
	session,ok:=h.authenticate(w,r);if !ok{return}
	var body struct{ MessageIDs []string `json:"message_ids"` }
	decoder:=json.NewDecoder(http.MaxBytesReader(w,r.Body,16<<10));decoder.DisallowUnknownFields()
	if err:=decoder.Decode(&body);err!=nil{writeError(w,http.StatusBadRequest,"invalid_request","Invalid request");return}
	items,err:=h.service.List(r.Context(),session.UserID,body.MessageIDs)
	if err!=nil{
		if errors.Is(err,ErrInvalidMessage){writeError(w,http.StatusBadRequest,"invalid_message_ids","Invalid message ids");return}
		h.logger.Error("batch reactions failed","error",err,"user_id",session.UserID)
		writeError(w,http.StatusServiceUnavailable,"temporarily_unavailable","Try again later")
		return
	}
	writeJSON(w,http.StatusOK,map[string]any{"items":items})
}

func (h *ReactionBatchHTTPHandler) authenticate(w http.ResponseWriter,r *http.Request)(identity.AuthenticatedSession,bool){parts:=strings.Fields(r.Header.Get("Authorization"));if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer")||len(parts[1])>512{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};session,err:=h.auth.AuthenticateAccessToken(r.Context(),parts[1]);if err!=nil{writeError(w,http.StatusUnauthorized,"unauthorized","Authentication required");return identity.AuthenticatedSession{},false};return session,true}
