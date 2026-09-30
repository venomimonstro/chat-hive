'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

export type AdminPrincipal = { user_id: string; roles: string[] };
export type ModerationCase = {
  id: string;
  target_type: 'user' | 'message' | 'post' | 'group';
  target_id: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  status: string;
  reason: string;
  report_count: number;
  created_at: string;
  updated_at: string;
};
export type SecurityEvent = {
  id: number;
  event_type: string;
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical';
  user_id?: string;
  session_id?: string;
  source_ip?: string;
  subject_type: string;
  subject_id: string;
  metadata: Record<string, unknown>;
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

export async function getAdminPrincipal(): Promise<AdminPrincipal | null> {
  const response = await call('/api/v1/admin/me');
  if (response.status === 404) return null;
  if (!response.ok) throw new Error('Admin access unavailable');
  return response.json() as Promise<AdminPrincipal>;
}

export async function listModerationCases(): Promise<ModerationCase[]> {
  const response = await call('/api/v1/admin/moderation/cases?limit=100');
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить очередь');
  const payload = await response.json() as { items: ModerationCase[] };
  return payload.items;
}

export async function listSecurityEvents(severity = ''): Promise<SecurityEvent[]> {
  const params = new URLSearchParams({ limit: '200' });
  if (severity) params.set('severity', severity);
  const response = await call(`/api/v1/admin/security/events?${params.toString()}`);
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить security events');
  const payload = await response.json() as { items: SecurityEvent[] };
  return payload.items;
}

export async function decideModerationCase(caseId: string, decision: 'resolve' | 'dismiss', reason: string) {
  const response = await call(`/api/v1/admin/moderation/cases/${encodeURIComponent(caseId)}/decision`, {
    method: 'POST', body: JSON.stringify({ decision, reason })
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message ?? 'Не удалось завершить кейс');
  }
}
