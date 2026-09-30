'use client';

import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button } from '../../../components/ui';
import { getAccessToken, refreshSession } from '../../../lib/api';
import { MediaObject, uploadImage } from '../../../lib/media';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
type PostKind = 'thought' | 'photo' | 'post';
type SelectedMedia = { object: MediaObject; preview: string };

function limitFor(kind: PostKind) {
  if (kind === 'thought') return 700;
  if (kind === 'photo') return 2000;
  return 8000;
}

export default function CreatePostPage() {
  const router = useRouter();
  const search = useSearchParams();
  const rawKind = search.get('kind');
  const initialKind: PostKind = rawKind === 'post' || rawKind === 'photo' ? rawKind : 'thought';
  const [kind, setKind] = useState<PostKind>(initialKind);
  const [body, setBody] = useState('');
  const [visibility, setVisibility] = useState<'public' | 'followers'>('public');
  const [media, setMedia] = useState<SelectedMedia[]>([]);
  const [uploading, setUploading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const limit = useMemo(() => limitFor(kind), [kind]);

  function changeKind(next: PostKind) {
    if (next === 'thought' && media.length) {
      setError('Для формата «Мысль» удалите изображения.');
      return;
    }
    setKind(next);
    setError('');
  }

  async function chooseImages(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? []);
    event.target.value = '';
    if (!files.length) return;
    if (media.length + files.length > 10) {
      setError('Можно добавить не больше 10 изображений.');
      return;
    }
    setUploading(true);
    setError('');
    try {
      const uploaded: SelectedMedia[] = [];
      for (const file of files) {
        const object = await uploadImage(file);
        uploaded.push({ object, preview: URL.createObjectURL(file) });
      }
      setMedia((current) => [...current, ...uploaded]);
      setKind('photo');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить изображение');
    } finally {
      setUploading(false);
    }
  }

  function removeImage(id: string) {
    setMedia((current) => {
      const target = current.find((item) => item.object.id === id);
      if (target) URL.revokeObjectURL(target.preview);
      return current.filter((item) => item.object.id !== id);
    });
  }

  async function publish(event: FormEvent) {
    event.preventDefault();
    const text = body.trim();
    const invalidText = kind === 'photo' ? text.length > limit : !text || text.length > limit;
    if (invalidText || (kind === 'photo' && media.length === 0) || busy || uploading) return;
    setBusy(true);
    setError('');
    try {
      let access = getAccessToken();
      if (!access) access = (await refreshSession())?.access_token ?? null;
      if (!access) {
        router.replace('/login');
        return;
      }
      const payload = JSON.stringify({ kind, body: text, visibility, media_ids: media.map((item) => item.object.id) });
      let response = await fetch(`${API_BASE}/api/v1/posts`, {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json', Authorization: `Bearer ${access}` },
        body: payload
      });
      if (response.status === 401) {
        const refreshed = await refreshSession();
        if (refreshed) {
          response = await fetch(`${API_BASE}/api/v1/posts`, {
            method: 'POST', credentials: 'include',
            headers: { 'Content-Type': 'application/json', Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` },
            body: payload
          });
        }
      }
      if (!response.ok) {
        const result = await response.json().catch(() => null);
        throw new Error(result?.error?.message ?? 'Не удалось опубликовать');
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
        <div><span className="eyebrow">СОЗДАТЬ</span><h1>{kind === 'thought' ? 'Мысль' : kind === 'photo' ? 'Фото' : 'Пост'}</h1></div>
        <a href="/create" aria-label="Закрыть">✕</a>
      </header>
      <form className="postComposer" onSubmit={publish}>
        <div className="postKindSwitch postKindSwitch--three" role="group" aria-label="Формат публикации">
          <button type="button" className={kind === 'thought' ? 'isActive' : ''} onClick={() => changeKind('thought')}>Мысль</button>
          <button type="button" className={kind === 'photo' ? 'isActive' : ''} onClick={() => changeKind('photo')}>Фото</button>
          <button type="button" className={kind === 'post' ? 'isActive' : ''} onClick={() => changeKind('post')}>Пост</button>
        </div>
        {kind === 'photo' ? (
          <div className="photoComposer">
            <label className="photoPicker">
              <input type="file" accept="image/jpeg,image/png" multiple onChange={chooseImages} disabled={uploading || media.length >= 10} />
              <span>{uploading ? 'Загружаем…' : media.length ? 'Добавить ещё' : 'Выбрать фото'}</span>
              <small>JPEG/PNG · до 8 МБ · максимум 10</small>
            </label>
            {media.length ? <div className="photoPreviewGrid">{media.map((item) => (
              <div className="photoPreview" key={item.object.id}>
                <img src={item.preview} alt="Предпросмотр" />
                <button type="button" onClick={() => removeImage(item.object.id)} aria-label="Удалить изображение">✕</button>
              </div>
            ))}</div> : null}
          </div>
        ) : null}
        <textarea autoFocus={kind !== 'photo'} value={body} onChange={(event) => setBody(event.target.value)} maxLength={limit} placeholder={kind === 'thought' ? 'Что думаете?' : kind === 'photo' ? 'Добавьте подпись…' : 'Расскажите подробнее…'} aria-label="Текст публикации" />
        <div className="postComposerMeta"><span>{body.length} / {limit}</span></div>
        <label className="postAudience"><span>Кто увидит</span><select value={visibility} onChange={(event) => setVisibility(event.target.value as 'public' | 'followers')}><option value="public">Все</option><option value="followers">Подписчики</option></select></label>
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        <Button type="submit" disabled={busy || uploading || (kind === 'photo' ? media.length === 0 || body.length > limit : body.trim().length === 0 || body.length > limit)} fullWidth>{busy ? 'Публикуем…' : 'Опубликовать'}</Button>
      </form>
    </AppShell>
  );
}
