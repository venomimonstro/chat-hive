'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Surface } from '../../components/ui';
import { Community, discoverCommunities } from '../../lib/communities';

export default function CommunitiesPage() {
  const router = useRouter();
  const [items, setItems] = useState<Community[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    void discoverCommunities()
      .then(setItems)
      .catch((err) => {
        if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
        setError(err instanceof Error ? err.message : 'Не удалось загрузить сообщества');
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell active="Открыть">
      <header className="screenHeader">
        <div><span className="eyebrow">DISCOVER</span><h1>Сообщества</h1></div>
        <a className="uiButton uiButton--secondary" href="/communities/new">Создать</a>
      </header>
      <main className="communityCatalog">
        {loading ? <p className="chatListState">Ищем активные сообщества…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? <Surface className="communityEmpty"><strong>Публичных сообществ пока нет</strong><p>Создайте первое тематическое сообщество. После модерации оно появится здесь.</p></Surface> : null}
        {items.map((item) => (
          <a className="communityCatalogCard" href={`/c/${item.slug}`} key={item.id}>
            <div className="communityMark">#</div>
            <div><strong>{item.title}</strong><span>@{item.slug} · {item.members_count} участников</span><p>{item.description || 'Без описания'}</p></div>
          </a>
        ))}
      </main>
    </AppShell>
  );
}
