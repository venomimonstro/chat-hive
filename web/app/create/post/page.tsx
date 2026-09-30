'use client';

import { FormEvent, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button } from '../../../components/ui';
import { getAccessToken, refreshSession } from '../../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type PostKind = 'thought' | 'post';

function limitFor(kind: PostKind) {
  return kind === 'thought' ? 700 : 8000;
}

export default function CreatePostPage() {
  const router = useRouter();
  const search = useSearchParams();
  const initialKind: PostKind = search.get('kind') === 'post' ? 'post' : 'thought';
  const [kind, setKind] = useState<PostKind>(initialKind);
  const [body, setBody] = useState('');
  const [visibility, setVisibility] = useState<'public' | 'followers'>('public');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const limit = useMemo(() => limitFor(kind), [kind]);

  async function publish(event: FormEvent) {
    event.preventDefault();
    const text = body.trim();
    if (!text || text.length > limit || busy) return;
    setBusy(true);
    setError('');
    try {
      let access = getAccessToken();
      if (!access) access = (await refreshSession())?.access_token ?? null;
      if (!access) {
        router.replace('/login');
        return;
      }
      let response = await fetch(`${API_BASE}/api/v1/posts`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json', Authorization: `Bearer ${access}` },
        body: JSON.stringify({ kind, body: text, visibility })
      });
      if (response.status === 401) {
        const refreshed = await refreshSession();
        if (refreshed) {
          response = await fetch(`${API_BASE}/api/v1/posts`, {
            method: 'POST', credentials: 'include',
            headers: { 'Content-Type': 'application/json', Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` },
            body: JSON.stringify({ kind, body: text, visibility })
          });
        }
      }
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(payload?.error?.message ?? 'Не удалось опубликовать');
      }
      const post = await response.json() as { id: string };
      router.replace(`/post/${post.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось опубликовать');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">СОЗДАТЬ</span><h1>{kind === 'thought' ? 'Мысль' : 'Пост'}</h1></div>
        <a href="/create" aria-label="Закрыть">✕</a>
      </header>
      <form className="postComposer" onSubmit={publish}>
        <div className="postKindSwitch" role="group" aria-label="Формат публикации">
          <button type="button" className={kind === 'thought' ? 'isActive' : ''} onClick={() => setKind('thought')}>Мысль</button>
          <button type="button" className={kind === 'post' ? 'isActive' : ''} onClick={() => setKind('post')}>Пост</button>
        </div>
        <textarea
          autoFocus
          value={body}
          onChange={(event) => setBody(event.target.value)}
          maxLength={limit}
          placeholder={kind === 'thought' ? 'Что думаете?' : 'Расскажите подробнее…'}
          aria-label="Текст публикации"
        />
        <div className="postComposerMeta"><span>{body.length} / {limit}</span></div>
        <label className="postAudience">
          <span>Кто увидит</span>
          <select value={visibility} onChange={(event) => setVisibility(event.target.value as 'public' | 'followers')}>
            <option value="public">Все</option>
            <option value="followers">Подписчики</option>
          </select>
        </label>
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        <Button type="submit" disabled={busy || body.trim().length === 0 || body.length > limit} fullWidth>
          {busy ? 'Публикуем…' : 'Опубликовать'}
        </Button>
      </form>
    </AppShell>
  );
}
