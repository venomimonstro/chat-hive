'use client';

import { getAccessToken, refreshSession, setAccessToken } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

type BeginPayload = {
  ceremony_id: string;
  options: { publicKey: Record<string, any> };
};

export type PasskeyInfo = {
  credential_id: string;
  label: string;
  created_at: string;
  last_used_at?: string;
};

type SessionPayload = {
  user_id: string;
  session_id: string;
  access_token: string;
  access_expires_at: string;
};

function ensureSupported() {
  if (typeof window === 'undefined' || !window.PublicKeyCredential || !navigator.credentials) {
    throw new Error('Passkeys не поддерживаются этим браузером');
  }
}

function decodeBase64URL(value: string): ArrayBuffer {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes.buffer;
}

function encodeBase64URL(value: ArrayBuffer | ArrayBufferView | null | undefined): string {
  if (!value) return '';
  const view = ArrayBuffer.isView(value)
    ? new Uint8Array(value.buffer, value.byteOffset, value.byteLength)
    : new Uint8Array(value);
  let binary = '';
  for (const byte of view) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function creationOptions(input: Record<string, any>): PublicKeyCredentialCreationOptions {
  return {
    ...input,
    challenge: decodeBase64URL(input.challenge),
    user: { ...input.user, id: decodeBase64URL(input.user.id) },
    excludeCredentials: (input.excludeCredentials ?? []).map((item: any) => ({
      ...item,
      id: decodeBase64URL(item.id)
    }))
  } as PublicKeyCredentialCreationOptions;
}

function requestOptions(input: Record<string, any>): PublicKeyCredentialRequestOptions {
  return {
    ...input,
    challenge: decodeBase64URL(input.challenge),
    allowCredentials: (input.allowCredentials ?? []).map((item: any) => ({
      ...item,
      id: decodeBase64URL(item.id)
    }))
  } as PublicKeyCredentialRequestOptions;
}

async function parseError(response: Response) {
  try {
    const payload = await response.json();
    return payload?.error?.message ?? 'Passkey request failed';
  } catch {
    return 'Passkey request failed';
  }
}

async function authFetch(path: string, init: RequestInit, retry = true): Promise<Response> {
  let token = getAccessToken();
  if (!token) token = (await refreshSession())?.access_token ?? null;
  if (!token) throw new Error('Authentication required');

  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${token}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401 && retry) {
    const refreshed = await refreshSession();
    if (!refreshed) throw new Error('Authentication required');
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

function registrationJSON(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAttestationResponse & {
    getTransports?: () => string[];
    getAuthenticatorData?: () => ArrayBuffer;
    getPublicKey?: () => ArrayBuffer | null;
    getPublicKeyAlgorithm?: () => number;
  };
  return {
    id: credential.id,
    rawId: encodeBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: {
      clientDataJSON: encodeBase64URL(response.clientDataJSON),
      attestationObject: encodeBase64URL(response.attestationObject),
      transports: response.getTransports?.() ?? [],
      authenticatorData: encodeBase64URL(response.getAuthenticatorData?.()),
      publicKey: encodeBase64URL(response.getPublicKey?.()),
      publicKeyAlgorithm: response.getPublicKeyAlgorithm?.() ?? 0
    }
  };
}

function assertionJSON(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: encodeBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: {
      clientDataJSON: encodeBase64URL(response.clientDataJSON),
      authenticatorData: encodeBase64URL(response.authenticatorData),
      signature: encodeBase64URL(response.signature),
      ...(response.userHandle ? { userHandle: encodeBase64URL(response.userHandle) } : {})
    }
  };
}

export async function registerPasskey() {
  ensureSupported();
  const begin = await authFetch('/api/v1/auth/passkeys/register/begin', { method: 'POST' });
  if (!begin.ok) throw new Error(await parseError(begin));
  const payload = await begin.json() as BeginPayload;

  const credential = await navigator.credentials.create({
    publicKey: creationOptions(payload.options.publicKey)
  });
  if (!(credential instanceof PublicKeyCredential)) throw new Error('Создание passkey отменено');

  const finish = await authFetch(
    `/api/v1/auth/passkeys/register/finish/${encodeURIComponent(payload.ceremony_id)}`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(registrationJSON(credential))
    }
  );
  if (!finish.ok) throw new Error(await parseError(finish));
}

export async function loginWithPasskey(): Promise<SessionPayload> {
  ensureSupported();
  const begin = await fetch(`${API_BASE}/api/v1/auth/passkeys/login/begin`, {
    method: 'POST',
    headers: { Accept: 'application/json' },
    credentials: 'include',
    cache: 'no-store'
  });
  if (!begin.ok) throw new Error(await parseError(begin));
  const payload = await begin.json() as BeginPayload;

  const credential = await navigator.credentials.get({
    publicKey: requestOptions(payload.options.publicKey)
  });
  if (!(credential instanceof PublicKeyCredential)) throw new Error('Вход с passkey отменён');

  const finish = await fetch(
    `${API_BASE}/api/v1/auth/passkeys/login/finish/${encodeURIComponent(payload.ceremony_id)}`,
    {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      credentials: 'include',
      cache: 'no-store',
      body: JSON.stringify(assertionJSON(credential))
    }
  );
  if (!finish.ok) throw new Error(await parseError(finish));
  const session = await finish.json() as SessionPayload;
  setAccessToken(session.access_token);
  return session;
}


export async function listPasskeys(): Promise<PasskeyInfo[]> {
  const response = await authFetch('/api/v1/auth/passkeys', { method: 'GET' });
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { items: PasskeyInfo[] };
  return payload.items;
}

export async function deletePasskey(credentialId: string) {
  const response = await authFetch(
    `/api/v1/auth/passkeys/${encodeURIComponent(credentialId)}`,
    { method: 'DELETE' }
  );
  if (!response.ok && response.status !== 404) throw new Error(await parseError(response));
}
