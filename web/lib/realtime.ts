'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

export type RealtimeEvent = {
  type: string;
  chat_id?: string;
  message_id?: string;
  sender_id?: string;
  sequence?: number;
  occurred_at?: string;
};

async function issueTicket() {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  let response = await fetch(`${API_BASE}/api/v1/realtime/ticket`, {
    method: 'POST', credentials: 'include', headers: { Accept: 'application/json', Authorization: `Bearer ${access}` }
  });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    response = await fetch(`${API_BASE}/api/v1/realtime/ticket`, {
      method: 'POST', credentials: 'include', headers: { Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` }
    });
  }
  if (!response.ok) return null;
  return await response.json() as { ticket: string; expires_at: string; expires_in: number };
}

function websocketURL(ticket: string) {
  const base = new URL(API_BASE || window.location.origin);
  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:';
  base.pathname = '/api/v1/realtime';
  base.search = new URLSearchParams({ ticket }).toString();
  return base.toString();
}

export class RealtimeClient {
  private socket: WebSocket | null = null;
  private stopped = false;
  private connectionAttempt = 0;
  private connecting = false;
  private reconnectAttempt = 0;
  private reconnectTimer: number | null = null;
  private pingTimer: number | null = null;

  start() {
    if (!this.stopped && (this.socket || this.connecting || this.reconnectTimer !== null)) return;
    this.stopped = false;
    void this.connect();
  }

  stop() {
    this.stopped = true;
    this.connectionAttempt += 1;
    this.connecting = false;
    this.reconnectAttempt = 0;
    if (this.reconnectTimer !== null) window.clearTimeout(this.reconnectTimer);
    if (this.pingTimer !== null) window.clearInterval(this.pingTimer);
    this.reconnectTimer = null;
    this.pingTimer = null;
    this.socket?.close(1000, 'client shutdown');
    this.socket = null;
    window.dispatchEvent(new CustomEvent<boolean>('chat:connection', { detail: false }));
  }

  private async connect() {
    if (this.stopped || this.socket || this.connecting) return;
    this.connecting = true;
    const attempt = ++this.connectionAttempt;
    let ticket: Awaited<ReturnType<typeof issueTicket>>;
    try { ticket = await issueTicket(); }
    catch { ticket = null; }
    // Only the attempt that set connecting is allowed to clear it. A stale
    // request from before stop()/start() must not unlock a newer attempt.
    if (attempt === this.connectionAttempt) this.connecting = false;
    // A pending ticket request may finish after logout/unmount/stop.
    if (this.stopped || attempt !== this.connectionAttempt) return;
    if (!ticket) {
      this.scheduleReconnect();
      return;
    }

    let socket: WebSocket;
    try { socket = new WebSocket(websocketURL(ticket.ticket)); }
    catch { this.scheduleReconnect(); return; }
    if (this.stopped || attempt !== this.connectionAttempt) {
      socket.close(1000, 'client shutdown');
      return;
    }
    this.socket = socket;
    let lastPongAt = Date.now();

    socket.onopen = () => {
      if (this.stopped || this.socket !== socket) { socket.close(1000, 'client shutdown'); return; }
      window.dispatchEvent(new CustomEvent<boolean>('chat:connection', { detail: true }));
      this.reconnectAttempt = 0;
      lastPongAt = Date.now();
      if (this.pingTimer !== null) window.clearInterval(this.pingTimer);
      this.pingTimer = window.setInterval(() => {
        if (this.stopped || socket !== this.socket || socket.readyState !== WebSocket.OPEN) return;
        // TCP/WebSocket can remain half-open after a network transition.
        // If two or more heartbeats go unanswered, force reconnect and
        // recover missed events through the durable REST synchronization.
        if (Date.now() - lastPongAt > 75_000) {
          socket.close(4000, 'heartbeat timeout');
          return;
        }
        try { socket.send(JSON.stringify({ type: 'ping' })); }
        catch { socket.close(4000, 'heartbeat send failed'); }
      }, 25_000);
    };

    socket.onmessage = (message) => {
      if (this.stopped || this.socket !== socket) return;
      try {
        const event = JSON.parse(String(message.data)) as RealtimeEvent;
        if (event.type === 'pong' || event.type === 'ready') {
          lastPongAt = Date.now();
          return;
        }
        window.dispatchEvent(new CustomEvent<RealtimeEvent>('chat:realtime', { detail: event }));
      } catch {
        // Invalid server events are ignored; REST sync remains authoritative.
      }
    };

    socket.onclose = () => {
      if (this.socket !== socket) return;
      this.socket = null;
      window.dispatchEvent(new CustomEvent<boolean>('chat:connection', { detail: false }));
      if (this.pingTimer !== null) window.clearInterval(this.pingTimer);
      this.pingTimer = null;
      this.scheduleReconnect();
    };
    socket.onerror = () => socket.close();
  }

  private scheduleReconnect() {
    if (this.stopped || this.reconnectTimer !== null) return;
    const capped = Math.min(30_000, 1000 * 2 ** Math.min(this.reconnectAttempt, 5));
    const jitter = Math.floor(Math.random() * Math.max(250, Math.floor(capped * 0.25)));
    this.reconnectAttempt += 1;
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, capped + jitter);
  }
}
