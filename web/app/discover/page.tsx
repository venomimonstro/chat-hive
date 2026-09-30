'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Avatar, Surface } from '../../components/ui';
import { getAccessToken, refreshSession } from '../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type FeedMode = 'for-you' | 'following';
type FeedItem = {
  id: string;
  author_id: string;
  author_username: string;
  author_name: string;
  kind: 'thought' | 'photo' | 'post';
  body: string;
  replies_count: number;
  reactions_count: number;
  created_at: string;
  reason: string;
};

async function loadFeed(mode: FeedMode): Promise<FeedItem[] | null> {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  let response = await fetch(`${API_BASE}/api/v1/feed?mode=${encodeURIComponent(mode)}&limit=30`, {
    credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${access}` }
  });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    response = await fetch(`${API_BASE}/api/v1/feed?mode=${encodeURIComponent(mode)}&limit=30`, {
      credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` }
    });
  }
  if (!response.ok) throw new Error('Не удалось загрузить ленту');
  const payload = await response.json() as { items: FeedItem[] };
  return payload.items;
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function DiscoverPage() {
  const router = useRouter();
  const [mode, setMode] = useState<FeedMode>('for-you');
  const [items, setItems] = useState<FeedItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    setError('');
    void loadFeed(mode)
      .then((feed) => {
        if (!feed) {
          router.replace('/login');
          return;
        }
        setItems(feed);
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [mode]);

  return (
    <AppShell active="Открыть">
      <header className="screenHeader discoverHeader">
        <div><span className="eyebrow">DISCOVER</span><h1>Открыть</h1></div>
        <a className="discoverSearch" href="/search" aria-label="Поиск">⌕</a>
      </header>
      <div className="discoverTabs" role="tablist" aria-label="Лента">
        <button role="tab" aria-selected={mode === 'for-you'} className={mode === 'for-you' ? 'isActive' : ''} onClick={() => setMode('for-you')}>Для тебя</button>
        <button role="tab" aria-selected={mode === 'following'} className={mode === 'following' ? 'isActive' : ''} onClick={() => setMode('following')}>Подписки</button>
      </div>
      <main className="discoverFeed">
        {loading ? <p className="chatListState">Подбираем публикации…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? (
          <Surface className="discoverEmpty"><strong>Лента пока тихая</strong><p>Подпишитесь на людей или создайте первую публикацию — CHAT начнёт строить ваш social graph.</p><a href="/create/post?kind=thought">Написать мысль</a></Surface>
        ) : null}
        {items.map((item) => (
          <article className="discoverCard" key={item.id}>
            <div className="discoverReason">{item.reason}</div>
            <a className="postAuthor" href={`/u/${item.author_username}`}>
              <Avatar name={item.author_name || item.author_username} />
              <div><strong>{item.author_name || item.author_username}</strong><span>@{item.author_username} · {formatDate(item.created_at)}</span></div>
            </a>
            <a className="discoverPostLink" href={`/post/${item.id}`}>
              <p>{item.body}</p>
            </a>
            <div className="postActions">
              <span>♡ {item.reactions_count}</span>
              <span>💬 {item.replies_count}</span>
              <a href={`/post/${item.id}`}>Обсудить</a>
            </div>
          </article>
        ))}
      </main>
    </AppShell>
  );
}
