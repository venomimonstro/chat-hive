#!/usr/bin/env python3
"""Low-volume adversarial checks for an explicitly owned CHAT CORE staging host.

This is a safe negative-test suite, NOT a penetration test, fuzz storm,
vulnerability scan of unrelated hosts or a guarantee of no exploitable flaws.
"""
from __future__ import annotations
import json
import os
import re
import ssl
import sys
import uuid
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler, HTTPSHandler
from urllib.error import HTTPError

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,req,fp,code,msg,headers,newurl):
        raise RuntimeError("unexpected redirect in protected API")

def call(opener, origin, path, method="GET", token=None, payload=None, extra=None):
    headers = {"Accept":"application/json"}
    if token: headers["Authorization"] = "Bearer " + token
    if extra: headers.update(extra)
    if payload is not None: headers["Content-Type"] = "application/json"
    request = Request(origin+path, data=payload, headers=headers, method=method)
    try:
        with opener.open(request,timeout=15) as response:
            status, headers, body = response.status, response.headers, response.read(8192)
    except HTTPError as err:
        status, headers, body = err.code, err.headers, err.read(8192)
    try: data = json.loads(body) if body else {}
    except (ValueError,UnicodeDecodeError): data = {}
    return status, headers, data

def require(condition, reason):
    if not condition: raise AssertionError(reason)

def main():
    origin = os.environ.get("CHAT_CORE_URL","").rstrip("/")
    parsed = urlsplit(origin)
    require(parsed.scheme=="https" and bool(parsed.hostname) and not parsed.username and
            not parsed.password and not parsed.path and not parsed.query and not parsed.fragment,
            "CHAT_CORE_URL must be a bare HTTPS origin")
    require(os.environ.get("CHAT_CORE_ATTACK_SIMULATION_ACK") ==
            "I_OWN_THIS_STAGING_HOST_AND_APPROVE_SAFE_TESTS",
            "explicit staging-owner acknowledgement required")
    token = os.environ.get("CHAT_CORE_ACCESS_TOKEN_A","")
    require(len(token)>16,"dedicated staging test-user token required")
    opener = build_opener(NoRedirect,HTTPSHandler(context=ssl.create_default_context()))
    results = []
    def check(statuses,path,method="GET",bearer=None,payload=None,extra=None,label=None):
        status,headers,data = call(opener,origin,path,method,bearer,payload,extra)
        require(status in statuses,f"{label or path}: unexpected HTTP {status}; wanted {sorted(statuses)}")
        results.append(label or path)
        return status,headers,data

    _, meta, status = 0, None, None
    status,headers,meta=check({200},"/api/v1",label="restricted deployment metadata")
    require(meta.get("launch_mode")=="core","unrestricted feature mode exposed")
    for path in ("/api/v1/chats","/api/v1/me/profile","/api/v1/me/preferences"):
        check({401,403},path,label="unauthenticated "+path)
    check({401,403},"/api/v1/chats",bearer="tampered.access.token",
          label="forged bearer must fail")
    admin_status,_,_=check({401,403},"/api/v1/admin/security/events",bearer=token,
                          label="normal account cannot query privileged audit log")
    for path in ("/api/v1/channels","/api/v1/groups","/api/v1/feed",
                 "/api/v1/profiles/public-user","/api/v1/media/images/anything",
                 "/api/v1/notifications"):
        check({404},path,bearer=token,label="unlaunched "+path)
    for path,method in (("/api/v1/posts","POST"),("/api/v1/media/images","POST"),
                        ("/api/v1/channels","POST")):
        check({404},path,method,bearer=token,label="future write rejected "+path)
    check({404},f"/api/v1/chats/{uuid.uuid4()}/messages",bearer=token,
          label="nonmember chat history concealed")
    check({404},"/api/v1/me/preferences","POST",bearer=token,payload=b"{}",
          label="disallowed preference method")
    # Exercise parser rejection but never submit a valid write. Must stay 400,
    # rather than updating the user's saved language from the first object.
    check({400},"/api/v1/me/preferences","PUT",bearer=token,
          payload=b'{"language":"ru"}{"language":"en"}',
          label="trailing JSON cannot bypass preference validation")
    check({400},"/api/v1/chats/direct","POST",bearer=token,
          payload=b'{"username":"safe-user"}{"username":"other-user"}',
          label="trailing JSON cannot create an unauthorized conversation")
    status,h,_=check({403},"/api/v1/auth/session","OPTIONS",
                    extra={"Origin":"https://attacker.invalid",
                           "Access-Control-Request-Method":"GET"},
                    label="cross-site CORS preflight denied")
    require(not h.get("Access-Control-Allow-Origin"),"attacker origin reflected into CORS")
    status,h,_=check({200},"/api/v1",label="security response headers")
    require("no-store" in h.get("Cache-Control","").lower(),"API response is cacheable")
    require(h.get("X-Content-Type-Options","").lower()=="nosniff","missing nosniff")
    require(h.get("X-Frame-Options","").lower()=="deny","missing clickjacking protection")
    status,_,_=check({403},"/api/v1/realtime?ticket=invalid","GET",
                     extra={"Origin":"https://attacker.invalid"},
                     label="foreign-origin websocket rejected")
    print(f"SAFE ADVERSARIAL CORE CHECKS PASS: {len(results)} requests/controls; no brute forcing or stress traffic")

if __name__=="__main__":
    try: main()
    except Exception as exc:
        # No token, session body or personal message content in error output.
        print(f"SAFE ADVERSARIAL CHECKS BLOCKED: {type(exc).__name__}: {exc}",file=sys.stderr)
        raise SystemExit(1)
