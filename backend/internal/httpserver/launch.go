package httpserver

import (
    "encoding/json"
    "net/http"
    "strings"
)

// Core launch is intentionally fail-closed. Future routes stay registered for
// development but are unreachable in the limited public launch, even when
// called by an old client or hand-written HTTP request.
func coreAPIAllowed(method, path string) bool {
    if method == http.MethodOptions { return true }
    if path == "/health/live" || path == "/health/ready" || path == "/api/v1" { return method == http.MethodGet }
    if !strings.HasPrefix(path, "/api/v1/") { return false }
    fragments := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
    match := func(verbs ...string) bool { for _, verb := range verbs { if method == verb { return true } }; return false }
    switch fragments[0] {
    case "admin":
        // Keep administrative routes explicitly enumerated. These handlers
        // separately require privileged passkey sessions and role checks.
        // Adding a new admin handler must not silently widen core access.
        switch {
        case len(fragments)==2 && fragments[1]=="me":
            return match(http.MethodGet)
        case len(fragments)==3 && fragments[1]=="moderation" && fragments[2]=="cases":
            return match(http.MethodGet)
        case len(fragments)==5 && fragments[1]=="moderation" && fragments[2]=="cases" && fragments[4]=="decision":
            return match(http.MethodPost)
        case len(fragments)==3 && fragments[1]=="security" &&
            (fragments[2]=="events" || fragments[2]=="alerts" || fragments[2]=="summary"):
            return match(http.MethodGet)
        case len(fragments)==5 && fragments[1]=="security" && fragments[2]=="alerts" && fragments[4]=="ack":
            return match(http.MethodPost)
        case len(fragments)==3 && fragments[1]=="ops" &&
            (fragments[2]=="flags" || fragments[2]=="runtime"):
            return match(http.MethodGet)
        case len(fragments)==4 && fragments[1]=="ops" && fragments[2]=="flags":
            return match(http.MethodPut)
        case len(fragments)==3 && fragments[1]=="product" && fragments[2]=="metrics":
            return match(http.MethodGet)
        case len(fragments)==3 && fragments[1]=="beta" && fragments[2]=="readiness":
            return match(http.MethodGet)
        }
        return false
    case "growth":
        return len(fragments)==2 && fragments[1]=="events" && match(http.MethodPost)
    case "auth":
        // Explicit endpoints only; newly added authentication methods must
        // not become available in the limited launch accidentally.
        switch {
        case len(fragments)==3 && fragments[1]=="email" &&
            (fragments[2]=="start" || fragments[2]=="complete"):
            return match(http.MethodPost)
        case len(fragments)==3 && fragments[1]=="yandex" &&
            (fragments[2]=="start" || fragments[2]=="callback"):
            return match(http.MethodGet)
        case len(fragments)==2 && fragments[1]=="passkeys":
            return match(http.MethodGet)
        case len(fragments)==3 && fragments[1]=="passkeys":
            return match(http.MethodDelete)
        case len(fragments)==4 && fragments[1]=="passkeys" &&
            (fragments[2]=="register" || fragments[2]=="login") && fragments[3]=="begin":
            return match(http.MethodPost)
        case len(fragments)==5 && fragments[1]=="passkeys" &&
            (fragments[2]=="register" || fragments[2]=="login") && fragments[3]=="finish":
            return match(http.MethodPost)
        case len(fragments)==2 && (fragments[1]=="refresh" || fragments[1]=="session" || fragments[1]=="sessions"):
            if fragments[1]=="refresh" { return match(http.MethodPost) }
            return match(http.MethodGet)
        case len(fragments)==3 && fragments[1]=="sessions":
            return match(http.MethodDelete)
        }
        return false
    case "onboarding":
        return len(fragments)==2 && fragments[1]=="interests" && match(http.MethodGet)
    case "me":
        if len(fragments)==2 {
            switch fragments[1] {
            case "profile": return match(http.MethodGet)
            case "onboarding": return match(http.MethodPut)
            case "data-export": return match(http.MethodGet)
            case "account": return match(http.MethodDelete)
            case "preferences": return match(http.MethodGet,http.MethodPut)
            }
        }
    case "chats":
        if len(fragments)==1 { return match(http.MethodGet) }
        if len(fragments)==2 && fragments[1]=="direct" { return match(http.MethodPost) }
        if len(fragments)==3 {
            switch fragments[2] {
            case "messages": return match(http.MethodGet,http.MethodPost)
            case "read": return match(http.MethodPost)
            }
        }
    case "messages":
        if len(fragments)==2 { return match(http.MethodPut,http.MethodDelete) }
    case "message-requests":
        if len(fragments)==1 { return match(http.MethodGet) }
        if len(fragments)==3 && (fragments[2]=="accept" || fragments[2]=="reject") {
            return match(http.MethodPost)
        }
    case "realtime":
        if len(fragments)==1 { return match(http.MethodGet) }
        if len(fragments)==2 && fragments[1]=="ticket" { return match(http.MethodPost) }
    case "profiles":
        // Social follower counts, interests and biographies are not required
        // for core private messaging. Avoid public profile enumeration in v1.
        // Retain explicit contact blocking as an authenticated safety control.
        if len(fragments)==3 && (fragments[2]=="block" || fragments[2]=="unblock") {
            return match(http.MethodPost,http.MethodDelete)
        }
    }
    return false
}

func coreLaunchMiddleware(enabled bool, next http.Handler) http.Handler {
    if !enabled { return next }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if coreAPIAllowed(r.Method,r.URL.Path) {
            next.ServeHTTP(w,r)
            return
        }
        w.Header().Set("Cache-Control","no-store")
        w.Header().Set("Content-Type","application/json; charset=utf-8")
        w.WriteHeader(http.StatusNotFound)
        _ = json.NewEncoder(w).Encode(map[string]any{"error":map[string]string{
            "code":"feature_not_launched","message":"Feature not available in this release",
        }})
    })
}
