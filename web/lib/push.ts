'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
const VAPID_PUBLIC_KEY = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY ?? '';

function base64UrlToBytes(value: string) {
  const padding = '='.repeat((4 - value.length % 4) % 4);
  const base64 = (value + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = window.atob(base64);
  return Uint8Array.from(raw, (char) => char.charCodeAt(0));
}

async function accessToken() {
  let token = getAccessToken();
  if (!token) token = (await refreshSession())?.access_token ?? null;
  return token;
}

async function authRequest(path: string, init: RequestInit) {
  let token = await accessToken();
  if (!token) return null;
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);
  headers.set('Content-Type', 'application/json');
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    token = refreshed.access_token;
    headers.set('Authorization', `Bearer ${token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include' });
  }
  return response;
}

export function pushAvailable() {
  return Boolean(VAPID_PUBLIC_KEY) && typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;
}

export async function enablePush() {
  if (!pushAvailable()) throw new Error('Push-уведомления пока недоступны');
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') throw new Error('Разрешение на уведомления не выдано');
  const registration = await navigator.serviceWorker.ready;
  let subscription = await registration.pushManager.getSubscription();
  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: base64UrlToBytes(VAPID_PUBLIC_KEY)
    });
  }
  const json = subscription.toJSON();
  const response = await authRequest('/api/v1/notifications/push-subscription', {
    method: 'PUT',
    body: JSON.stringify({ endpoint: subscription.endpoint, keys: { p256dh: json.keys?.p256dh ?? '', auth: json.keys?.auth ?? '' } })
  });
  if (!response) throw new Error('Требуется вход');
  if (!response.ok) throw new Error('Не удалось включить push-уведомления');
  return subscription;
}

export async function disablePush() {
  if (!('serviceWorker' in navigator)) return;
  const registration = await navigator.serviceWorker.ready;
  const subscription = await registration.pushManager.getSubscription();
  if (!subscription) return;
  const endpoint = subscription.endpoint;
  const response = await authRequest('/api/v1/notifications/push-subscription', { method: 'DELETE', body: JSON.stringify({ endpoint }) });
  if (response && !response.ok && response.status !== 404) throw new Error('Не удалось отключить push-уведомления');
  await subscription.unsubscribe();
}
