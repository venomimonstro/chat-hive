'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Avatar, Button, Surface } from '../../components/ui';
import { decideMessageRequest, listMessageRequests, MessageRequest } from '../../lib/messageRequests';
import { useLanguage } from '../../lib/language';
const requestEN: Record<string,string> = {
  "БЕЗОПАСНОСТЬ": "PRIVACY",
  "Запросы": "Message requests",
  "Назад": "Back",
  "Загружаем запросы…": "Loading message requests…",
  "Новых запросов нет": "No new requests",
  "Сообщения от незнакомых людей будут появляться здесь, а не среди обычных чатов.": "Messages from new contacts appear here before entering your regular chats.",
  "общих интересов": "shared interests",
  "общих подписок": "mutual connections",
  "Новый контакт": "New contact",
  "Пользователь хочет начать разговор.": "Someone wants to start a conversation.",
  "Принять": "Accept",
  "Удалить": "Decline",
  "Профиль": "Profile",
  "Не удалось загрузить запросы": "Could not load requests",
  "Не удалось обработать запрос": "Could not process request"
};

function formatDate(value: string, language: 'ru' | 'en' = 'ru') {
  return new Date(value).toLocaleString(language === 'en' ? 'en-US' : 'ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export default function RequestsPage() {
  const { language, t } = useLanguage();
  const tr = (value: string) => language === 'en' ? (requestEN[value] ?? value) : value;
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
      setError(err instanceof Error ? err.message : tr("Не удалось загрузить запросы"));
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
      setError(err instanceof Error ? err.message : tr("Не удалось обработать запрос"));
    } finally {
      setBusy('');
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">{tr("БЕЗОПАСНОСТЬ")}</span><h1>{tr("Запросы")}</h1></div>
        <a href="/" aria-label="Назад">←</a>
      </header>
      <main className="requestInbox">
        {loading ? <p className="chatListState">{tr("Загружаем запросы…")}</p> : null}
        {error ? <p className="messengerError" role="alert">{error}</p> : null}
        {!loading && !error && items.length === 0 ? (
          <Surface className="requestEmpty"><strong>{tr("Новых запросов нет")}</strong><p>{tr("Сообщения от незнакомых людей будут появляться здесь, а не среди обычных чатов.")}</p></Surface>
        ) : null}
        {items.map((item) => (
          <Surface className="requestCard" key={item.chat_id}>
            <div className="requestPerson">
              <Avatar name={item.display_name || item.username} />
              <div><a href={`/?chat=${encodeURIComponent(item.chat_id)}`}><strong>{item.display_name || item.username}</strong></a><span>@{item.username} · {formatDate(item.created_at, language)}</span></div>
            </div>
            <div className="requestSignals"><span>{tr("Новый контакт")}</span></div>
            <p className="requestMessage">{item.body || tr("Пользователь хочет начать разговор.")}</p>
            <div className="requestActions">
              <Button onClick={() => void decide(item, true)} disabled={busy === item.chat_id}>{tr("Принять")}</Button>
              <Button variant="secondary" onClick={() => void decide(item, false)} disabled={busy === item.chat_id}>{tr("Удалить")}</Button>
              
            </div>
          </Surface>
        ))}
      </main>
    </AppShell>
  );
}
