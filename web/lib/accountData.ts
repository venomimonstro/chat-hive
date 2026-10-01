'use client';

import { getAccessToken, refreshSession, setAccessToken } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

async function authorized(path: string, init: RequestInit = {}) {
  let token = getAccessToken();
  if (!token) token = (await refreshSession())?.access_token ?? null;
  if (!token) throw new Error('Требуется вход');
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Требуется вход');
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

async function message(response: Response) {
  const body = await response.json().catch(() => null);
  return body?.error?.message ?? 'Не удалось выполнить операцию';
}

export async function downloadAccountExport() {
  const response = await authorized('/api/v1/me/data-export');
  if (!response.ok) throw new Error(await message(response));
  const blob = await response.blob();
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = href;
  anchor.download = `chat-data-${new Date().toISOString().slice(0, 10)}.json`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(href), 1000);
}

export async function deleteAccount(confirmation: string) {
  const response = await authorized('/api/v1/me/account', {
    method: 'DELETE',
    body: JSON.stringify({ confirmation })
  });
  if (!response.ok) throw new Error(await message(response));
  setAccessToken(null);
}
