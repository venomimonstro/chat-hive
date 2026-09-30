'use client';

import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { AppShell } from '../components/AppShell';
import { Avatar, Badge, Button, IconButton, SearchField } from '../components/ui';
import {
  ChatMessage, ChatSummary, ensureDirectChat, getCurrentSession, listChats, listMessages,
  markChatRead, sendTextMessage
} from '../lib/api';

type LocalMessage = ChatMessage & { local_status?: 'sending' | 'failed' };
type OutboxItem = { chat_id: string; client_message_id: string; body: string; created_at: string };

const OUTBOX_KEY = 'chat_outbox_v1';

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

function loadOutbox(): OutboxItem[] {
  if (typeof window === 'undefined') return [];
  try {
    const parsed = JSON.parse(window.localStorage.getItem(OUTBOX_KEY) ?? '[]');
    return Array.isArray(parsed) ? parsed.filter((item) => item && typeof item.chat_id === 'string' && typeof item.client_message_id === 'string' && typeof item.body === 'string') : [];
  } catch {
    return [];
  }
}

function saveOutbox(items: OutboxItem[]) {
  if (typeof window === 'undefined') return;
  if (items.length === 0) window.localStorage.removeItem(OUTBOX_KEY);
  else window.localStorage.setItem(OUTBOX_KEY, JSON.stringify(items.slice(-200)));
}

function replaceChatQuery(chatID?: string) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (chatID) url.searchParams.set('chat', chatID);
  else url.searchParams.delete('chat');
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
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
  const flushingRef = useRef(false);

  function chooseChat(chat: ChatSummary | null) {
    setSelected(chat);
    replaceChatQuery(chat?.chat_id);
  }

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

  async function flushOutbox() {
    if (flushingRef.current || typeof navigator !== 'undefined' && !navigator.onLine) return;
    flushingRef.current = true;
    try {
      const items = loadOutbox();
      const remaining: OutboxItem[] = [];
      for (const item of items) {
        try {
          const result = await sendTextMessage(item.chat_id, item.body, item.client_message_id);
          if (selectedIdRef.current === item.chat_id) {
            setMessages((current) => [...current.filter((message) => message.client_message_id !== item.client_message_id), result.message].sort((a, b) => a.sequence - b.sequence));
          }
        } catch {
          remaining.push(item);
        }
      }
      saveOutbox(remaining);
      if (items.length !== remaining.length) await refreshChats();
    } finally {
      flushingRef.current = false;
    }
  }

  useEffect(() => {
    Promise.all([getCurrentSession(), listChats()])
      .then(([session, items]) => {
        if (!session) {
          window.location.replace('/login');
          return;
        }
        setUserId(session.user_id);
        setChats(items);
        const requested = new URLSearchParams(window.location.search).get('chat');
        if (requested) {
          const match = items.find((item) => item.chat_id === requested);
          if (match) setSelected(match);
        }
        void flushOutbox();
      })
      .catch(() => setError('Не удалось загрузить чаты'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    const online = () => void flushOutbox();
    window.addEventListener('online', online);
    const timer = window.setInterval(() => { void refreshChats(); void flushOutbox(); }, 5000);
    return () => {
      window.removeEventListener('online', online);
      window.clearInterval(timer);
    };
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
        const pending = loadOutbox().filter((item) => item.chat_id === selected.chat_id).map<LocalMessage>((item) => ({
          id: `local-${item.client_message_id}`,
          chat_id: item.chat_id,
          sender_id: userId,
          client_message_id: item.client_message_id,
          sequence: Number.MAX_SAFE_INTEGER,
          type: 'text',
          body: item.body,
          created_at: item.created_at,
          local_status: navigator.onLine ? 'sending' : 'failed'
        }));
        const serverIDs = new Set(ordered.map((item) => item.client_message_id));
        setMessages([...ordered, ...pending.filter((item) => !serverIDs.has(item.client_message_id))]);
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
        // REST polling is temporary transport. Durable outbox preserves unsent messages.
      }
    }, 3000);

    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [selected?.chat_id, userId]);

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
      chooseChat(chat);
      setNewUsername('');
      setShowNewChat(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать диалог');
    }
  }

  async function deliver(item: OutboxItem) {
    setMessages((current) => current.map((message) => message.client_message_id === item.client_message_id ? { ...message, local_status: 'sending' } : message));
    try {
      const result = await sendTextMessage(item.chat_id, item.body, item.client_message_id);
      const remaining = loadOutbox().filter((queued) => queued.client_message_id !== item.client_message_id);
      saveOutbox(remaining);
      setMessages((current) => [...current.filter((message) => message.client_message_id !== item.client_message_id), result.message].sort((a, b) => a.sequence - b.sequence));
      await refreshChats();
    } catch {
      setMessages((current) => current.map((message) => message.client_message_id === item.client_message_id ? { ...message, local_status: 'failed' } : message));
    }
  }

  async function send(event: FormEvent) {
    event.preventDefault();
    if (!selected) return;
    const text = draft.trim();
    if (!text) return;
    const item: OutboxItem = {
      chat_id: selected.chat_id,
      client_message_id: crypto.randomUUID(),
      body: text,
      created_at: new Date().toISOString()
    };
    saveOutbox([...loadOutbox(), item]);
    setDraft('');
    setMessages((current) => [...current, {
      id: `local-${item.client_message_id}`,
      chat_id: item.chat_id,
      sender_id: userId,
      client_message_id: item.client_message_id,
      sequence: Number.MAX_SAFE_INTEGER,
      type: 'text',
      body: item.body,
      created_at: item.created_at,
      local_status: navigator.onLine ? 'sending' : 'failed'
    }]);
    if (navigator.onLine) await deliver(item);
  }

  function retry(message: LocalMessage) {
    const item = loadOutbox().find((queued) => queued.client_message_id === message.client_message_id);
    if (item) void deliver(item);
  }

  return (
    <AppShell active="Чаты" wide>
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
              <button className={`chatRow chatRowButton ${selected?.chat_id === chat.chat_id ? 'isSelected' : ''}`} key={chat.chat_id} onClick={() => chooseChat(chat)}>
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
                <IconButton className="conversationBack" type="button" aria-label="Назад к чатам" onClick={() => chooseChat(null)}>‹</IconButton>
                <Avatar name={chatName(selected)} size="sm" />
                <div className="conversationIdentity"><strong>{chatName(selected)}</strong>{selected.peer_username ? <span>@{selected.peer_username}</span> : <span>Группа</span>}</div>
                {selected.kind === 'group' ? <a className="conversationSettings" href={`/groups/${selected.chat_id}`} aria-label="Настройки группы">•••</a> : null}
              </header>
              <div className="messageList" aria-live="polite">
                {messageLoading ? <div className="chatListState">Загружаем историю…</div> : null}
                {messages.map((message) => {
                  const mine = message.sender_id === userId;
                  return (
                    <div className={`messageRow ${mine ? 'isMine' : ''}`} key={message.client_message_id}>
                      <div className={`messageBubble ${message.local_status === 'failed' ? 'isFailed' : ''}`}>
                        <p>{message.deleted_at ? 'Сообщение удалено' : message.body}</p>
                        <div className="messageMeta"><time>{formatTime(message.created_at)}</time>{message.local_status === 'sending' ? <span>отправляем…</span> : null}{message.local_status === 'failed' ? <button type="button" onClick={() => retry(message)}>повторить</button> : null}</div>
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
