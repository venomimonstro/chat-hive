'use client';

import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { AppShell } from '../components/AppShell';
import { Avatar, Badge, Button, IconButton, SearchField, Surface } from '../components/ui';
import {
  ChatMessage, ChatSummary, ensureDirectChat, getCurrentSession, listChats, listMessages,
  markChatRead, sendTextMessage
} from '../lib/api';

type LocalMessage = ChatMessage & { local_status?: 'sending' | 'failed' };

function chatName(chat: ChatSummary) {
  return chat.kind === 'direct' ? (chat.peer_display_name || chat.peer_username || 'Диалог') : (chat.title || 'Группа');
}

function formatTime(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  const today = new Date();
  if (date.toDateString() === today.toDateString()) return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' });
}

export function ChatsClient() {
  const [userId, setUserId] = useState('');
  const [chats, setChats] = useState<ChatSummary[]>([]);
  const [selected, setSelected] = useState<ChatSummary | null>(null);
  const [messages, setMessages] = useState<LocalMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [messageLoading, setMessageLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [showNewChat, setShowNewChat] = useState(false);
  const [draft, setDraft] = useState('');
  const [error, setError] = useState('');
  const selectedIdRef = useRef('');

  async function refreshChats() {
    try {
      const items = await listChats();
      setChats(items);
      if (selectedIdRef.current) {
        const fresh = items.find((item) => item.chat_id === selectedIdRef.current);
        if (fresh) setSelected(fresh);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить чаты');
    }
  }

  useEffect(() => {
    Promise.all([getCurrentSession(), listChats()])
      .then(([session, items]) => {
        if (session) setUserId(session.user_id);
        setChats(items);
      })
      .catch(() => setError('Не удалось загрузить чаты'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => { void refreshChats(); }, 5000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    selectedIdRef.current = selected?.chat_id ?? '';
    if (!selected) {
      setMessages([]);
      return;
    }
    let active = true;
    setMessageLoading(true);
    listMessages(selected.chat_id)
      .then(async (items) => {
        if (!active) return;
        const ordered = [...items].reverse();
        setMessages(ordered);
        const last = ordered.at(-1);
        if (last) await markChatRead(selected.chat_id, last.sequence).catch(() => undefined);
      })
      .catch((err) => active && setError(err instanceof Error ? err.message : 'Не удалось загрузить сообщения'))
      .finally(() => active && setMessageLoading(false));

    const timer = window.setInterval(async () => {
      if (!active) return;
      try {
        const items = await listMessages(selected.chat_id);
        const ordered = [...items].reverse();
        setMessages((current) => {
          const pending = current.filter((item) => item.local_status === 'sending' || item.local_status === 'failed');
          const serverIDs = new Set(ordered.map((item) => item.client_message_id));
          return [...ordered, ...pending.filter((item) => !serverIDs.has(item.client_message_id))];
        });
        const last = ordered.at(-1);
        if (last) await markChatRead(selected.chat_id, last.sequence).catch(() => undefined);
      } catch {
        // Temporary polling transport is best-effort; durable sync remains available on reconnect.
      }
    }, 3000);

    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [selected?.chat_id]);

  const filteredChats = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return chats;
    return chats.filter((chat) => `${chatName(chat)} ${chat.peer_username ?? ''}`.toLowerCase().includes(needle));
  }, [chats, search]);

  async function createChat(event: FormEvent) {
    event.preventDefault();
    const username = newUsername.trim().replace(/^@/, '');
    if (!username) return;
    setError('');
    try {
      const chat = await ensureDirectChat(username);
      setChats((current) => [chat, ...current.filter((item) => item.chat_id !== chat.chat_id)]);
      setSelected(chat);
      setNewUsername('');
      setShowNewChat(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать диалог');
    }
  }

  async function send(event: FormEvent, retry?: LocalMessage) {
    event.preventDefault();
    if (!selected) return;
    const text = (retry?.body ?? draft).trim();
    if (!text) return;
    const clientId = retry?.client_message_id ?? crypto.randomUUID();
    if (!retry) setDraft('');

    const optimistic: LocalMessage = retry ?? {
      id: `local-${clientId}`,
      chat_id: selected.chat_id,
      sender_id: userId,
      client_message_id: clientId,
      sequence: Number.MAX_SAFE_INTEGER,
      type: 'text',
      body: text,
      created_at: new Date().toISOString(),
      local_status: 'sending'
    };
    setMessages((current) => [...current.filter((item) => item.client_message_id !== clientId), { ...optimistic, local_status: 'sending' }]);

    try {
      const result = await sendTextMessage(selected.chat_id, text, clientId);
      setMessages((current) => [...current.filter((item) => item.client_message_id !== clientId), result.message].sort((a, b) => a.sequence - b.sequence));
      await refreshChats();
    } catch {
      setMessages((current) => current.map((item) => item.client_message_id === clientId ? { ...item, local_status: 'failed' } : item));
    }
  }

  return (
    <AppShell active="Чаты">
      <div className={`messenger ${selected ? 'hasConversation' : ''}`}>
        <section className="conversationListPane">
          <header className="screenHeader">
            <div><span className="eyebrow">CHAT</span><h1>Чаты</h1></div>
            <IconButton type="button" aria-label="Новое сообщение" onClick={() => setShowNewChat((value) => !value)}>＋</IconButton>
          </header>
          <div className="screenSearch"><SearchField value={search} onChange={(e) => setSearch(e.target.value)} aria-label="Поиск чатов" placeholder="Поиск чатов" /></div>

          {showNewChat ? (
            <form className="newChatForm" onSubmit={createChat}>
              <input autoFocus value={newUsername} onChange={(e) => setNewUsername(e.target.value)} placeholder="@username" aria-label="Username пользователя" />
              <Button type="submit">Открыть</Button>
            </form>
          ) : null}
          {error ? <div className="messengerError" role="alert">{error}</div> : null}

          <div className="chatList" aria-label="Список чатов">
            {loading ? <div className="chatListState">Загружаем чаты…</div> : null}
            {!loading && filteredChats.length === 0 ? <div className="chatListState">Диалогов пока нет. Нажмите ＋ и введите username.</div> : null}
            {filteredChats.map((chat) => (
              <button className={`chatRow chatRowButton ${selected?.chat_id === chat.chat_id ? 'isSelected' : ''}`} key={chat.chat_id} onClick={() => setSelected(chat)}>
                <Avatar name={chatName(chat)} />
                <div className="chatCopy">
                  <div className="chatHeadline"><strong>{chatName(chat)}</strong><time>{formatTime(chat.last_message?.created_at ?? chat.updated_at)}</time></div>
                  <div className="chatPreview"><span>{chat.last_message?.body || (chat.kind === 'direct' ? `@${chat.peer_username}` : 'Новый чат')}</span>{chat.unread_count > 0 ? <Badge>{chat.unread_count}</Badge> : null}</div>
                </div>
              </button>
            ))}
          </div>
        </section>

        <section className="conversationPane">
          {!selected ? (
            <div className="conversationPlaceholder"><div>✦</div><strong>Выберите чат</strong><span>Или создайте новый диалог по username.</span></div>
          ) : (
            <>
              <header className="conversationHeader">
                <IconButton className="conversationBack" type="button" aria-label="Назад к чатам" onClick={() => setSelected(null)}>‹</IconButton>
                <Avatar name={chatName(selected)} size="sm" />
                <div><strong>{chatName(selected)}</strong>{selected.peer_username ? <span>@{selected.peer_username}</span> : null}</div>
              </header>
              <div className="messageList" aria-live="polite">
                {messageLoading ? <div className="chatListState">Загружаем историю…</div> : null}
                {messages.map((message) => {
                  const mine = message.sender_id === userId;
                  return (
                    <div className={`messageRow ${mine ? 'isMine' : ''}`} key={message.client_message_id}>
                      <div className={`messageBubble ${message.local_status === 'failed' ? 'isFailed' : ''}`}>
                        <p>{message.deleted_at ? 'Сообщение удалено' : message.body}</p>
                        <div className="messageMeta"><time>{formatTime(message.created_at)}</time>{message.local_status === 'sending' ? <span>отправляем…</span> : null}{message.local_status === 'failed' ? <button type="button" onClick={(e) => void send(e as unknown as FormEvent, message)}>повторить</button> : null}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
              <form className="messageComposer" onSubmit={send}>
                <button type="button" aria-label="Добавить вложение" disabled>＋</button>
                <textarea value={draft} onChange={(e) => setDraft(e.target.value)} rows={1} maxLength={4096} placeholder="Сообщение" aria-label="Сообщение" onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); e.currentTarget.form?.requestSubmit(); } }} />
                <button className="sendMessageButton" type="submit" disabled={!draft.trim()} aria-label="Отправить">↑</button>
              </form>
            </>
          )}
        </section>
      </div>
    </AppShell>
  );
}
