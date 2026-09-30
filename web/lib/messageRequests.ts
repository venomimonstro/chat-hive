'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

export type MessageRequest = {
  chat_id: string;
  requester_id: string;
  username: string;
  display_name: string;
  body: string;
  created_at: string;
  shared_interests: number;
  mutual_followers: number;
};

async function call(path: string, init: RequestInit = {}) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) throw new Error('Authentication required');
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${access}`);
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Authentication required');
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

export async function listMessageRequests(): Promise<MessageRequest[]> {
  const response = await call('/api/v1/message-requests?limit=100');
  if (!response.ok) throw new Error('Не удалось загрузить запросы');
  const payload = await response.json() as { items: MessageRequest[] };
  return payload.items;
}

export async function decideMessageRequest(chatId: string, accept: boolean) {
  const response = await call(`/api/v1/message-requests/${encodeURIComponent(chatId)}/${accept ? 'accept' : 'reject'}`, { method: 'POST' });
  if (!response.ok && response.status !== 404) throw new Error('Не удалось обработать запрос');
}
