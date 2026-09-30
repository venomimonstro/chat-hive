'use client';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
const ACCESS_KEY = 'chat_access_token';

type SessionPayload = {
  user_id: string;
  session_id: string;
  access_token: string;
  access_expires_at: string;
};

export type DeviceSession = {
  session_id: string;
  user_agent: string;
  last_ip: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
};

export type Interest = { slug: string; label_ru: string; label_en: string };
export type Profile = {
  user_id: string;
  username: string;
  display_name: string;
  bio: string;
  interests: string[];
  completed: boolean;
};
export type PublicProfile = {
  user_id: string;
  username: string;
  display_name: string;
  bio: string;
  interests: string[];
  followers_count: number;
  following_count: number;
  is_following: boolean;
  is_blocked: boolean;
  is_self: boolean;
};

export type ChatMessage = {
  id: string;
  chat_id: string;
  sender_id?: string;
  client_message_id: string;
  sequence: number;
  type: 'text' | 'image' | 'system';
  body: string;
  reply_to_id?: string;
  created_at: string;
  edited_at?: string;
  deleted_at?: string;
};

export type ChatSummary = {
  chat_id: string;
  kind: 'direct' | 'group';
  title: string;
  peer_username?: string;
  peer_display_name?: string;
  last_message?: ChatMessage;
  last_read_sequence: number;
  unread_count: number;
  updated_at: string;
};

export type Group = {
  chat_id: string;
  title: string;
  description: string;
  role: 'owner' | 'admin' | 'member';
  members_count: number;
};

export type GroupMember = {
  user_id: string;
  username: string;
  display_name: string;
  role: 'owner' | 'admin' | 'member';
};

export function getAccessToken() {
  if (typeof window === 'undefined') return null;
  return window.sessionStorage.getItem(ACCESS_KEY);
}

export function setAccessToken(token: string | null) {
  if (typeof window === 'undefined') return;
  if (token) window.sessionStorage.setItem(ACCESS_KEY, token);
  else window.sessionStorage.removeItem(ACCESS_KEY);
}

async function parseError(response: Response) {
  try {
    const body = await response.json();
    return body?.error?.message ?? 'Request failed';
  } catch {
    return 'Request failed';
  }
}

async function request(path: string, init: RequestInit = {}, retry = true): Promise<Response> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  const access = getAccessToken();
  if (access) headers.set('Authorization', `Bearer ${access}`);

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
    cache: 'no-store'
  });

  if (response.status === 401 && retry && path !== '/api/v1/auth/refresh') {
    const refreshed = await refreshSession();
    if (refreshed) return request(path, init, false);
  }
  return response;
}

export async function startEmailLogin(email: string) {
  const response = await request('/api/v1/auth/email/start', { method: 'POST', body: JSON.stringify({ email }) }, false);
  if (!response.ok) throw new Error(await parseError(response));
}

export async function completeEmailLogin(token: string): Promise<SessionPayload> {
  const response = await request('/api/v1/auth/email/complete', { method: 'POST', body: JSON.stringify({ token }) }, false);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as SessionPayload;
  setAccessToken(payload.access_token);
  return payload;
}

export async function startYandexLogin(): Promise<string> {
  const response = await request('/api/v1/auth/yandex/start', {}, false);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { authorization_url: string };
  return payload.authorization_url;
}

export async function refreshSession(): Promise<SessionPayload | null> {
  const response = await fetch(`${API_BASE}/api/v1/auth/refresh`, {
    method: 'POST', credentials: 'include', headers: { Accept: 'application/json' }, cache: 'no-store'
  });
  if (!response.ok) {
    setAccessToken(null);
    return null;
  }
  const payload = await response.json() as SessionPayload;
  setAccessToken(payload.access_token);
  return payload;
}

export async function getCurrentSession() {
  const response = await request('/api/v1/auth/session');
  if (!response.ok) return null;
  return response.json() as Promise<{ user_id: string; session_id: string; expires_at: string }>;
}

export async function listSessions(): Promise<{ items: DeviceSession[]; current_session_id: string }> {
  const response = await request('/api/v1/auth/sessions');
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<{ items: DeviceSession[]; current_session_id: string }>;
}

export async function revokeSession(sessionId: string, isCurrent = false) {
  const response = await request(`/api/v1/auth/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' });
  if (!response.ok && response.status !== 404) throw new Error(await parseError(response));
  if (isCurrent) setAccessToken(null);
}

export async function listInterests(): Promise<Interest[]> {
  const response = await request('/api/v1/onboarding/interests', {}, false);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { items: Interest[] };
  return payload.items;
}

export async function getProfile(): Promise<Profile | null> {
  const response = await request('/api/v1/me/profile');
  if (!response.ok) return null;
  return response.json() as Promise<Profile>;
}

export async function completeOnboarding(input: { username: string; display_name: string; bio: string; interests: string[] }): Promise<Profile> {
  const response = await request('/api/v1/me/onboarding', { method: 'PUT', body: JSON.stringify(input) });
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<Profile>;
}

export async function getPublicProfile(username: string): Promise<PublicProfile> {
  const response = await request(`/api/v1/profiles/${encodeURIComponent(username)}`);
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<PublicProfile>;
}

export async function setFollow(username: string, follow: boolean) {
  const response = await request(`/api/v1/profiles/${encodeURIComponent(username)}/follow`, { method: follow ? 'POST' : 'DELETE' });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function setBlock(username: string, block: boolean) {
  const response = await request(`/api/v1/profiles/${encodeURIComponent(username)}/block`, { method: block ? 'POST' : 'DELETE' });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function listChats(before = ''): Promise<ChatSummary[]> {
  const query = before ? `?before=${encodeURIComponent(before)}` : '';
  const response = await request(`/api/v1/chats${query}`);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { items: ChatSummary[] };
  return payload.items;
}

export async function ensureDirectChat(username: string): Promise<ChatSummary> {
  const response = await request('/api/v1/chats/direct', { method: 'POST', body: JSON.stringify({ username }) });
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<ChatSummary>;
}

export async function listMessages(chatId: string, beforeSequence?: number): Promise<ChatMessage[]> {
  const query = beforeSequence ? `?before_sequence=${beforeSequence}` : '';
  const response = await request(`/api/v1/chats/${encodeURIComponent(chatId)}/messages${query}`);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { items: ChatMessage[] };
  return payload.items;
}

export async function sendTextMessage(chatId: string, text: string, clientMessageId = crypto.randomUUID(), replyToId = ''): Promise<{ message: ChatMessage; duplicate: boolean }> {
  const response = await request(`/api/v1/chats/${encodeURIComponent(chatId)}/messages`, {
    method: 'POST',
    body: JSON.stringify({ client_message_id: clientMessageId, text, reply_to_id: replyToId })
  });
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<{ message: ChatMessage; duplicate: boolean }>;
}

export async function markChatRead(chatId: string, sequence: number) {
  const response = await request(`/api/v1/chats/${encodeURIComponent(chatId)}/read`, {
    method: 'POST', body: JSON.stringify({ sequence })
  });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function createGroup(input: { title: string; description: string }): Promise<Group> {
  const response = await request('/api/v1/groups', { method: 'POST', body: JSON.stringify(input) });
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<Group>;
}

export async function getGroup(chatId: string): Promise<Group> {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}`);
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<Group>;
}

export async function listGroupMembers(chatId: string): Promise<GroupMember[]> {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}/members`);
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { items: GroupMember[] };
  return payload.items;
}

export async function setGroupMemberRole(chatId: string, username: string, role: 'admin' | 'member') {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}/members/${encodeURIComponent(username)}/role`, { method: 'PUT', body: JSON.stringify({ role }) });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function removeGroupMember(chatId: string, username: string) {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}/members/${encodeURIComponent(username)}`, { method: 'DELETE' });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function leaveGroup(chatId: string) {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}/leave`, { method: 'POST' });
  if (!response.ok) throw new Error(await parseError(response));
}

export async function createGroupInvite(chatId: string, ttlHours = 168, maxUses = 0): Promise<string> {
  const response = await request(`/api/v1/groups/${encodeURIComponent(chatId)}/invites`, { method: 'POST', body: JSON.stringify({ ttl_hours: ttlHours, max_uses: maxUses }) });
  if (!response.ok) throw new Error(await parseError(response));
  const payload = await response.json() as { token: string };
  return payload.token;
}

export async function joinGroupByInvite(token: string): Promise<Group> {
  const response = await request('/api/v1/groups/join', { method: 'POST', body: JSON.stringify({ token }) });
  if (!response.ok) throw new Error(await parseError(response));
  return response.json() as Promise<Group>;
}
