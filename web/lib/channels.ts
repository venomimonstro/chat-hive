'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

export type Channel = {
  id: string;
  owner_id: string;
  slug: string;
  title: string;
  description: string;
  moderation_status: 'pending' | 'approved' | 'rejected';
  subscribers_count: number;
  subscribed: boolean;
  mine: boolean;
  created_at: string;
};

export type ChannelPost = { id: string; kind: string; body: string; created_at: string };

async function authenticatedCall(path: string, init: RequestInit = {}) {
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

async function publicRead(path: string) {
  const access = getAccessToken();
  const headers = new Headers({ Accept: 'application/json' });
  if (access) headers.set('Authorization', `Bearer ${access}`);
  let response = await fetch(`${API_BASE}${path}`, { headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401 && access) {
    const refreshed = await refreshSession();
    const retryHeaders = new Headers({ Accept: 'application/json' });
    if (refreshed) retryHeaders.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { headers: retryHeaders, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

async function fail(response: Response) {
  const payload = await response.json().catch(() => null);
  return new Error(payload?.error?.message ?? 'Request failed');
}

export async function discoverChannels(): Promise<Channel[]> {
  const response = await publicRead('/api/v1/channels?limit=50');
  if (!response.ok) throw await fail(response);
  const payload = await response.json() as { items: Channel[] };
  return payload.items;
}

export async function createChannel(input: { slug: string; title: string; description: string }): Promise<Channel> {
  const response = await authenticatedCall('/api/v1/channels', { method: 'POST', body: JSON.stringify(input) });
  if (!response.ok) throw await fail(response);
  return response.json() as Promise<Channel>;
}

export async function getChannel(slug: string): Promise<Channel> {
  const response = await publicRead(`/api/v1/channels/${encodeURIComponent(slug)}`);
  if (!response.ok) throw await fail(response);
  return response.json() as Promise<Channel>;
}

export async function setChannelSubscription(slug: string, enabled: boolean): Promise<Channel> {
  const response = await authenticatedCall(`/api/v1/channels/${encodeURIComponent(slug)}/subscription`, { method: enabled ? 'PUT' : 'DELETE' });
  if (!response.ok) throw await fail(response);
  return response.json() as Promise<Channel>;
}

export async function listChannelPosts(slug: string): Promise<ChannelPost[]> {
  const response = await publicRead(`/api/v1/channels/${encodeURIComponent(slug)}/posts?limit=50`);
  if (!response.ok) throw await fail(response);
  const payload = await response.json() as { items: ChannelPost[] };
  return payload.items;
}

export async function createChannelPost(slug: string, body: string): Promise<ChannelPost> {
  const response = await authenticatedCall(`/api/v1/channels/${encodeURIComponent(slug)}/posts`, { method: 'POST', body: JSON.stringify({ body }) });
  if (!response.ok) throw await fail(response);
  return response.json() as Promise<ChannelPost>;
}
