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
  private reconnectAttempt = 0;
  private reconnectTimer: number | null = null;
  private pingTimer: number | null = null;

  start() {
    this.stopped = false;
    void this.connect();
  }

  stop() {
    this.stopped = true;
    if (this.reconnectTimer !== null) window.clearTimeout(this.reconnectTimer);
    if (this.pingTimer !== null) window.clearInterval(this.pingTimer);
    this.reconnectTimer = null;
    this.pingTimer = null;
    this.socket?.close(1000, 'client shutdown');
    this.socket = null;
  }

  private async connect() {
    if (this.stopped || this.socket) return;
    const ticket = await issueTicket().catch(() => null);
    if (!ticket) {
      this.scheduleReconnect();
      return;
    }

    let socket: WebSocket;
    try { socket = new WebSocket(websocketURL(ticket.ticket)); }
    catch { this.scheduleReconnect(); return; }
    this.socket = socket;

    socket.onopen = () => {
      this.reconnectAttempt = 0;
      if (this.pingTimer !== null) window.clearInterval(this.pingTimer);
      this.pingTimer = window.setInterval(() => {
        if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'ping' }));
      }, 25_000);
    };

    socket.onmessage = (message) => {
      try {
        const event = JSON.parse(String(message.data)) as RealtimeEvent;
        window.dispatchEvent(new CustomEvent<RealtimeEvent>('chat:realtime', { detail: event }));
      } catch {
        // Invalid server events are ignored; REST sync remains authoritative.
      }
    };

    socket.onclose = () => {
      if (this.socket === socket) this.socket = null;
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
