'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { AppShell } from '../../../../components/AppShell';
import { Avatar, Surface } from '../../../../components/ui';
import { getAccessToken, refreshSession } from '../../../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type Connection = {
  username: string;
  display_name: string;
  bio: string;
  followers_count: number;
  shared_interests: number;
  is_following: boolean;
};

async function loadConnections(username: string, mode: 'followers' | 'following'): Promise<Connection[] | null> {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (access) headers.Authorization = `Bearer ${access}`;
  let response = await fetch(`${API_BASE}/api/v1/profiles/${encodeURIComponent(username)}/${mode}?limit=100`, {
    credentials: 'include', cache: 'no-store', headers
  });
  if (response.status === 401 && access) {
    const refreshed = await refreshSession();
    if (refreshed) {
      response = await fetch(`${API_BASE}/api/v1/profiles/${encodeURIComponent(username)}/${mode}?limit=100`, {
        credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` }
      });
    }
  }
  if (!response.ok) throw new Error('Не удалось загрузить список');
  const payload = await response.json() as { items: Connection[] };
  return payload.items;
}

export default function ConnectionsPage() {
  const params = useParams<{ username: string }>();
  const search = useSearchParams();
  const router = useRouter();
  const initial = search.get('tab') === 'following' ? 'following' : 'followers';
  const [mode, setMode] = useState<'followers' | 'following'>(initial);
  const [items, setItems] = useState<Connection[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    setError('');
    const url = new URL(window.location.href);
    url.searchParams.set('tab', mode);
    window.history.replaceState({}, '', `${url.pathname}${url.search}`);
    void loadConnections(params.username, mode)
      .then(setItems)
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [mode, params.username]);

  return (
    <AppShell active="Я">
      <header className="screenHeader">
        <div><span className="eyebrow">@{params.username}</span><h1>Связи</h1></div>
        <button className="connectionsClose" type="button" onClick={() => router.back()} aria-label="Назад">✕</button>
      </header>
      <div className="discoverTabs connectionsTabs" role="tablist">
        <button role="tab" aria-selected={mode === 'followers'} className={mode === 'followers' ? 'isActive' : ''} onClick={() => setMode('followers')}>Подписчики</button>
        <button role="tab" aria-selected={mode === 'following'} className={mode === 'following' ? 'isActive' : ''} onClick={() => setMode('following')}>Подписки</button>
      </div>
      <main className="connectionsList">
        {loading ? <p className="chatListState">Загружаем…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? <Surface className="searchEmpty"><strong>Список пока пуст</strong></Surface> : null}
        {items.map((item) => (
          <a className="connectionRow" href={`/u/${item.username}`} key={item.username}>
            <Avatar name={item.display_name || item.username} />
            <div><strong>{item.display_name || item.username}</strong><span>@{item.username} · {item.followers_count} подписчиков</span>{item.bio ? <p>{item.bio}</p> : null}{item.shared_interests > 0 ? <small>{item.shared_interests} общих интереса</small> : null}</div>
          </a>
        ))}
      </main>
    </AppShell>
  );
}
