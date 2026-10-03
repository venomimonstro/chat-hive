'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';
const SESSION_KEY = 'chat.acquisition_id';

export type GrowthObjectType = '' | 'profile' | 'post' | 'community' | 'channel' | 'group_invite';
export type GrowthEventName =
  | 'public_view'
  | 'login_started'
  | 'signup_completed'
  | 'follow'
  | 'community_join'
  | 'channel_subscribe'
  | 'invite_join'
  | 'first_post'
  | 'first_message';

function acquisitionId() {
  if (typeof window === 'undefined') return '';
  let value = window.sessionStorage.getItem(SESSION_KEY) ?? '';
  if (!value) {
    value = crypto.randomUUID();
    window.sessionStorage.setItem(SESSION_KEY, value);
  }
  return value;
}

async function optionalAccessToken() {
  let token = getAccessToken();
  if (token) return token;
  try {
    token = (await refreshSession())?.access_token ?? null;
  } catch {
    token = null;
  }
  return token;
}

export async function recordGrowthEvent(
  eventName: GrowthEventName,
  objectType: GrowthObjectType = '',
  objectId = ''
) {
  if (typeof window === 'undefined') return;
  const token = await optionalAccessToken();
  const headers = new Headers({ 'Content-Type': 'application/json', Accept: 'application/json' });
  if (token) headers.set('Authorization', `Bearer ${token}`);

  await fetch(`${API_BASE}/api/v1/growth/events`, {
    method: 'POST',
    headers,
    credentials: 'include',
    cache: 'no-store',
    keepalive: true,
    body: JSON.stringify({
      event_name: eventName,
      acquisition_id: acquisitionId(),
      object_type: objectType,
      object_id: objectId.slice(0, 128),
      source: document.referrer
    })
  }).catch(() => undefined);
}
