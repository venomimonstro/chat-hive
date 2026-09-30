'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { AuthenticatedImage } from '../../../components/AuthenticatedImage';
import { Avatar, Button, Surface } from '../../../components/ui';
import { getAccessToken, refreshSession } from '../../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type MediaRef = { id: string; mime_type: string; width: number; height: number; url: string };
type Post = {
  id: string;
  author_id: string;
  author_username: string;
  author_name: string;
  kind: 'thought' | 'photo' | 'post';
  body: string;
  visibility: 'public' | 'followers';
  media: MediaRef[];
  replies_count: number;
  reactions_count: number;
  created_at: string;
  updated_at: string;
  mine: boolean;
  saved: boolean;
};

type Reply = {
  id: string;
  post_id: string;
  author_username: string;
  author_name: string;
  body: string;
  created_at: string;
  mine: boolean;
};

async function publicRead(path: string) {
  const access = getAccessToken();
  const headers = new Headers({ Accept: 'application/json' });
  if (access) headers.set('Authorization', `Bearer ${access}`);
  let response = await fetch(`${API_BASE}${path}`, { headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401 && access) {
    const refreshed = await refreshSession();
    const retryHeaders = new Headers({ Accept: 'application/json' });
    if (refreshed) retryHeaders.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { headers: retryHeaders, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

async function authenticatedFetch(path: string, init: RequestInit = {}) {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  headers.set('Authorization', `Bearer ${access}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  let response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    headers.set('Authorization', `Bearer ${refreshed.access_token}`);
    response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include', cache: 'no-store' });
  }
  return response;
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export function PostClient({ postId }: { postId: string }) {
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);
  const [replies, setReplies] = useState<Reply[]>([]);
  const [reply, setReply] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [reactionPending, setReactionPending] = useState(false);
  const [reactedLocally, setReactedLocally] = useState(false);
  const [error, setError] = useState('');

  function requireLogin() {
    const next = typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : `/post/${postId}`;
    router.push(`/login?next=${encodeURIComponent(next)}`);
  }

  async function load() {
    setError('');
    const [postResponse, repliesResponse] = await Promise.all([
      publicRead(`/api/v1/posts/${encodeURIComponent(postId)}`),
      publicRead(`/api/v1/posts/${encodeURIComponent(postId)}/replies?limit=100`)
    ]);
    if (!postResponse.ok) throw new Error('Публикация недоступна');
    if (!repliesResponse.ok) throw new Error('Не удалось загрузить ответы');
    const postPayload = await postResponse.json() as Post;
    const repliesPayload = await repliesResponse.json() as { items: Reply[] };
    setPost({ ...postPayload, media: postPayload.media ?? [] });
    setReplies(repliesPayload.items);
  }

  useEffect(() => {
    void load().catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки')).finally(() => setLoading(false));
  }, [postId]);

  async function submitReply(event: FormEvent) {
    event.preventDefault();
    const text = reply.trim();
    if (!text || busy) return;
    setBusy(true); setError('');
    try {
      const response = await authenticatedFetch(`/api/v1/posts/${encodeURIComponent(postId)}/replies`, { method: 'POST', body: JSON.stringify({ body: text }) });
      if (!response) { requireLogin(); return; }
      if (!response.ok) throw new Error('Не удалось отправить ответ');
      const created = await response.json() as Reply;
      setReplies((current) => [...current, created]);
      setPost((current) => current ? { ...current, replies_count: current.replies_count + 1 } : current);
      setReply('');
    } catch (err) { setError(err instanceof Error ? err.message : 'Не удалось отправить ответ'); }
    finally { setBusy(false); }
  }

  async function react() {
    if (!post || reactionPending || reactedLocally) return;
    setReactionPending(true);
    try {
      const response = await authenticatedFetch(`/api/v1/posts/${encodeURIComponent(post.id)}/reactions/${encodeURIComponent('❤️')}`, { method: 'PUT' });
      if (!response) { requireLogin(); return; }
      if (!response.ok) throw new Error('Не удалось поставить реакцию');
      setReactedLocally(true);
      setPost((current) => current ? { ...current, reactions_count: current.reactions_count + 1 } : current);
    } catch (err) { setError(err instanceof Error ? err.message : 'Не удалось поставить реакцию'); }
    finally { setReactionPending(false); }
  }

  async function toggleSave() {
    if (!post) return;
    const next = !post.saved;
    const response = await authenticatedFetch(`/api/v1/posts/${encodeURIComponent(post.id)}/saved`, { method: next ? 'PUT' : 'DELETE' });
    if (!response) { requireLogin(); return; }
    if (response.ok) setPost({ ...post, saved: next });
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">ПУБЛИКАЦИЯ</span><h1>{post?.kind === 'thought' ? 'Мысль' : post?.kind === 'photo' ? 'Фото' : 'Пост'}</h1></div><a href="/discover" aria-label="Назад">←</a></header>
      <main className="postPageBody">
        {loading ? <p className="chatListState">Загружаем…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {post ? <Surface className="postCard">
          <a className="postAuthor" href={`/u/${post.author_username}`}><Avatar name={post.author_name || post.author_username} /><div><strong>{post.author_name || post.author_username}</strong><span>@{post.author_username} · {formatDate(post.created_at)}</span></div></a>
          {post.media.length ? <div className={`postMediaGrid ${post.media.length === 1 ? 'single' : ''}`}>{post.media.map((item) => <AuthenticatedImage key={item.id} className="postMediaImage" src={item.url} alt="Изображение публикации" />)}</div> : null}
          {post.body ? <p className="postBody">{post.body}</p> : null}
          <div className="postActions"><button type="button" onClick={react} disabled={reactionPending || reactedLocally}>{reactedLocally ? '♥' : '♡'} {post.reactions_count}</button><span>💬 {post.replies_count}</span><button type="button" onClick={toggleSave}>{post.saved ? 'Сохранено' : 'Сохранить'}</button></div>
        </Surface> : null}
        {post ? <section className="replySection">
          <h2>Ответы</h2>
          <form className="replyForm" onSubmit={submitReply}><textarea value={reply} onChange={(event) => setReply(event.target.value)} maxLength={2000} placeholder="Ответить по делу…" aria-label="Ответ" /><Button type="submit" disabled={busy || !reply.trim()}>{busy ? 'Отправляем…' : 'Ответить'}</Button></form>
          <div className="replyList">{replies.length === 0 ? <p className="chatListState">Пока без ответов. Начните разговор.</p> : null}{replies.map((item) => <article className="replyCard" key={item.id}><Avatar name={item.author_name || item.author_username} size="sm" /><div><a href={`/u/${item.author_username}`}><strong>{item.author_name || item.author_username}</strong></a><span>@{item.author_username} · {formatDate(item.created_at)}</span><p>{item.body}</p></div></article>)}</div>
        </section> : null}
      </main>
    </AppShell>
  );
}
