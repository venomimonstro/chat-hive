'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { getAccessToken } from '../../../lib/api';
import { Community, getCommunity, joinCommunity, leaveCommunity } from '../../../lib/communities';

export function CommunityClient({ slug }: { slug: string }) {
  const router = useRouter();
  const [community, setCommunity] = useState<Community | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  function requireLogin() { router.push(`/login?next=${encodeURIComponent(`/c/${slug}`)}`); }

  async function load() {
    setError('');
    try { setCommunity(await getCommunity(slug)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Сообщество недоступно'); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, [slug]);

  async function join() {
    if (busy) return;
    if (!getAccessToken()) { requireLogin(); return; }
    setBusy(true); setError('');
    try { setCommunity(await joinCommunity(slug)); }
    catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { requireLogin(); return; }
      setError(err instanceof Error ? err.message : 'Не удалось вступить');
    } finally { setBusy(false); }
  }

  async function leave() {
    if (busy) return;
    setBusy(true); setError('');
    try { await leaveCommunity(slug); router.replace('/communities'); }
    catch (err) { setError(err instanceof Error ? err.message : 'Не удалось выйти'); }
    finally { setBusy(false); }
  }

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">СООБЩЕСТВО</span><h1>{community?.title ?? 'Сообщество'}</h1></div><a href="/communities">←</a></header>
      <main className="communityPage">
        {loading ? <p className="chatListState">Загружаем…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {community ? <Surface className="communityHero">
          <div className="communityHeroMark">#</div>
          <h2>{community.title}</h2><span>@{community.slug}</span>
          <p>{community.description || 'Без описания'}</p>
          <div className="communityStats"><strong>{community.members_count}</strong><span>участников</span></div>
          {community.visibility === 'public' && community.moderation_status === 'pending' ? <div className="communityPending">Публичное размещение ожидает модерации. Пока страницу видят только участники.</div> : null}
          {community.moderation_status === 'rejected' ? <div className="communityPending isRejected">Публичное размещение не одобрено. Сообщество остаётся доступно владельцу и участникам.</div> : null}
          <div className="communityActions">
            {community.role ? <a className="uiButton uiButton--primary" href={`/?chat=${encodeURIComponent(community.chat_id)}`}>Открыть чат</a> : <Button onClick={join} disabled={busy}>Вступить</Button>}
            {community.role && community.role !== 'owner' ? <Button variant="secondary" onClick={leave} disabled={busy}>Выйти</Button> : null}
            <a className="uiButton uiButton--ghost" href={`/report?type=group&id=${encodeURIComponent(community.chat_id)}`}>Пожаловаться</a>
          </div>
        </Surface> : null}
      </main>
    </AppShell>
  );
}
