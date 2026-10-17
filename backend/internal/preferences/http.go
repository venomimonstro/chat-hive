package preferences

import (
 "encoding/json"
 "io"
 "log/slog"
 "net/http"

 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/venomimonstro/chat-hive/backend/internal/authhttp"
 "github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type Handler struct {
 pool *pgxpool.Pool
 guard *authhttp.Guard
 logger *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, auth *identity.Service, logger *slog.Logger) *Handler {
 return &Handler{pool:pool,guard:authhttp.New(auth),logger:logger}
}
func validLanguage(language string) bool { return language=="ru" || language=="en" }
func (h *Handler) Register(mux *http.ServeMux) {
 mux.HandleFunc("GET /api/v1/me/preferences", h.get)
 mux.HandleFunc("PUT /api/v1/me/preferences", h.set)
}
func (h *Handler) get(w http.ResponseWriter,r *http.Request) {
 session,ok:=h.guard.Required(w,r);if !ok{return}
 var language string
 err:=h.pool.QueryRow(r.Context(),`SELECT COALESCE((SELECT language FROM user_preferences WHERE user_id=$1::uuid),'ru')`,session.UserID).Scan(&language)
 if err!=nil { h.logger.Error("load preferences", "error",err);writeError(w,http.StatusServiceUnavailable);return }
 writeLanguage(w,language)
}
func (h *Handler) set(w http.ResponseWriter,r *http.Request) {
 session,ok:=h.guard.Required(w,r);if !ok{return}
 var value struct{Language string `json:"language"`}
 r.Body=http.MaxBytesReader(w,r.Body,2048)
 decoder:=json.NewDecoder(r.Body)
 decoder.DisallowUnknownFields()
 if err:=decoder.Decode(&value);err!=nil || !validLanguage(value.Language){
    writeJSON(w,http.StatusBadRequest,map[string]any{"error":map[string]string{"code":"invalid_language","message":"Choose ru or en"}});return
 }
 var trailing any
 if err:=decoder.Decode(&trailing);err != io.EOF {
    writeJSON(w,http.StatusBadRequest,map[string]any{"error":map[string]string{"code":"invalid_request","message":"Only one JSON object allowed"}});return
 }
 _,err:=h.pool.Exec(r.Context(),`INSERT INTO user_preferences(user_id,language) VALUES($1::uuid,$2)
 ON CONFLICT(user_id) DO UPDATE SET language=EXCLUDED.language,updated_at=now()`,session.UserID,value.Language)
 if err!=nil {h.logger.Error("store preferences", "error",err);writeError(w,http.StatusServiceUnavailable);return}
 writeLanguage(w,value.Language)
}
func writeLanguage(w http.ResponseWriter,language string){writeJSON(w,http.StatusOK,map[string]string{"language":language})}
func writeError(w http.ResponseWriter,status int) {writeJSON(w,status,map[string]any{"error":map[string]string{"code":"temporarily_unavailable","message":"Try again later"}})}
func writeJSON(w http.ResponseWriter,status int,data any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(data)}
