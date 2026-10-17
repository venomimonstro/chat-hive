#!/usr/bin/env python3
"""End-to-end smoke of CHAT core launch on two explicitly dedicated test accounts."""
from __future__ import annotations
import json
import os
from urllib.parse import urlsplit, quote
from urllib.request import Request, HTTPSHandler, build_opener, HTTPRedirectHandler
from urllib.error import HTTPError
import ssl
import sys
import uuid

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("unexpected HTTP redirect")

def request(opener, origin: str, method: str, path: str, token: str = "", body=None):
    headers = {"Accept": "application/json", "Cache-Control": "no-store"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(origin + path, data=data, headers=headers, method=method)
    try:
        with opener.open(req, timeout=20) as response:
            status, payload = response.status, response.read(1024*1024)
    except HTTPError as exc:
        status, payload = exc.code, exc.read(4096)
    try:
        output = json.loads(payload) if payload else {}
    except (json.JSONDecodeError, UnicodeDecodeError):
        output = {}
    return status, output

def need(status, actual, operation):
    if actual != status:
        raise RuntimeError(f"{operation}: expected HTTP {status}, got {actual}")

def run():
    origin = os.getenv("CHAT_CORE_URL", "").rstrip("/")
    parsed = urlsplit(origin)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.query or parsed.fragment or parsed.path:
        raise ValueError("CHAT_CORE_URL must be an HTTPS origin without path or credentials")
    if os.getenv("CHAT_CORE_SMOKE_CONFIRM") != "I_AM_USING_DEDICATED_TEST_USERS":
        raise ValueError("Use two dedicated test accounts and explicitly acknowledge the test writes")
    a = os.getenv("CHAT_CORE_ACCESS_TOKEN_A", "")
    b = os.getenv("CHAT_CORE_ACCESS_TOKEN_B", "")
    peer = os.getenv("CHAT_CORE_USERNAME_B", "")
    if min(len(a),len(b)) < 16 or a == b or not peer or len(peer)>32:
        raise ValueError("two different dedicated tokens and a recipient username are required")
    opener = build_opener(NoRedirect, HTTPSHandler(context=ssl.create_default_context()))
    code,_ = request(opener,origin,"GET","/health/ready")
    need(200,code,"readiness")
    code,sess_a = request(opener,origin,"GET","/api/v1/auth/session",a)
    need(200,code,"session A")
    code,sess_b = request(opener,origin,"GET","/api/v1/auth/session",b)
    need(200,code,"session B")
    if not sess_a.get("user_id") or not sess_b.get("user_id") or sess_a["user_id"] == sess_b["user_id"]:
        raise RuntimeError("test accounts are missing or identical")
    code,profile = request(opener,origin,"GET","/api/v1/me/profile",b)
    need(200,code,"B profile")
    if profile.get("username") != peer:
        raise RuntimeError("B username mismatch")
    code,settings = request(opener,origin,"GET","/api/v1/me/preferences",a)
    need(200,code,"read language preferences")
    previous = settings.get("language")
    if previous not in ("ru","en"): raise RuntimeError("invalid saved language")
    changed = "en" if previous == "ru" else "ru"
    code,data = request(opener,origin,"PUT","/api/v1/me/preferences",a,{"language":changed})
    need(200,code,"save account language")
    if data.get("language") != changed: raise RuntimeError("language change was not persisted")
    code,data = request(opener,origin,"GET","/api/v1/me/preferences",a)
    if code != 200 or data.get("language") != changed: raise RuntimeError("language read-back failed")
    code,_ = request(opener,origin,"PUT","/api/v1/me/preferences",a,{"language":previous})
    need(200,code,"restore original language")
    code,chat = request(opener,origin,"POST","/api/v1/chats/direct",a,{"username":peer})
    need(200,code,"create direct chat")
    cid = chat.get("chat_id")
    if chat.get("kind") != "direct" or not isinstance(cid,str): raise RuntimeError("created chat is not direct")
    message_id = str(uuid.uuid4())
    body = "CHAT_CORE_E2E_" + uuid.uuid4().hex[:12]
    payload={"client_message_id":message_id,"text":body}
    code,sent = request(opener,origin,"POST",f"/api/v1/chats/{quote(cid)}/messages",a,payload)
    need(201,code,"send direct message")
    stored=sent.get("message",{})
    if stored.get("body") != body: raise RuntimeError("message did not persist")
    code,replay = request(opener,origin,"POST",f"/api/v1/chats/{quote(cid)}/messages",a,payload)
    need(200,code,"idempotent duplicate message")
    if not replay.get("duplicate") or replay.get("message",{}).get("id") != stored.get("id"):
        raise RuntimeError("duplicate send created new message or lost idempotency")
    code,history = request(opener,origin,"GET",f"/api/v1/chats/{quote(cid)}/messages",a)
    need(200,code,"message history")
    matches=[m for m in history.get("items",[]) if m.get("client_message_id")==message_id]
    if len(matches) != 1: raise RuntimeError("message missing or duplicated in history")
    code,requests = request(opener,origin,"GET","/api/v1/message-requests",b)
    need(200,code,"recipient request inbox")
    pending=any(item.get("chat_id")==cid for item in requests.get("items",[]))
    if pending:
        code,_ = request(opener,origin,"POST",f"/api/v1/message-requests/{quote(cid)}/accept",b,{})
        need(204,code,"recipient accepts message request")
    code,received = request(opener,origin,"GET",f"/api/v1/chats/{quote(cid)}/messages",b)
    need(200,code,"recipient receives message")
    if not any(m.get("client_message_id")==message_id for m in received.get("items",[])):
        raise RuntimeError("recipient did not receive message")
    for method,path in [
        ("GET","/api/v1/channels"),("GET","/api/v1/communities"),
        ("GET","/api/v1/feed"),("GET","/api/v1/media/images/sample"),
        ("POST","/api/v1/posts"),("POST","/api/v1/media/images"),
        ("POST","/api/v1/groups"),
        ("GET","/api/v1/messages/"+stored["id"]+"/reactions")
    ]:
        code,_ = request(opener,origin,method,path,a)
        need(404,code,"disabled feature "+path)
    code,_ = request(opener,origin,"GET","/api/v1/chats")
    need(401,code,"unauthenticated access")
    print("CORE E2E PASS: two users, account language, direct chat, one-time send, recipient delivery, disabled future features")

if __name__ == "__main__":
    try:
        run()
    except Exception as exc:
        # Deliberately do not print HTTP request headers or credential contents.
        print(f"CORE E2E BLOCKED: {type(exc).__name__}: {exc}",file=sys.stderr)
        sys.exit(1)
