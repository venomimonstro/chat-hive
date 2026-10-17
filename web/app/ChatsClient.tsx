'use client';

import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { AppShell } from '../components/AppShell';
import { Avatar, Badge, Button, IconButton, SearchField } from '../components/ui';
import {
  ChatMessage, ChatSummary, ensureDirectChat, getCurrentSession, listChats, listMessages,
  markChatRead, sendTextMessage
} from '../lib/api';
import { deleteChatMessage, editChatMessage } from '../lib/messageActions';
import {
  enqueueOutbox, findOutbox, listOutbox, OutboxItem, removeOutbox
} from '../lib/outbox';
import type { RealtimeEvent } from '../lib/realtime';
import { reconcileTimeline } from '../lib/messageTimeline';
import { nextOutboxRetry, readyForOutboxRetry, RetryState } from '../lib/outboxRetry';
import { useLanguage, type Language } from '../lib/language';

type LocalMessage = ChatMessage & { local_status?: 'sending' | 'failed' };
const PAGE_SIZE = 50;

function chatName(chat: ChatSummary) {
  return chat.kind === 'direct' ? (chat.peer_display_name || chat.peer_username || 'Диалог') : (chat.title || 'Группа');
}

function formatTime(value?: string, language: Language = 'ru') {
  if (!value) return '';
  const date = new Date(value);
  const today = new Date();
  if (date.toDateString() === today.toDateString()) return date.toLocaleTimeString(language === 'en' ? 'en-US' : 'ru-RU', { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleDateString(language === 'en' ? 'en-US' : 'ru-RU', { day: '2-digit', month: '2-digit' });
}

function replaceChatQuery(chatID?: string) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (chatID) url.searchParams.set('chat', chatID);
  else url.searchParams.delete('chat');
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

function asLocalMessage(item: OutboxItem, userId: string): LocalMessage {
  return {
    id: `local-${item.client_message_id}`,
    chat_id: item.chat_id,
    sender_id: userId,
    client_message_id: item.client_message_id,
    sequence: Number.MAX_SAFE_INTEGER,
    type: 'text',
    body: item.body,
    reply_to_id: item.reply_to_id,
    created_at: item.created_at,
    local_status: typeof navigator !== 'undefined' && navigator.onLine ? 'sending' : 'failed'
  };
}

export function ChatsClient() {
  const { language, t } = useLanguage();
  const [userId, setUserId] = useState('');
  const [chats, setChats] = useState<ChatSummary[]>([]);
  const [selected, setSelected] = useState<ChatSummary | null>(null);
  const [messages, setMessages] = useState<LocalMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [messageLoading, setMessageLoading] = useState(false);
  const [olderLoading, setOlderLoading] = useState(false);
  const [hasOlder, setHasOlder] = useState(false);
  const [search, setSearch] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [showNewChat, setShowNewChat] = useState(false);
  const [draft, setDraft] = useState('');
  const [replyingTo, setReplyingTo] = useState<LocalMessage | null>(null);
  const [editing, setEditing] = useState<LocalMessage | null>(null);
  const [error, setError] = useState('');
  const selectedIdRef = useRef('');
  const flushingRef = useRef(false);
  const realtimeConnectedRef = useRef(false);
  const sendingIdsRef = useRef(new Set<string>());
  const outboxRetryRef = useRef(new Map<string, RetryState>());
  const accountRef = useRef('');
  const syncingChatsRef = useRef(new Set<string>());

  function chooseChat(chat: ChatSummary | null) {
    // React effects run after render. Update the guard synchronously so
    // responses from the previous conversation cannot mutate the new view.
    selectedIdRef.current = chat?.chat_id ?? '';
    setMessages([]);
    setHasOlder(false);
    setOlderLoading(false);
    setMessageLoading(Boolean(chat));
    setSelected(chat);
    setReplyingTo(null);
    setEditing(null);
    setDraft('');
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

  async function syncLatest(chatID: string) {
    if (!chatID || syncingChatsRef.current.has(chatID)) return;
    syncingChatsRef.current.add(chatID);
    try {
      const [items, queued] = await Promise.all([listMessages(chatID, undefined, PAGE_SIZE), listOutbox(accountRef.current, chatID)]);
      if (selectedIdRef.current !== chatID) return;
      const ordered = [...items].reverse();
      setMessages((current) => reconcileTimeline(current, items, queued, userId, chatID, true));
      // Reactions belong to a later release; no disabled API polling.
      const last = ordered.at(-1);
      if (last) await markChatRead(chatID, last.sequence).catch(() => undefined);
    } catch {
      // REST polling/recovery will retry. Realtime never becomes the durable source of truth.
    } finally {
      syncingChatsRef.current.delete(chatID);
    }
  }

  async function flushOutbox() {
    if (!accountRef.current || flushingRef.current || (typeof navigator !== 'undefined' && !navigator.onLine)) return;
    flushingRef.current = true;
    try {
      const session = await getCurrentSession();
      if (!session || session.user_id !== accountRef.current) return;
      const items = await listOutbox(session.user_id);
      let delivered = 0;
      for (const item of items) {
        if (item.user_id !== accountRef.current) break;
        if (sendingIdsRef.current.has(item.client_message_id)) continue;
        if (!readyForOutboxRetry(outboxRetryRef.current.get(item.client_message_id), Date.now())) continue;
        sendingIdsRef.current.add(item.client_message_id);
        try {
          const result = await sendTextMessage(item.chat_id, item.body, item.client_message_id, item.reply_to_id ?? '');
          await removeOutbox(item.user_id, item.client_message_id);
          outboxRetryRef.current.delete(item.client_message_id);
          delivered++;
          if (selectedIdRef.current === item.chat_id) {
            setMessages((current) => [
              ...current.filter((message) => message.client_message_id !== item.client_message_id),
              result.message
            ].sort((a, b) => a.sequence - b.sequence));
          }
        } catch {
          // Retain durable data, but avoid hammering a degraded API.
          outboxRetryRef.current.set(item.client_message_id,
            nextOutboxRetry(outboxRetryRef.current.get(item.client_message_id), Date.now()));
        } finally {
          sendingIdsRef.current.delete(item.client_message_id);
        }
      }
      // Bound ephemeral retry metadata to messages that remain durably queued.
      const pendingIDs = new Set(items.map((item) => item.client_message_id));
      for (const key of outboxRetryRef.current.keys()) {
        if (!pendingIDs.has(key)) outboxRetryRef.current.delete(key);
      }
      if (delivered > 0) await refreshChats();
    } catch {
      // IndexedDB failure must not crash the messenger shell.
    } finally {
      flushingRef.current = false;
    }
  }

  useEffect(() => {
    let mounted = true;
    void getCurrentSession()
      .then(async (session) => {
        if (!mounted) return;
        if (!session) {
          window.location.replace('/login');
          return;
        }
        // Establish account ownership BEFORE fetching chats. A transient
        // list failure must not leave the user in an uninitialized shell.
        accountRef.current = session.user_id;
        setUserId(session.user_id);
        void flushOutbox();
        const items = await listChats();
        if (!mounted || accountRef.current !== session.user_id) return;
        setChats(items);
        const requested = new URLSearchParams(window.location.search).get('chat');
        if (requested) {
          const match = items.find((item) => item.chat_id === requested);
          if (match) setSelected(match);
        }
      })
      .catch(() => { if (mounted) setError('Не удалось загрузить чаты. Проверьте соединение и повторите попытку.'); })
      .finally(() => { if (mounted) setLoading(false); });
    return () => { mounted = false; };
  }, []);

  useEffect(() => {
    const onConnection = (event: Event) => {
      realtimeConnectedRef.current = (event as CustomEvent<boolean>).detail === true;
    };
    const onForeground = () => {
      if (document.visibilityState !== 'visible') return;
      void refreshChats();
      void flushOutbox();
    };
    window.addEventListener('chat:connection', onConnection);
    window.addEventListener('online', onForeground);
    document.addEventListener('visibilitychange', onForeground);
    // Realtime events refresh immediately. Without a socket, REST is the
    // recovery path; periodically reconcile even when connected.
    const fast = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      void flushOutbox();
      if (!realtimeConnectedRef.current) void refreshChats();
    }, 5000);
    const reconcile = window.setInterval(() => {
      if (document.visibilityState === 'visible') void refreshChats();
    }, 30000);
    return () => {
      window.removeEventListener('chat:connection', onConnection);
      window.removeEventListener('online', onForeground);
      document.removeEventListener('visibilitychange', onForeground);
      window.clearInterval(fast);
      window.clearInterval(reconcile);
    };
  }, []);

  useEffect(() => {
    const listener = (rawEvent: Event) => {
      const event = rawEvent as CustomEvent<RealtimeEvent>;
      const detail = event.detail;
      if (!detail || detail.type !== 'message.created' || !detail.chat_id) return;
      void refreshChats();
      if (detail.chat_id === selectedIdRef.current) void syncLatest(detail.chat_id);
    };
    window.addEventListener('chat:realtime', listener as EventListener);
    return () => window.removeEventListener('chat:realtime', listener as EventListener);
  }, [userId]);

  useEffect(() => {
    selectedIdRef.current = selected?.chat_id ?? '';
    if (!selected) {
      setMessages([]);
      setHasOlder(false);
        return;
    }

    let active = true;
    setMessages([]);
    setHasOlder(false);
    setMessageLoading(true);
    Promise.all([listMessages(selected.chat_id, undefined, PAGE_SIZE), listOutbox(accountRef.current, selected.chat_id)])
      .then(async ([items, queued]) => {
        if (!active) return;
        const ordered = [...items].reverse();
        setHasOlder(items.length === PAGE_SIZE);
        setMessages((current) => reconcileTimeline(current, items, queued, userId, selected.chat_id));
        // Reactions belong to a later release.
        const last = ordered.at(-1);
        if (last) await markChatRead(selected.chat_id, last.sequence).catch(() => undefined);
      })
      .catch((err) => active && setError(err instanceof Error ? err.message : 'Не удалось загрузить сообщения'))
      .finally(() => active && setMessageLoading(false));

    let elapsed = 0;
    const timer = window.setInterval(() => {
      if (!active || document.visibilityState !== 'visible') return;
      elapsed += 5;
      if (!realtimeConnectedRef.current || elapsed >= 30) {
        elapsed = 0;
        void syncLatest(selected.chat_id);
      }
    }, 5000);
    // Resume immediately after a sleeping tab, network recovery or a fresh
    // realtime handshake. The server remains authoritative for missed events.
    const resync = () => {
      if (active && document.visibilityState === 'visible') void syncLatest(selected.chat_id);
    };
    const onConnection = (event: Event) => {
      if ((event as CustomEvent<boolean>).detail === true) resync();
    };
    document.addEventListener('visibilitychange', resync);
    window.addEventListener('online', resync);
    window.addEventListener('chat:connection', onConnection);

    return () => {
      active = false;
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', resync);
      window.removeEventListener('online', resync);
      window.removeEventListener('chat:connection', onConnection);
    };
  }, [selected?.chat_id, userId]);

  const filteredChats = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return chats.filter((chat) => chat.kind === 'direct');
    return chats.filter((chat) => chat.kind === 'direct').filter((chat) => `${chatName(chat)} ${chat.peer_username ?? ''}`.toLowerCase().includes(needle));
  }, [chats, search]);

  const messageByID = useMemo(() => new Map(messages.filter((item) => !item.local_status).map((item) => [item.id, item])), [messages]);

  async function loadOlder() {
    if (!selected || olderLoading || !hasOlder) return;
    const first = messages.find((item) => Number.isFinite(item.sequence) && item.sequence < Number.MAX_SAFE_INTEGER);
    if (!first) return;
    setOlderLoading(true);
    try {
      const items = await listMessages(selected.chat_id, first.sequence, PAGE_SIZE);
      if (selectedIdRef.current !== selected.chat_id) return;
      const ordered = [...items].reverse();
      // The same page may already have arrived through realtime recovery:
      // dedupe by stable client ID rather than blindly prepending it.
      setMessages((current) => reconcileTimeline(current, items, [], userId, selected.chat_id, true));
      setHasOlder(items.length === PAGE_SIZE);
      // Reactions belong to a later release.
    } catch (err) {
      if (selectedIdRef.current !== selected.chat_id) return;
      setError(err instanceof Error ? err.message : 'Не удалось загрузить старые сообщения');
    } finally {
      if (selectedIdRef.current === selected.chat_id) setOlderLoading(false);
    }
  }

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
    if (!accountRef.current || item.user_id !== accountRef.current || sendingIdsRef.current.has(item.client_message_id)) return;
    sendingIdsRef.current.add(item.client_message_id);
    outboxRetryRef.current.delete(item.client_message_id); // Explicit user sends/retries bypass backoff.
    setMessages((current) => current.map((message) => message.client_message_id === item.client_message_id ? { ...message, local_status: 'sending' } : message));
    try {
      const session = await getCurrentSession();
      if (!session || session.user_id !== item.user_id) throw new Error('Account changed');
      const result = await sendTextMessage(item.chat_id, item.body, item.client_message_id, item.reply_to_id ?? '');
      await removeOutbox(item.user_id, item.client_message_id);
      outboxRetryRef.current.delete(item.client_message_id);
      if (selectedIdRef.current === item.chat_id) {
        setMessages((current) => [...current.filter((message) => message.client_message_id !== item.client_message_id), result.message].sort((a, b) => a.sequence - b.sequence));
      }
      await refreshChats();
    } catch {
      outboxRetryRef.current.set(item.client_message_id,
        nextOutboxRetry(outboxRetryRef.current.get(item.client_message_id), Date.now()));
      if (selectedIdRef.current === item.chat_id) {
        setMessages((current) => current.map((message) => message.client_message_id === item.client_message_id ? { ...message, local_status: 'failed' } : message));
      }
    } finally {
      sendingIdsRef.current.delete(item.client_message_id);
    }
  }

  async function send(event: FormEvent) {
    event.preventDefault();
    if (!selected || !accountRef.current || accountRef.current !== userId) return;
    const text = draft.trim();
    if (!text) return;

    if (editing) {
      try {
        const updated = await editChatMessage(editing.id, text);
        setMessages((current) => current.map((item) => item.id === updated.id ? updated : item));
        setDraft('');
        setEditing(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Не удалось изменить сообщение');
      }
      return;
    }

    const item: OutboxItem = {
      user_id: accountRef.current,
      chat_id: selected.chat_id,
      client_message_id: crypto.randomUUID(),
      body: text,
      reply_to_id: replyingTo?.id,
      created_at: new Date().toISOString()
    };
    try {
      await enqueueOutbox(item);
    } catch {
      setError('Не удалось сохранить сообщение. Проверьте свободное место и очередь неотправленных сообщений.');
      return;
    }
    setDraft('');
    setReplyingTo(null);
    if (selectedIdRef.current === item.chat_id) {
      setMessages((current) => [...current, asLocalMessage(item, userId)]);
    }
    // The item is durably queued under the original chat even if the user
    // navigated away while IndexedDB was committing it.
    if (navigator.onLine) await deliver(item);
  }

  async function retry(message: LocalMessage) {
    const item = await findOutbox(accountRef.current, message.client_message_id).catch(() => null);
    if (item) await deliver(item);
  }

  function beginReply(message: LocalMessage) {
    if (message.local_status || message.deleted_at) return;
    setEditing(null);
    setReplyingTo(message);
  }

  function beginEdit(message: LocalMessage) {
    if (message.local_status || message.deleted_at || message.sender_id !== userId) return;
    setReplyingTo(null);
    setEditing(message);
    setDraft(message.body);
  }

  async function removeMessage(message: LocalMessage) {
    if (message.local_status || message.sender_id !== userId) return;
    try {
      await deleteChatMessage(message.id);
      setMessages((current) => current.map((item) => item.id === message.id ? { ...item, body: '', deleted_at: new Date().toISOString() } : item));
      if (editing?.id === message.id) {
        setEditing(null);
        setDraft('');
      }
      if (replyingTo?.id === message.id) setReplyingTo(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось удалить сообщение');
    }
  }

  function cancelComposerContext() {
    setReplyingTo(null);
    setEditing(null);
    setDraft('');
  }

  return (
    <AppShell active="Чаты" wide>
      <div className={`messenger ${selected ? 'hasConversation' : ''}`}>
        <section className="conversationListPane">
          <header className="screenHeader">
            <div><span className="eyebrow">CHAT</span><h1>{t.chats}</h1></div>
            <IconButton type="button" aria-label={t.newChat} onClick={() => setShowNewChat((value) => !value)}>＋</IconButton>
          </header>
          <div className="screenSearch"><SearchField value={search} onChange={(e) => setSearch(e.target.value)} aria-label={t.searchChats} placeholder={t.searchChats} /></div>

          {showNewChat ? (
            <form className="newChatForm" onSubmit={createChat}>
              <input autoFocus value={newUsername} onChange={(e) => setNewUsername(e.target.value)} placeholder="@username" aria-label={t.username} />
              <Button type="submit">{t.open}</Button>
            </form>
          ) : null}
          {error ? <div className="messengerError" role="alert">{error}</div> : null}

          <div className="chatList" aria-label={t.chats}>
            {loading ? <div className="chatListState">{t.loadingChats}</div> : null}
            {!loading && filteredChats.length === 0 ? <div className="chatListState">{t.emptyChats}</div> : null}
            {filteredChats.map((chat) => (
              <button className={`chatRow chatRowButton ${selected?.chat_id === chat.chat_id ? 'isSelected' : ''}`} key={chat.chat_id} onClick={() => chooseChat(chat)}>
                <Avatar name={chatName(chat)} />
                <div className="chatCopy">
                  <div className="chatHeadline"><strong>{chatName(chat)}</strong><time>{formatTime(chat.last_message?.created_at ?? chat.updated_at, language)}</time></div>
                  <div className="chatPreview"><span>{chat.last_message?.body || (chat.kind === 'direct' ? `@${chat.peer_username}` : 'Новый чат')}</span>{chat.unread_count > 0 ? <Badge>{chat.unread_count}</Badge> : null}</div>
                </div>
              </button>
            ))}
          </div>
        </section>

        <section className="conversationPane">
          {!selected ? (
            <div className="conversationPlaceholder"><div aria-hidden="true">◇</div><strong>{t.chooseChat}</strong><span>{t.chooseChatHint}</span></div>
          ) : (
            <>
              <header className="conversationHeader">
                <IconButton className="conversationBack" type="button" aria-label={t.back} onClick={() => chooseChat(null)}>‹</IconButton>
                <Avatar name={chatName(selected)} size="sm" />
                <div className="conversationIdentity"><strong>{chatName(selected)}</strong>{selected.peer_username ? <span>@{selected.peer_username}</span> : null}</div>
              </header>
              <div className="messageList" aria-live="polite">
                {hasOlder ? <button className="loadOlderButton" type="button" onClick={loadOlder} disabled={olderLoading}>{olderLoading ? t.loading : t.older}</button> : null}
                {messageLoading ? <div className="chatListState">{t.loadingHistory}</div> : null}
                {messages.map((message) => {
                  const mine = message.sender_id === userId;
                  const replied = message.reply_to_id ? messageByID.get(message.reply_to_id) : undefined;
                  return (
                    <div className={`messageRow ${mine ? 'isMine' : ''}`} key={message.client_message_id}>
                      <div className={`messageBubble ${message.local_status === 'failed' ? 'isFailed' : ''}`}>
                        {replied ? <button className="messageReplyPreview" type="button" onClick={() => document.getElementById(`message-${replied.id}`)?.scrollIntoView({ block: 'center' })}>{replied.body || 'Сообщение удалено'}</button> : null}
                        <p id={`message-${message.id}`}>{message.deleted_at ? t.removed : message.body}</p>
                        <div className="messageMeta">
                          <time>{formatTime(message.created_at, language)}</time>
                          {message.edited_at ? <span>{t.edited}</span> : null}
                          {message.local_status === 'sending' ? <span>{t.sending}</span> : null}
                          {message.local_status === 'failed' ? <button type="button" onClick={() => void retry(message)}>{t.retry}</button> : null}
                        </div>
                        {!message.local_status && !message.deleted_at ? (
                          <div className="messageActions">
                            <button type="button" onClick={() => beginReply(message)}>{t.reply}</button>
                            {mine ? <button type="button" onClick={() => beginEdit(message)}>{t.edit}</button> : null}
                            {mine ? <button type="button" className="danger" onClick={() => void removeMessage(message)}>{t.delete}</button> : null}
                          </div>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
              {(replyingTo || editing) ? (
                <div className="composerContext">
                  <div><strong>{editing ? t.editing : t.answer}</strong><span>{editing ? editing.body : replyingTo?.body}</span></div>
                  <button type="button" onClick={cancelComposerContext} aria-label={t.cancel}>✕</button>
                </div>
              ) : null}
              <form className="messageComposer" onSubmit={send}>
                
                <textarea value={draft} onChange={(e) => setDraft(e.target.value)} rows={1} maxLength={4096} placeholder={editing ? t.edit : t.message} aria-label={t.message} onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); e.currentTarget.form?.requestSubmit(); } }} />
                <button className="sendMessageButton" type="submit" disabled={!draft.trim()} aria-label={editing ? t.save : t.send}>{editing ? '✓' : '↑'}</button>
              </form>
            </>
          )}
        </section>
      </div>
    </AppShell>
  );
}