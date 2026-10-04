'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

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
export type SecurityCounter = { key: string; count: number };
export type SecurityAlert = {
  event_id: number;
  event_type: string;
  severity: 'high' | 'critical';
  status: 'open' | 'acknowledged' | 'resolved';
  source_ip?: string;
  subject_type: string;
  subject_id: string;
  metadata: Record<string, unknown>;
  created_at: string;
  acknowledged_at?: string;
  acknowledged_by?: string;
  note: string;
};

export type RuntimeSnapshot = {
  realtime_connections: number;
  realtime_published: number;
  realtime_dropped: number;
  db_acquired: number;
  db_idle: number;
  db_max: number;
  db_acquire_count: number;
  db_acquire_duration_ms: number;
  generated_at: string;
};

export type PlatformFlag = {
  key: string;
  enabled: boolean;
  reason: string;
  updated_by?: string;
  updated_at: string;
};

export type BetaReadiness = {
  privileged_without_passkey: number;
  open_high_critical_alerts: number;
  open_critical_cases: number;
  disabled_features: string[];
  internal_ready: boolean;
  external_required: string[];
  generated_at: string;
};

export type ProductMetrics = {
  window_days: number;
  registrations: number;
  onboarding_completed: number;
  first_message_users: number;
  first_post_users: number;
  follow_creators: number;
  public_views: number;
  login_starts: number;
  signup_completed_events: number;
  invite_joins: number;
  community_joins: number;
  channel_subscribes: number;
  active_users: number;
  d1_eligible: number;
  d1_retained: number;
  d7_eligible: number;
  d7_retained: number;
  open_moderation_cases: number;
  generated_at: string;
};

export type SecuritySummary = {
  critical_15m: number;
  high_15m: number;
  medium_15m: number;
  critical_1h: number;
  high_1h: number;
  events_24h: number;
  top_event_types_1h: SecurityCounter[];
  top_source_ips_1h: SecurityCounter[];
  generated_at: string;
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
  if (response.status === 403) {
    const payload = await response.clone().json().catch(() => null);
    if (payload?.error?.code === 'passkey_required') {
      if (typeof window !== 'undefined') {
        const next = window.location.pathname + window.location.search;
        window.location.assign(`/login?next=${encodeURIComponent(next)}`);
      }
      throw new Error('Для привилегированного доступа войдите с passkey');
    }
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

export async function listSecurityAlerts(status = 'open'): Promise<SecurityAlert[]> {
  const params = new URLSearchParams({ limit: '200', status });
  const response = await call(`/api/v1/admin/security/alerts?${params.toString()}`);
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить security alerts');
  const payload = await response.json() as { items: SecurityAlert[] };
  return payload.items;
}

export async function acknowledgeSecurityAlert(eventId: number, note: string) {
  const response = await call(`/api/v1/admin/security/alerts/${eventId}/ack`, {
    method: 'POST',
    body: JSON.stringify({ note })
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message ?? 'Не удалось подтвердить security alert');
  }
}

export async function getBetaReadiness(): Promise<BetaReadiness> {
  const response = await call('/api/v1/admin/beta/readiness');
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить readiness');
  return response.json() as Promise<BetaReadiness>;
}

export async function getProductMetrics(days: 7 | 30 | 90 = 7): Promise<ProductMetrics> {
  const response = await call(`/api/v1/admin/product/metrics?days=${days}`);
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить продуктовые метрики');
  return response.json() as Promise<ProductMetrics>;
}

export async function getSecuritySummary(): Promise<SecuritySummary> {
  const response = await call('/api/v1/admin/security/summary');
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить security summary');
  return response.json() as Promise<SecuritySummary>;
}

export async function getRuntimeSnapshot(): Promise<RuntimeSnapshot> {
  const response = await call('/api/v1/admin/ops/runtime');
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить runtime metrics');
  return response.json() as Promise<RuntimeSnapshot>;
}

export async function listPlatformFlags(): Promise<PlatformFlag[]> {
  const response = await call('/api/v1/admin/ops/flags');
  if (response.status === 404) throw new Error('Access denied');
  if (!response.ok) throw new Error('Не удалось загрузить operational controls');
  const payload = await response.json() as { items: PlatformFlag[] };
  return payload.items;
}

export async function setPlatformFlag(key: string, enabled: boolean, reason: string) {
  const response = await call(`/api/v1/admin/ops/flags/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: JSON.stringify({ enabled, reason })
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message ?? 'Не удалось изменить operational control');
  }
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
