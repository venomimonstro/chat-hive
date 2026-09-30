'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { Channel, ChannelPost, createChannelPost, getChannel, listChannelPosts, setChannelSubscription } from '../../../lib/channels';

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function ChannelPage() {
  const params = useParams<{ slug: string }>();
  const router = useRouter();
  const slug = params.slug;
  const [channel, setChannel] = useState<Channel | null>(null);
  const [posts, setPosts] = useState<ChannelPost[]>([]);
  const [body, setBody] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      const [channelValue, postValues] = await Promise.all([getChannel(slug), listChannelPosts(slug)]);
      setChannel(channelValue); setPosts(postValues);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Канал недоступен');
    } finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, [slug]);

  async function toggleSubscription() {
    if (!channel || channel.mine || busy) return;
    setBusy(true); setError('');
    try { setChannel(await setChannelSubscription(slug, !channel.subscribed)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Не удалось изменить подписку'); }
    finally { setBusy(false); }
  }

  async function publish(event: FormEvent) {
    event.preventDefault();
    const text = body.trim();
    if (!channel?.mine || !text || busy) return;
    setBusy(true); setError('');
    try {
      const post = await createChannelPost(slug, text);
      setPosts((current) => [post, ...current]);
      setBody('');
    } catch (err) { setError(err instanceof Error ? err.message : 'Не удалось опубликовать'); }
    finally { setBusy(false); }
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">КАНАЛ</span><h1>{channel?.title ?? 'Канал'}</h1></div><a href="/channels">←</a></header>
      <main className="channelPage">
        {loading ? <p className="chatListState">Загружаем…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {channel ? <>
          <Surface className="channelHero">
            <div className="channelHeroMark">◈</div><h2>{channel.title}</h2><span>@{channel.slug}</span><p>{channel.description || 'Без описания'}</p>
            <div className="channelStats"><strong>{channel.subscribers_count}</strong><span>подписчиков</span></div>
            {channel.moderation_status === 'pending' ? <div className="channelModerationNote">Канал ожидает модерации и пока не показывается в публичном каталоге.</div> : null}
            {channel.moderation_status === 'rejected' ? <div className="channelModerationNote isRejected">Публичное размещение канала не одобрено.</div> : null}
            <div className="channelActions">
              {!channel.mine ? <Button onClick={toggleSubscription} disabled={busy}>{channel.subscribed ? 'Отписаться' : 'Подписаться'}</Button> : <span className="channelOwnerBadge">Вы владелец</span>}
            </div>
          </Surface>
          {channel.mine ? <Surface className="channelComposer"><form onSubmit={publish}><textarea value={body} onChange={(e) => setBody(e.target.value)} maxLength={8000} rows={4} placeholder="Новая публикация канала…" /><Button type="submit" disabled={busy || !body.trim()}>Опубликовать</Button></form></Surface> : null}
          <section className="channelPosts">
            {posts.length === 0 ? <Surface className="channelEmpty">Публикаций пока нет.</Surface> : null}
            {posts.map((post) => <article className="channelPost" key={post.id}><a href={`/post/${post.id}`}><p>{post.body}</p><span>{formatDate(post.created_at)}</span></a></article>)}
          </section>
        </> : null}
      </main>
    </AppShell>
  );
}
