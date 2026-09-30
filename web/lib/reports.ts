'use client';

import { getAccessToken, refreshSession } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

export type ReportTargetType = 'user' | 'message' | 'post' | 'group';
export type ReportCategory = 'spam' | 'scam' | 'harassment' | 'threats' | 'illegal_content' | 'sexual_content' | 'impersonation' | 'malware' | 'other';

export async function submitReport(input: { target_type: ReportTargetType; target_id: string; category: ReportCategory; note: string }) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) throw new Error('Authentication required');
  const body = JSON.stringify(input);
  let response = await fetch(`${API_BASE}/api/v1/reports`, {
    method: 'POST', credentials: 'include', cache: 'no-store',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', Authorization: `Bearer ${access}` }, body
  });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Authentication required');
    response = await fetch(`${API_BASE}/api/v1/reports`, {
      method: 'POST', credentials: 'include', cache: 'no-store',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json', Authorization: `Bearer ${refreshed.access_token}` }, body
    });
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message ?? 'Не удалось отправить жалобу');
  }
  return response.json() as Promise<{ id: string }>;
}
