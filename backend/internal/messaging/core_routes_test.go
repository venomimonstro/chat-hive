package messaging
import (
  "net/http"
  "net/http/httptest"
  "testing"
)
func TestCoreRegistrationNeverExposesReactions(t *testing.T){
  handler:=&HTTPHandler{}
  mux:=http.NewServeMux()
  handler.RegisterCore(mux)
  for _,method:=range []string{http.MethodGet,http.MethodPut,http.MethodDelete}{
    req:=httptest.NewRequest(method,"/api/v1/messages/123/reactions/heart",nil)
    rec:=httptest.NewRecorder()
    mux.ServeHTTP(rec,req)
    if rec.Code!=http.StatusNotFound{t.Errorf("unexpected reaction route %s: %d",method,rec.Code)}
  }
}
