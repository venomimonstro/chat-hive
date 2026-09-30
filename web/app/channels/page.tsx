'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Surface } from '../../components/ui';
import { Channel, discoverChannels } from '../../lib/channels';

export default function ChannelsPage() {
  const router = useRouter();
  const [items, setItems] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    void discoverChannels().then(setItems).catch((err) => {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Не удалось загрузить каналы');
    }).finally(() => setLoading(false));
  }, []);

  return (
    <AppShell active="Открыть">
      <header className="screenHeader"><div><span className="eyebrow">DISCOVER</span><h1>Каналы</h1></div><a className="uiButton uiButton--secondary" href="/channels/new">Создать</a></header>
      <main className="channelCatalog">
        {loading ? <p className="chatListState">Загружаем каналы…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? <Surface className="channelEmpty"><strong>Публичных каналов пока нет</strong><p>После модерации новые каналы появятся здесь.</p></Surface> : null}
        {items.map((item) => <a className="channelCatalogCard" href={`/channel/${item.slug}`} key={item.id}><div className="channelMark">◈</div><div><strong>{item.title}</strong><span>@{item.slug} · {item.subscribers_count} подписчиков</span><p>{item.description || 'Без описания'}</p></div></a>)}
      </main>
    </AppShell>
  );
}
