'use client';

const KEY = 'chat:return-to';

function normalize(value: string | null | undefined) {
  if (!value) return '';
  const trimmed = value.trim();
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) return '';
  try {
    const parsed = new URL(trimmed, 'https://chat.local');
    if (parsed.origin !== 'https://chat.local') return '';
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return '';
  }
}

export function rememberReturnTo(value: string | null | undefined) {
  if (typeof window === 'undefined') return;
  const safe = normalize(value);
  if (safe) sessionStorage.setItem(KEY, safe);
}

export function peekReturnTo() {
  if (typeof window === 'undefined') return '';
  return normalize(sessionStorage.getItem(KEY));
}

export function takeReturnTo(fallback = '/') {
  if (typeof window === 'undefined') return fallback;
  const safe = normalize(sessionStorage.getItem(KEY));
  sessionStorage.removeItem(KEY);
  return safe || fallback;
}
