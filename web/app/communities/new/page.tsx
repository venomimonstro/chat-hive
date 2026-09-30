'use client';

import { FormEvent, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { createCommunity } from '../../../lib/communities';

export default function NewCommunityPage() {
  const router = useRouter();
  const [slug, setSlug] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const canSubmit = useMemo(() => /^[a-z0-9][a-z0-9_-]{2,47}$/.test(slug) && title.trim().length >= 2, [slug, title]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!canSubmit || busy) return;
    setBusy(true); setError('');
    try {
      const community = await createCommunity({ slug, title: title.trim(), description: description.trim(), visibility });
      router.replace(`/c/${community.slug}`);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Не удалось создать сообщество');
    } finally { setBusy(false); }
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">СООБЩЕСТВО</span><h1>Создать</h1></div><a href="/communities" aria-label="Закрыть">✕</a></header>
      <main className="communityCreate"><Surface className="communityCreateCard">
        <form onSubmit={submit}>
          <label><span>Название</span><input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={80} placeholder="Фотографы Москвы" /></label>
          <label><span>Адрес</span><div className="communitySlug"><span>chat.ru/c/</span><input value={slug} onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))} maxLength={48} placeholder="photo_moscow" /></div></label>
          <label><span>Описание</span><textarea value={description} onChange={(e) => setDescription(e.target.value)} maxLength={1000} rows={4} /></label>
          <fieldset><legend>Доступ</legend><label className="communityChoice"><input type="radio" checked={visibility === 'private'} onChange={() => setVisibility('private')} /><div><strong>Приватное</strong><span>Создаётся сразу и не показывается в каталоге.</span></div></label><label className="communityChoice"><input type="radio" checked={visibility === 'public'} onChange={() => setVisibility('public')} /><div><strong>Публичное</strong><span>Сначала проходит модерацию, затем появляется в Discover.</span></div></label></fieldset>
          {error ? <p className="messengerError" role="alert">{error}</p> : null}
          <Button type="submit" fullWidth disabled={!canSubmit || busy}>{busy ? 'Создаём…' : 'Создать сообщество'}</Button>
        </form>
      </Surface></main>
    </AppShell>
  );
}
