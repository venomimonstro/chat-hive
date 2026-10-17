package httpserver

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestCoreLaunchRouteAllowlist(t *testing.T) {
    allow := []struct{ method,path string }{
        {"GET","/health/ready"},{"GET","/api/v1/auth/session"},
        {"POST","/api/v1/auth/email/start"},{"POST","/api/v1/auth/refresh"},
        {"GET","/api/v1/admin/me"},{"POST","/api/v1/growth/events"},
        {"POST","/api/v1/auth/passkeys/login/begin"},
        {"POST","/api/v1/auth/passkeys/register/finish/c"},
        {"GET","/api/v1/auth/yandex/callback"},
        {"DELETE","/api/v1/auth/sessions/id"},
        {"GET","/api/v1/me/profile"},{"PUT","/api/v1/me/onboarding"},
        {"PUT","/api/v1/me/preferences"},
        {"GET","/api/v1/chats"},{"POST","/api/v1/chats/direct"},
        {"GET","/api/v1/chats/id/messages"},{"POST","/api/v1/chats/id/messages"},
        {"POST","/api/v1/chats/id/read"},{"GET","/api/v1/message-requests"},
        {"POST","/api/v1/message-requests/id/accept"},{"GET","/api/v1/realtime"},
        {"POST","/api/v1/realtime/ticket"},{"POST","/api/v1/profiles/alice/block"},
        {"DELETE","/api/v1/me/account"},
    }
    for _,tc:=range allow {if !coreAPIAllowed(tc.method,tc.path){t.Errorf("blocked core route %s %s",tc.method,tc.path)}}
    blocked:=[]struct{method,path string}{
        {"GET","/api/v1/feed"},{"GET","/api/v1/channels"},
        {"POST","/api/v1/auth/unexpected/future"},
        {"GET","/api/v1/auth/refresh"},
        {"POST","/api/v1/auth/email/start/extra"},
        {"GET","/api/v1/auth/session/secrets"},
        {"POST","/api/v1/channels"},{"POST","/api/v1/groups"},
        {"GET","/api/v1/media/images/id"},{"POST","/api/v1/media/images"},
        {"GET","/api/v1/notifications"},
        {"GET","/api/v1/discovery/people"},{"POST","/api/v1/posts"},
        {"POST","/api/v1/messages/id/reactions/👍"},
        {"GET","/api/v1/messages/id/reactions"},
        {"POST","/api/v1/messages/reactions/batch"},
        {"GET","/api/v1/profiles/alice"},
        {"POST","/api/v1/profiles/alice/follow"},
        {"GET","/api/v1/me/data-export/anything"},
        {"POST","/api/v1/me/onboarding"},
    }
    for _,tc:=range blocked {if coreAPIAllowed(tc.method,tc.path){t.Errorf("exposed future route %s %s",tc.method,tc.path)}}
}
func TestCoreLaunchBlocksWithoutCallingHandlers(t *testing.T) {
    calls:=0
    handler:=coreLaunchMiddleware(true,http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls++;w.WriteHeader(http.StatusNoContent)}))
    forbidden:=httptest.NewRecorder()
    handler.ServeHTTP(forbidden,httptest.NewRequest("POST","/api/v1/media/images",nil))
    if forbidden.Code!=http.StatusNotFound || calls!=0 {t.Fatalf("blocked endpoint passed through: %d %d",forbidden.Code,calls)}
    ok:=httptest.NewRecorder()
    handler.ServeHTTP(ok,httptest.NewRequest("GET","/api/v1/chats",nil))
    if ok.Code!=http.StatusNoContent || calls!=1 {t.Fatal("allowed messenger route failed")}
}
