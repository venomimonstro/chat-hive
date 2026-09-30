'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Avatar, Button, Surface } from '../../components/ui';
import { decideMessageRequest, listMessageRequests, MessageRequest } from '../../lib/messageRequests';

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function RequestsPage() {
  const router = useRouter();
  const [items, setItems] = useState<MessageRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      setItems(await listMessageRequests());
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') {
        router.replace('/login');
        return;
      }
      setError(err instanceof Error ? err.message : 'Не удалось загрузить запросы');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  async function decide(item: MessageRequest, accept: boolean) {
    if (busy) return;
    setBusy(item.chat_id);
    setError('');
    try {
      await decideMessageRequest(item.chat_id, accept);
      setItems((current) => current.filter((request) => request.chat_id !== item.chat_id));
      if (accept) router.push(`/?chat=${encodeURIComponent(item.chat_id)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось обработать запрос');
    } finally {
      setBusy('');
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">БЕЗОПАСНОСТЬ</span><h1>Запросы</h1></div>
        <a href="/" aria-label="Назад">←</a>
      </header>
      <main className="requestInbox">
        {loading ? <p className="chatListState">Загружаем запросы…</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? (
          <Surface className="requestEmpty"><strong>Новых запросов нет</strong><p>Сообщения от незнакомых людей будут появляться здесь, а не среди обычных чатов.</p></Surface>
        ) : null}
        {items.map((item) => (
          <Surface className="requestCard" key={item.chat_id}>
            <div className="requestPerson">
              <Avatar name={item.display_name || item.username} />
              <div><a href={`/u/${item.username}`}><strong>{item.display_name || item.username}</strong></a><span>@{item.username} · {formatDate(item.created_at)}</span></div>
            </div>
            <div className="requestSignals">
              {item.shared_interests > 0 ? <span>{item.shared_interests} общих интересов</span> : null}
              {item.mutual_followers > 0 ? <span>{item.mutual_followers} общих подписок</span> : null}
              {!item.shared_interests && !item.mutual_followers ? <span>Новый контакт</span> : null}
            </div>
            <p className="requestMessage">{item.body || 'Пользователь хочет начать разговор.'}</p>
            <div className="requestActions">
              <Button onClick={() => void decide(item, true)} disabled={busy === item.chat_id}>Принять</Button>
              <Button variant="secondary" onClick={() => void decide(item, false)} disabled={busy === item.chat_id}>Удалить</Button>
              <a className="uiButton uiButton--ghost" href={`/u/${item.username}`}>Профиль</a>
            </div>
          </Surface>
        ))}
      </main>
    </AppShell>
  );
}
