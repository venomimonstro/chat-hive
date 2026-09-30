'use client';

import { FormEvent, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { createChannel } from '../../../lib/channels';

export default function NewChannelPage() {
  const router = useRouter();
  const [slug, setSlug] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const canSubmit = useMemo(() => /^[a-z0-9][a-z0-9_-]{2,47}$/.test(slug) && title.trim().length >= 2, [slug, title]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!canSubmit || busy) return;
    setBusy(true); setError('');
    try {
      const channel = await createChannel({ slug, title: title.trim(), description: description.trim() });
      router.replace(`/channel/${channel.slug}`);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Не удалось создать канал');
    } finally { setBusy(false); }
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">КАНАЛ</span><h1>Создать</h1></div><a href="/channels" aria-label="Закрыть">✕</a></header>
      <main className="channelCreate"><Surface className="channelCreateCard"><form onSubmit={submit}>
        <label><span>Название</span><input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={80} placeholder="Новости продукта" /></label>
        <label><span>Адрес</span><div className="channelSlug"><span>chat.ru/channel/</span><input value={slug} onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))} maxLength={48} placeholder="product_news" /></div></label>
        <label><span>Описание</span><textarea value={description} onChange={(e) => setDescription(e.target.value)} maxLength={1000} rows={4} /></label>
        <div className="channelModerationNote">Канал создаётся сразу для владельца. В публичный каталог он попадёт только после модерации.</div>
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        <Button type="submit" fullWidth disabled={!canSubmit || busy}>{busy ? 'Создаём…' : 'Создать канал'}</Button>
      </form></Surface></main>
    </AppShell>
  );
}
