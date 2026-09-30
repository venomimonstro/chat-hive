'use client';

export type OutboxItem = {
  chat_id: string;
  client_message_id: string;
  body: string;
  created_at: string;
  reply_to_id?: string;
};

const DB_NAME = 'chat-client';
const DB_VERSION = 1;
const STORE = 'outbox';
const LEGACY_KEY = 'chat_outbox_v1';
const MAX_ITEMS = 500;
const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;

function available() {
  return typeof window !== 'undefined' && 'indexedDB' in window;
}

function openDB(): Promise<IDBDatabase> {
  if (!available()) return Promise.reject(new Error('IndexedDB unavailable'));
  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(DB_NAME, DB_VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: 'client_message_id' });
        store.createIndex('created_at', 'created_at');
        store.createIndex('chat_id', 'chat_id');
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('IndexedDB open failed'));
    request.onblocked = () => reject(new Error('IndexedDB upgrade blocked'));
  });
}

async function transaction<T>(mode: IDBTransactionMode, work: (store: IDBObjectStore, resolve: (value: T) => void, reject: (reason?: unknown) => void) => void): Promise<T> {
  const db = await openDB();
  return new Promise<T>((resolve, reject) => {
    const tx = db.transaction(STORE, mode);
    const store = tx.objectStore(STORE);
    let settled = false;
    const done = (value: T) => { if (!settled) { settled = true; resolve(value); } };
    const fail = (reason?: unknown) => { if (!settled) { settled = true; reject(reason); } };
    tx.onerror = () => fail(tx.error ?? new Error('IndexedDB transaction failed'));
    tx.onabort = () => fail(tx.error ?? new Error('IndexedDB transaction aborted'));
    work(store, done, fail);
  }).finally(() => db.close());
}

export async function enqueueOutbox(item: OutboxItem) {
  await transaction<void>('readwrite', (store, resolve, reject) => {
    const request = store.put(item);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
  });
  await pruneOutbox();
}

export async function removeOutbox(clientMessageId: string) {
  await transaction<void>('readwrite', (store, resolve, reject) => {
    const request = store.delete(clientMessageId);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
  });
}

export async function listOutbox(chatId?: string): Promise<OutboxItem[]> {
  const items = await transaction<OutboxItem[]>('readonly', (store, resolve, reject) => {
    const request = store.getAll();
    request.onsuccess = () => resolve((request.result ?? []) as OutboxItem[]);
    request.onerror = () => reject(request.error);
  });
  return items
    .filter((item) => !chatId || item.chat_id === chatId)
    .sort((a, b) => a.created_at.localeCompare(b.created_at));
}

export async function findOutbox(clientMessageId: string): Promise<OutboxItem | null> {
  return transaction<OutboxItem | null>('readonly', (store, resolve, reject) => {
    const request = store.get(clientMessageId);
    request.onsuccess = () => resolve((request.result as OutboxItem | undefined) ?? null);
    request.onerror = () => reject(request.error);
  });
}

export async function migrateLegacyOutbox() {
  if (typeof window === 'undefined') return;
  const raw = window.localStorage.getItem(LEGACY_KEY);
  if (!raw) return;
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      for (const value of parsed.slice(-MAX_ITEMS)) {
        if (!value || typeof value.chat_id !== 'string' || typeof value.client_message_id !== 'string' || typeof value.body !== 'string' || typeof value.created_at !== 'string') continue;
        await enqueueOutbox({
          chat_id: value.chat_id,
          client_message_id: value.client_message_id,
          body: value.body,
          created_at: value.created_at,
          reply_to_id: typeof value.reply_to_id === 'string' ? value.reply_to_id : undefined
        });
      }
    }
    window.localStorage.removeItem(LEGACY_KEY);
  } catch {
    // Preserve malformed legacy data rather than deleting it silently.
  }
}

export async function pruneOutbox() {
  const items = await listOutbox();
  const threshold = Date.now() - MAX_AGE_MS;
  const expired = items.filter((item) => {
    const time = Date.parse(item.created_at);
    return !Number.isFinite(time) || time < threshold;
  });
  const overflow = items.length > MAX_ITEMS ? items.slice(0, items.length - MAX_ITEMS) : [];
  const remove = new Set([...expired, ...overflow].map((item) => item.client_message_id));
  for (const id of remove) await removeOutbox(id);
}
