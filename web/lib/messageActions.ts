'use client';

import { ChatMessage, getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

export type ReactionSummary = {
  reaction: string;
  count: number;
  mine: boolean;
};

export type ReactionBatch = Record<string, ReactionSummary[]>;

async function apiFetch(path: string, init: RequestInit = {}) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) throw new Error('Authentication required');

  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${access}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Authentication required');
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

async function errorMessage(response: Response) {
  const payload = await response.json().catch(() => null);
  return payload?.error?.message ?? 'Request failed';
}

export async function editChatMessage(messageId: string, text: string): Promise<ChatMessage> {
  const response = await apiFetch(`/api/v1/messages/${encodeURIComponent(messageId)}`, {
    method: 'PUT', body: JSON.stringify({ text })
  });
  if (!response.ok) throw new Error(await errorMessage(response));
  return response.json() as Promise<ChatMessage>;
}

export async function deleteChatMessage(messageId: string) {
  const response = await apiFetch(`/api/v1/messages/${encodeURIComponent(messageId)}`, { method: 'DELETE' });
  if (!response.ok && response.status !== 404) throw new Error(await errorMessage(response));
}

export async function listChatMessageReactions(messageId: string): Promise<ReactionSummary[]> {
  const response = await apiFetch(`/api/v1/messages/${encodeURIComponent(messageId)}/reactions`);
  if (!response.ok) throw new Error(await errorMessage(response));
  const payload = await response.json() as { items: ReactionSummary[] };
  return payload.items;
}

export async function listReactionBatch(messageIds: string[]): Promise<ReactionBatch> {
  if (messageIds.length === 0) return {};
  const response = await apiFetch('/api/v1/messages/reactions/batch', {
    method: 'POST', body: JSON.stringify({ message_ids: messageIds.slice(0,100) })
  });
  if (!response.ok) throw new Error(await errorMessage(response));
  const payload = await response.json() as { items: ReactionBatch };
  return payload.items;
}

export async function setChatMessageReaction(messageId: string, reaction: string, enabled: boolean) {
  const response = await apiFetch(`/api/v1/messages/${encodeURIComponent(messageId)}/reactions/${encodeURIComponent(reaction)}`, {
    method: enabled ? 'PUT' : 'DELETE'
  });
  if (!response.ok) throw new Error(await errorMessage(response));
}
