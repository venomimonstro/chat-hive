'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

export type MediaObject = {
  id: string;
  mime_type: string;
  byte_size: number;
  width: number;
  height: number;
  url: string;
};

async function requiredAuthHeader() {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) throw new Error('Необходимо войти снова.');
  return { Authorization: `Bearer ${access}` };
}

export async function uploadImage(file: File): Promise<MediaObject> {
  if (!['image/jpeg', 'image/png'].includes(file.type)) throw new Error('Поддерживаются JPEG и PNG.');
  if (file.size <= 0 || file.size > 8 * 1024 * 1024) throw new Error('Размер изображения должен быть не больше 8 МБ.');
  const form = new FormData();
  form.append('file', file, file.name);
  let headers = await requiredAuthHeader();
  let response = await fetch(`${API_BASE}/api/v1/media/images`, { method: 'POST', body: form, credentials: 'include', headers });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Необходимо войти снова.');
    headers = { Authorization: `Bearer ${refreshed.access_token}` };
    response = await fetch(`${API_BASE}/api/v1/media/images`, { method: 'POST', body: form, credentials: 'include', headers });
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message ?? 'Не удалось загрузить изображение');
  }
  return response.json() as Promise<MediaObject>;
}

export async function fetchMediaBlob(url: string): Promise<Blob> {
  const access = getAccessToken();
  const headers: Record<string,string> = access ? { Authorization: `Bearer ${access}` } : {};
  let response = await fetch(`${API_BASE}${url}`, { credentials: 'include', cache: 'force-cache', headers });

  // A stale in-memory token may fail while the object is actually public. Refresh once;
  // if there is no valid session, retry anonymously. The backend remains the authority
  // and returns private/follower/chat media only when access is allowed.
  if (response.status === 401 && access) {
    const refreshed = await refreshSession();
    const retryHeaders: Record<string,string> = refreshed ? { Authorization: `Bearer ${refreshed.access_token}` } : {};
    response = await fetch(`${API_BASE}${url}`, { credentials: 'include', cache: 'force-cache', headers: retryHeaders });
  }
  if (!response.ok) throw new Error('Изображение недоступно');
  return response.blob();
}
