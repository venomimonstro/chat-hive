'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Avatar, Surface } from '../../components/ui';
import { getAccessToken, refreshSession } from '../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type SearchResults = {
  people: Array<{ username: string; display_name: string; bio: string; followers_count: number; shared_interests: number }>;
  posts: Array<{ id: string; author_username: string; author_name: string; kind: string; body: string }>;
  groups: Array<{ chat_id: string; title: string; description: string; members_count: number; is_member: boolean }>;
};

async function runSearch(query: string): Promise<SearchResults | null> {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  const url = `${API_BASE}/api/v1/search?q=${encodeURIComponent(query)}&limit=12`;
  let response = await fetch(url, { credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${access}` } });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    response = await fetch(url, { credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` } });
  }
  if (!response.ok) throw new Error('Не удалось выполнить поиск');
  return response.json() as Promise<SearchResults>;
}

export default function SearchPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [query, setQuery] = useState(searchParams.get('q') ?? '');
  const [results, setResults] = useState<SearchResults>({ people: [], posts: [], groups: [] });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const normalized = query.trim();
    const timer = window.setTimeout(() => {
      if (normalized.length < 2) {
        setResults({ people: [], posts: [], groups: [] });
        setError('');
        return;
      }
      setLoading(true);
      setError('');
      const url = new URL(window.location.href);
      url.searchParams.set('q', normalized);
      window.history.replaceState({}, '', `${url.pathname}${url.search}`);
      void runSearch(normalized)
        .then((payload) => {
          if (!payload) {
            router.replace('/login');
            return;
          }
          setResults(payload);
        })
        .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка поиска'))
        .finally(() => setLoading(false));
    }, 300);
    return () => window.clearTimeout(timer);
  }, [query, router]);

  const hasResults = results.people.length + results.posts.length + results.groups.length > 0;

  return (
    <AppShell active="Открыть">
      <header className="screenHeader">
        <div><span className="eyebrow">ПОИСК</span><h1>Найти в CHAT</h1></div>
        <a href="/discover" aria-label="Назад">✕</a>
      </header>
      <main className="searchPage">
        <div className="globalSearchBox"><span>⌕</span><input autoFocus value={query} onChange={(event) => setQuery(event.target.value)} maxLength={100} placeholder="Люди, публикации, группы" aria-label="Поиск" /></div>
        {query.trim().length < 2 ? <p className="searchHint">Введите минимум 2 символа.</p> : null}
        {loading ? <p className="chatListState">Ищем…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && query.trim().length >= 2 && !error && !hasResults ? <Surface className="searchEmpty"><strong>Ничего не найдено</strong><p>Попробуйте другой запрос или проверьте написание.</p></Surface> : null}

        {results.people.length > 0 ? <section className="searchSection"><h2>Люди</h2>{results.people.map((person) => <a className="searchPerson" href={`/u/${person.username}`} key={person.username}><Avatar name={person.display_name || person.username}/><div><strong>{person.display_name || person.username}</strong><span>@{person.username} · {person.followers_count} подписчиков</span>{person.bio ? <p>{person.bio}</p> : null}{person.shared_interests > 0 ? <small>{person.shared_interests} общих интереса</small> : null}</div></a>)}</section> : null}

        {results.posts.length > 0 ? <section className="searchSection"><h2>Публикации</h2>{results.posts.map((post) => <a className="searchPost" href={`/post/${post.id}`} key={post.id}><div><strong>{post.author_name || post.author_username}</strong><span>@{post.author_username}</span></div><p>{post.body}</p></a>)}</section> : null}

        {results.groups.length > 0 ? <section className="searchSection"><h2>Мои группы</h2>{results.groups.map((group) => <a className="searchGroup" href={`/?chat=${encodeURIComponent(group.chat_id)}`} key={group.chat_id}><div><strong>{group.title}</strong><span>{group.members_count} участников</span></div>{group.description ? <p>{group.description}</p> : null}</a>)}</section> : null}
      </main>
    </AppShell>
  );
}
