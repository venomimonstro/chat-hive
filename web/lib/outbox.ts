'use client';

export type OutboxItem = {
  user_id: string;
  chat_id: string;
  client_message_id: string;
  body: string;
  created_at: string;
  reply_to_id?: string;
};

const DB_NAME = 'chat-client';
const DB_VERSION = 2;
const STORE = 'outbox_scoped';
const MAX_ITEMS = 500;
const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;

// Old records lack verified ownership; preserve them for manual recovery,
// but never replay another account's messages after a login change.
function openDB(): Promise<IDBDatabase> {
  if (typeof window === 'undefined' || !('indexedDB' in window)) {
    return Promise.reject(new Error('IndexedDB unavailable'));
  }
  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(DB_NAME, DB_VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: 'client_message_id' });
        store.createIndex('by_user', 'user_id');
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('IndexedDB open failed'));
    request.onblocked = () => reject(new Error('IndexedDB upgrade blocked; close other CHAT tabs'));
  });
}

// Only count as durable after the entire transaction commits, not the
// individual IDB request (which can succeed before a quota/abort failure).
async function transaction<T>(
  mode: IDBTransactionMode,
  work: (store: IDBObjectStore, done: (result: T) => void, fail: (error?: unknown) => void) => void
): Promise<T> {
  const db = await openDB();
  return new Promise<T>((resolve, reject) => {
    let tx: IDBTransaction;
    try { tx = db.transaction(STORE, mode); } catch (error) { reject(error); return; }
    let ready = false;
    let result: T | undefined;
    const done = (value: T) => { ready = true; result = value; };
    tx.oncomplete = () => ready ? resolve(result as T) : reject(new Error('IndexedDB request did not complete'));
    tx.onerror = () => reject(tx.error ?? new Error('IndexedDB transaction failed'));
    tx.onabort = () => reject(tx.error ?? new Error('IndexedDB transaction aborted'));
    try { work(tx.objectStore(STORE), done, reject); }
    catch (error) { try { tx.abort(); } catch { /* already finished */ } reject(error); }
  }).finally(() => db.close());
}

function requireUser(userId: string) {
  if (!userId || !userId.trim()) throw new Error('Account required for offline messages');
}

export async function enqueueOutbox(item: OutboxItem) {
  requireUser(item.user_id);
  await transaction<void>('readwrite', (store, done, fail) => {
    const request = store.put(item);
    request.onsuccess = () => done();
    request.onerror = () => fail(request.error);
  });
  await pruneOutbox(item.user_id);
}

export async function removeOutbox(userId: string, clientMessageId: string) {
  requireUser(userId);
  await transaction<void>('readwrite', (store, done, fail) => {
    const request = store.get(clientMessageId);
    request.onerror = () => fail(request.error);
    request.onsuccess = () => {
      const item = request.result as OutboxItem | undefined;
      if (!item || item.user_id !== userId) { done(); return; }
      const deletion = store.delete(clientMessageId);
      deletion.onsuccess = () => done();
      deletion.onerror = () => fail(deletion.error);
    };
  });
}

export async function listOutbox(userId: string, chatId?: string): Promise<OutboxItem[]> {
  requireUser(userId);
  const items = await transaction<OutboxItem[]>('readonly', (store, done, fail) => {
    const request = store.index('by_user').getAll(userId);
    request.onsuccess = () => done((request.result ?? []) as OutboxItem[]);
    request.onerror = () => fail(request.error);
  });
  return items
    .filter((item) => item.user_id === userId && (!chatId || item.chat_id === chatId))
    .sort((a, b) => a.created_at.localeCompare(b.created_at));
}

export async function findOutbox(userId: string, clientMessageId: string): Promise<OutboxItem | null> {
  requireUser(userId);
  const item = await transaction<OutboxItem | null>('readonly', (store, done, fail) => {
    const request = store.get(clientMessageId);
    request.onsuccess = () => done((request.result as OutboxItem | undefined) ?? null);
    request.onerror = () => fail(request.error);
  });
  return item?.user_id === userId ? item : null;
}

export async function pruneOutbox(userId: string) {
  const items = await listOutbox(userId);
  const threshold = Date.now() - MAX_AGE_MS;
  const expired = items.filter((item) => {
    const time = Date.parse(item.created_at);
    return !Number.isFinite(time) || time < threshold;
  });
  const overflow = items.length > MAX_ITEMS ? items.slice(0, items.length - MAX_ITEMS) : [];
  for (const id of new Set([...expired, ...overflow].map((item) => item.client_message_id))) {
    await removeOutbox(userId, id);
  }
}
