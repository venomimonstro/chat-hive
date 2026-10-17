package accountdata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
    "os"
    "strconv"
	"strings"

	"github.com/venomimonstro/chat-hive/backend/internal/authhttp"
	"github.com/venomimonstro/chat-hive/backend/internal/identity"
)

type DataStore interface {
	WriteExport(ctx context.Context, userID string, w io.Writer) error
	DeleteAccount(ctx context.Context, userID string) error
}

type HTTPHandler struct {
	store  DataStore
	guard  *authhttp.Guard
	logger *slog.Logger
}

func NewHTTPHandler(store DataStore, auth *identity.Service, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{store: store, guard: authhttp.New(auth), logger: logger}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me/data-export", h.export)
	mux.HandleFunc("DELETE /api/v1/me/account", h.deleteAccount)
}

const maxAccountExportBytes int64 = 256 << 20 // bounded staging disk use

type cappedExportWriter struct {
    w io.Writer
    remaining int64
}
func (w *cappedExportWriter) Write(payload []byte) (int,error) {
    if int64(len(payload)) > w.remaining {
        return 0, errors.New("account export exceeds allowed size")
    }
    n, err := w.w.Write(payload)
    w.remaining -= int64(n)
    return n,err
}

// Stage the complete JSON response with owner-only filesystem permissions.
// This prevents partially generated exports being delivered with HTTP 200.
func stageAccountExport(ctx context.Context, store DataStore, userID string) (*os.File,int64,error) {
    file, err := os.CreateTemp("", "chat-account-export-*.json")
    if err != nil {return nil,0,err}
    clean := func() { _ = file.Close(); _ = os.Remove(file.Name()) }
    if err := file.Chmod(0600); err != nil {clean();return nil,0,err}
    limited := &cappedExportWriter{w:file,remaining:maxAccountExportBytes}
    if err := store.WriteExport(ctx,userID,limited); err != nil {clean();return nil,0,err}
    info, err := file.Stat()
    if err != nil {clean();return nil,0,err}
    if _, err := file.Seek(0,io.SeekStart); err != nil {clean();return nil,0,err}
    return file,info.Size(),nil
}

func (h *HTTPHandler) export(w http.ResponseWriter, r *http.Request) {
    session, ok := h.guard.Required(w,r)
    if !ok {return}
    file, size, err := stageAccountExport(r.Context(),h.store,session.UserID)
    if err != nil {
        h.logger.Error("account export generation failed","error",err,"user_id",session.UserID)
        w.Header().Set("Retry-After","30")
        writeAccountError(w,http.StatusServiceUnavailable,"export_unavailable","Export is temporarily unavailable")
        return
    }
    defer func(){ _ = file.Close(); _ = os.Remove(file.Name()) }()
    w.Header().Set("Content-Type","application/json; charset=utf-8")
    w.Header().Set("Content-Disposition",`attachment; filename="chat-data-export.json"`)
    w.Header().Set("Content-Length",strconv.FormatInt(size,10))
    w.Header().Set("Cache-Control","no-store")
    w.Header().Set("X-Content-Type-Options","nosniff")
    if _, err := io.Copy(w,file); err != nil {
        h.logger.Warn("account export delivery interrupted","error",err,"user_id",session.UserID)
    }
}

func (h *HTTPHandler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	session, ok := h.guard.Required(w, r)
	if !ok {
		return
	}
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Confirmation) != "DELETE MY CHAT ACCOUNT" {
        writeAccountError(w,http.StatusBadRequest,"confirmation_required","Explicit account deletion confirmation is required")
        return
    }
    var trailing any
    if err := decoder.Decode(&trailing); err != io.EOF {
        writeAccountError(w,http.StatusBadRequest,"invalid_request","Only one JSON object allowed")
        return
    }
	if err := h.store.DeleteAccount(r.Context(), session.UserID); err != nil {
		h.logger.Error("account deletion failed", "error", err, "user_id", session.UserID)
		writeAccountError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "Try again later")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func writeAccountError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
