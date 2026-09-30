'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

export type Community = {
  id: string;
  chat_id: string;
  slug: string;
  title: string;
  description: string;
  visibility: 'private' | 'public';
  moderation_status: 'not_required' | 'pending' | 'approved' | 'rejected';
  role: '' | 'owner' | 'moderator' | 'member';
  members_count: number;
  created_at: string;
};

async function call(path: string, init: RequestInit = {}) {
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
  const body = await response.json().catch(() => null);
  return body?.error?.message ?? 'Request failed';
}

export async function discoverCommunities(): Promise<Community[]> {
  const response = await call('/api/v1/communities?limit=50');
  if (!response.ok) throw new Error(await errorMessage(response));
  const payload = await response.json() as { items: Community[] };
  return payload.items;
}

export async function getCommunity(slug: string): Promise<Community> {
  const response = await call(`/api/v1/communities/${encodeURIComponent(slug)}`);
  if (!response.ok) throw new Error(await errorMessage(response));
  return response.json() as Promise<Community>;
}

export async function createCommunity(input: { slug: string; title: string; description: string; visibility: 'private' | 'public' }): Promise<Community> {
  const response = await call('/api/v1/communities', { method: 'POST', body: JSON.stringify(input) });
  if (!response.ok) throw new Error(await errorMessage(response));
  return response.json() as Promise<Community>;
}

export async function joinCommunity(slug: string): Promise<Community> {
  const response = await call(`/api/v1/communities/${encodeURIComponent(slug)}/join`, { method: 'POST' });
  if (!response.ok) throw new Error(await errorMessage(response));
  return response.json() as Promise<Community>;
}

export async function leaveCommunity(slug: string) {
  const response = await call(`/api/v1/communities/${encodeURIComponent(slug)}/leave`, { method: 'POST' });
  if (!response.ok) throw new Error(await errorMessage(response));
}
