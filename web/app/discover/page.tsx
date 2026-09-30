'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { AuthenticatedImage } from '../../components/AuthenticatedImage';
import { Avatar, Surface } from '../../components/ui';
import { getAccessToken, refreshSession } from '../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type DiscoverMode = 'for-you' | 'following' | 'people';
type FeedbackSignal = 'more_like_this' | 'not_interested' | 'hide';
type MediaRef = { id: string; mime_type: string; width: number; height: number; url: string };
type FeedItem = {
  id: string;
  author_id: string;
  author_username: string;
  author_name: string;
  kind: 'thought' | 'photo' | 'post';
  body: string;
  cover?: MediaRef;
  replies_count: number;
  reactions_count: number;
  created_at: string;
  reason: string;
};
type PersonRecommendation = {
  username: string;
  display_name: string;
  bio: string;
  followers_count: number;
  shared_interests: number;
  mutual_connections: number;
  reason: string;
};

async function authenticatedFetch(path: string, init: RequestInit = {}) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${access}`);
  if (init.body) headers.set('Content-Type', 'application/json');
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

async function loadFeed(mode: Exclude<DiscoverMode, 'people'>): Promise<FeedItem[] | null> {
  const response = await authenticatedFetch(`/api/v1/feed?mode=${encodeURIComponent(mode)}&limit=30`);
  if (!response) return null;
  if (!response.ok) throw new Error('Не удалось загрузить ленту');
  const payload = await response.json() as { items: FeedItem[] };
  return payload.items;
}

async function loadPeople(): Promise<PersonRecommendation[] | null> {
  const response = await authenticatedFetch('/api/v1/discovery/people?limit=40');
  if (!response) return null;
  if (!response.ok) throw new Error('Не удалось подобрать людей');
  const payload = await response.json() as { items: PersonRecommendation[] };
  return payload.items;
}

async function sendFeedback(postID: string, signal: FeedbackSignal) {
  const response = await authenticatedFetch(`/api/v1/feed/${encodeURIComponent(postID)}/feedback`, {
    method: 'PUT', body: JSON.stringify({ signal })
  });
  if (!response) throw new Error('Сессия завершена');
  if (!response.ok) throw new Error('Не удалось сохранить настройку ленты');
}

async function follow(username: string) {
  const response = await authenticatedFetch(`/api/v1/profiles/${encodeURIComponent(username)}/follow`, { method: 'POST' });
  if (!response) throw new Error('Сессия завершена');
  if (!response.ok) throw new Error('Не удалось подписаться');
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function DiscoverPage() {
  const router = useRouter();
  const [mode, setMode] = useState<DiscoverMode>('for-you');
  const [items, setItems] = useState<FeedItem[]>([]);
  const [people, setPeople] = useState<PersonRecommendation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [feedbackOpen, setFeedbackOpen] = useState<string | null>(null);
  const [feedbackBusy, setFeedbackBusy] = useState<string | null>(null);
  const [followBusy, setFollowBusy] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError('');
    const task = mode === 'people'
      ? loadPeople().then((result) => {
          if (!result) { router.replace('/login'); return; }
          setPeople(result);
        })
      : loadFeed(mode).then((result) => {
          if (!result) { router.replace('/login'); return; }
          setItems(result);
        });
    void task.catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки')).finally(() => setLoading(false));
  }, [mode]);

  async function applyFeedback(item: FeedItem, signal: FeedbackSignal) {
    if (feedbackBusy) return;
    setFeedbackBusy(item.id);
    setError('');
    try {
      await sendFeedback(item.id, signal);
      if (signal === 'hide' || signal === 'not_interested') {
        setItems((current) => current.filter((entry) => entry.id !== item.id));
      } else {
        setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, reason: 'Вы попросили больше такого' } : entry));
      }
      setFeedbackOpen(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось сохранить настройку');
    } finally {
      setFeedbackBusy(null);
    }
  }

  async function followPerson(person: PersonRecommendation) {
    if (followBusy) return;
    setFollowBusy(person.username);
    setError('');
    try {
      await follow(person.username);
      setPeople((current) => current.filter((item) => item.username !== person.username));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось подписаться');
    } finally {
      setFollowBusy(null);
    }
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader discoverHeader">
        <div><span className="eyebrow">DISCOVER</span><h1>Открыть</h1></div>
        <a className="discoverSearch" href="/search" aria-label="Поиск">⌕</a>
      </header>
      <div className="discoverTabs discoverTabs--three" role="tablist" aria-label="Открыть">
        <button role="tab" aria-selected={mode === 'for-you'} className={mode === 'for-you' ? 'isActive' : ''} onClick={() => setMode('for-you')}>Для тебя</button>
        <button role="tab" aria-selected={mode === 'people'} className={mode === 'people' ? 'isActive' : ''} onClick={() => setMode('people')}>Люди</button>
        <button role="tab" aria-selected={mode === 'following'} className={mode === 'following' ? 'isActive' : ''} onClick={() => setMode('following')}>Подписки</button>
      </div>
      <main className="discoverFeed">
        {loading ? <p className="chatListState">{mode === 'people' ? 'Ищем людей, с которыми вам может быть интересно…' : 'Подбираем публикации…'}</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}

        {mode === 'people' ? (
          <>
            {!loading && !error && people.length === 0 ? <Surface className="discoverEmpty"><strong>Новых рекомендаций пока нет</strong><p>Добавьте интересы в профиль и участвуйте в сообществах — подбор станет точнее.</p></Surface> : null}
            <section className="peopleDiscovery" aria-label="Рекомендуемые люди">
              {people.map((person) => (
                <article className="peopleCard" key={person.username}>
                  <a className="peopleCard__identity" href={`/u/${person.username}`}>
                    <Avatar name={person.display_name || person.username} />
                    <div><strong>{person.display_name || person.username}</strong><span>@{person.username}</span></div>
                  </a>
                  {person.bio ? <p>{person.bio}</p> : null}
                  <div className="peopleCard__meta">
                    <span>{person.reason}</span>
                    {person.shared_interests > 0 ? <span>{person.shared_interests} общих интереса</span> : null}
                    {person.mutual_connections > 0 ? <span>{person.mutual_connections} общих контакта</span> : null}
                  </div>
                  <button type="button" disabled={followBusy === person.username} onClick={() => void followPerson(person)}>{followBusy === person.username ? 'Подписываемся…' : 'Подписаться'}</button>
                </article>
              ))}
            </section>
          </>
        ) : (
          <>
            {!loading && !error && items.length === 0 ? (
              <Surface className="discoverEmpty"><strong>Лента пока тихая</strong><p>Подпишитесь на людей или создайте первую публикацию — CHAT начнёт строить ваш social graph.</p><a href="/create/post?kind=thought">Написать мысль</a></Surface>
            ) : null}
            {items.map((item) => (
              <article className="discoverCard" key={item.id}>
                <div className="discoverReasonRow">
                  <span className="discoverReason">{item.reason}</span>
                  {mode === 'for-you' ? (
                    <div className="discoverFeedback">
                      <button type="button" aria-label="Настроить рекомендацию" onClick={() => setFeedbackOpen((value) => value === item.id ? null : item.id)}>•••</button>
                      {feedbackOpen === item.id ? (
                        <div className="discoverFeedbackMenu">
                          <button type="button" disabled={feedbackBusy === item.id} onClick={() => void applyFeedback(item, 'more_like_this')}>Больше такого</button>
                          <button type="button" disabled={feedbackBusy === item.id} onClick={() => void applyFeedback(item, 'not_interested')}>Меньше такого</button>
                          <button type="button" disabled={feedbackBusy === item.id} onClick={() => void applyFeedback(item, 'hide')}>Скрыть публикацию</button>
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <a className="postAuthor" href={`/u/${item.author_username}`}>
                  <Avatar name={item.author_name || item.author_username} />
                  <div><strong>{item.author_name || item.author_username}</strong><span>@{item.author_username} · {formatDate(item.created_at)}</span></div>
                </a>
                <a className="discoverPostLink" href={`/post/${item.id}`}>
                  {item.cover ? <AuthenticatedImage className="discoverCover" src={item.cover.url} alt="Изображение публикации" /> : null}
                  {item.body ? <p>{item.body}</p> : null}
                </a>
                <div className="postActions">
                  <span>♡ {item.reactions_count}</span>
                  <span>💬 {item.replies_count}</span>
                  <a href={`/post/${item.id}`}>Обсудить</a>
                </div>
              </article>
            ))}
          </>
        )}
      </main>
    </AppShell>
  );
}
