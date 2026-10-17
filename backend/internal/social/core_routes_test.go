package social
import ("net/http";"net/http/httptest";"testing")
func TestCoreSocialRoutesDoNotOfferPublicProfiles(t *testing.T){
 h:=&HTTPHandler{}
 mux:=http.NewServeMux()
 h.RegisterCore(mux)
 for _,path:=range []string{"/api/v1/profiles/alice","/api/v1/profiles/alice/followers","/api/v1/discovery/people"}{
  response:=httptest.NewRecorder()
  mux.ServeHTTP(response,httptest.NewRequest(http.MethodGet,path,nil))
  if response.Code!=http.StatusNotFound{t.Errorf("public profile route registered: %s status %d",path,response.Code)}
 }
}
