'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Button, Surface } from '../../components/ui';
import { getAccessToken, refreshSession } from '../../lib/api';
import { disablePush, enablePush, pushAvailable } from '../../lib/push';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type NotificationItem = {
  id: string;
  kind: string;
  actor_id?: string;
  entity_type: string;
  entity_id: string;
  title: string;
  body: string;
  read_at?: string;
  created_at: string;
};

async function authFetch(path: string, init: RequestInit = {}) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${access}`);
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

function notificationHref(item: NotificationItem) {
  if (item.entity_type === 'chat') return `/?chat=${encodeURIComponent(item.entity_id)}`;
  if (item.entity_type === 'post') return `/post/${encodeURIComponent(item.entity_id)}`;
  if (item.entity_type === 'channel') return `/channels/${encodeURIComponent(item.entity_id)}`;
  if (item.entity_type === 'user') return `/u/id/${encodeURIComponent(item.entity_id)}`;
  return '/';
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function NotificationsPage() {
  const router = useRouter();
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [unread, setUnread] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pushBusy, setPushBusy] = useState(false);
  const [pushEnabled, setPushEnabled] = useState(false);
  const canPush = useMemo(() => pushAvailable(), []);

  async function load() {
    const response = await authFetch('/api/v1/notifications?limit=100');
    if (!response) { router.replace('/login'); return; }
    if (!response.ok) throw new Error('Не удалось загрузить уведомления');
    const payload = await response.json() as { items: NotificationItem[]; unread_count: number };
    setItems(payload.items);
    setUnread(payload.unread_count);
  }

  useEffect(() => {
    void load().catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки')).finally(() => setLoading(false));
    if ('serviceWorker' in navigator) {
      void navigator.serviceWorker.ready.then((registration) => registration.pushManager.getSubscription()).then((subscription) => setPushEnabled(Boolean(subscription))).catch(() => undefined);
    }
  }, []);

  async function markRead(item: NotificationItem) {
    if (item.read_at) return;
    const response = await authFetch(`/api/v1/notifications/${encodeURIComponent(item.id)}/read`, { method: 'PUT' });
    if (response?.ok) {
      const now = new Date().toISOString();
      setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, read_at: now } : entry));
      setUnread((value) => Math.max(0, value - 1));
    }
  }

  async function markAll() {
    const response = await authFetch('/api/v1/notifications/read-all', { method: 'PUT' });
    if (!response?.ok) return;
    const now = new Date().toISOString();
    setItems((current) => current.map((entry) => entry.read_at ? entry : { ...entry, read_at: now }));
    setUnread(0);
  }

  async function togglePush() {
    if (!canPush || pushBusy) return;
    setPushBusy(true);
    setError('');
    try {
      if (pushEnabled) {
        await disablePush();
        setPushEnabled(false);
      } else {
        await enablePush();
        setPushEnabled(true);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось изменить push-настройку');
    } finally {
      setPushBusy(false);
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">СОБЫТИЯ</span><h1>Уведомления{unread ? ` · ${unread}` : ''}</h1></div>
        <a href="/" aria-label="Назад">←</a>
      </header>
      <main className="notificationPage">
        <div className="notificationToolbar">
          <button type="button" onClick={() => void markAll()} disabled={unread === 0}>Прочитать всё</button>
          {canPush ? <Button type="button" onClick={() => void togglePush()} disabled={pushBusy}>{pushBusy ? 'Сохраняем…' : pushEnabled ? 'Отключить push' : 'Включить push'}</Button> : null}
        </div>
        {loading ? <p className="chatListState">Загружаем уведомления…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && items.length === 0 ? <Surface className="notificationEmpty"><strong>Пока тихо</strong><p>Новые сообщения, подписчики, ответы и публикации каналов появятся здесь.</p></Surface> : null}
        <section className="notificationList" aria-label="Уведомления">
          {items.map((item) => (
            <a key={item.id} className={`notificationItem ${item.read_at ? '' : 'isUnread'}`} href={notificationHref(item)} onClick={() => void markRead(item)}>
              <span className="notificationDot" aria-hidden="true" />
              <div><strong>{item.title}</strong>{item.body ? <p>{item.body}</p> : null}<time>{formatDate(item.created_at)}</time></div>
            </a>
          ))}
        </section>
      </main>
    </AppShell>
  );
}
