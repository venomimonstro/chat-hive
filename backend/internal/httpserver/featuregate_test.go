package httpserver

import (
  "context"
  "errors"
  "io"
  "log/slog"
  "net/http"
  "net/http/httptest"
  "testing"
)

type simulatedGate struct{ enabled bool; err error }
func (s simulatedGate) Enabled(context.Context,string)(bool,error){return s.enabled,s.err}

func TestFeatureKillSwitchUnavailableFailsClosed(t *testing.T) {
    hits:=0
    downstream:=http.HandlerFunc(func(w http.ResponseWriter,_ *http.Request){hits++;w.WriteHeader(200)})
    logger:=slog.New(slog.NewTextHandler(io.Discard,nil))
    gate:=operationalGateMiddleware(simulatedGate{err:errors.New("unavailable")},logger,downstream)
    request:=httptest.NewRequest("POST","/api/v1/posts",nil)
    recorder:=httptest.NewRecorder()
    gate.ServeHTTP(recorder,request)
    if recorder.Code!=503 || hits!=0 {t.Fatalf("feature executed after gate outage: status=%d hits=%d",recorder.Code,hits)}
    if recorder.Header().Get("Retry-After")=="" {t.Fatal("missing retry metadata")}
}
func TestFeatureKillSwitchDisabledAndEnabled(t *testing.T) {
    hits:=0
    downstream:=http.HandlerFunc(func(w http.ResponseWriter,_ *http.Request){hits++;w.WriteHeader(204)})
    logger:=slog.New(slog.NewTextHandler(io.Discard,nil))
    req:=httptest.NewRequest("POST","/api/v1/posts",nil)
    for _,tc:=range []struct{enabled bool;status int}{{false,503},{true,204}} {
        gate:=operationalGateMiddleware(simulatedGate{enabled:tc.enabled},logger,downstream)
        rec:=httptest.NewRecorder()
        gate.ServeHTTP(rec,req)
        if rec.Code!=tc.status {t.Fatalf("enabled=%v got=%d",tc.enabled,rec.Code)}
    }
    if hits!=1 {t.Fatalf("expected only enabled request, got %d",hits)}
}
