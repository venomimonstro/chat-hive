import type { ChatMessage } from './api';
import type { OutboxItem } from './outbox';

export type TimelineMessage = ChatMessage & { local_status?: 'sending' | 'failed' };

// Merge by the client-generated id so a delayed REST response cannot erase an
// in-flight IndexedDB write or resurrect an optimistic message after its ack.
export function reconcileTimeline(
  current: TimelineMessage[],
  fetchedNewestFirst: ChatMessage[],
  queued: OutboxItem[],
  userId: string,
  chatId: string,
  keepOlder = false
): TimelineMessage[] {
  const fetched = [...fetchedNewestFirst].reverse();
  const knownIDs = new Set(fetched.map((item) => item.client_message_id));
  const queue = queued.filter((item) => item.user_id === userId && item.chat_id === chatId);
  const queuedIDs = new Set(queue.map((item) => item.client_message_id));
  const oldByID = new Map(current.map((item) => [item.client_message_id, item]));
  const first = fetched[0]?.sequence;
  const last = fetched.at(-1)?.sequence;

  const retained = current.filter((item) => {
    if (item.chat_id !== chatId || knownIDs.has(item.client_message_id)) return false;
    // The queued read can finish before a concurrent optimistic send commits.
    if (item.local_status) return !queuedIDs.has(item.client_message_id);
    if (first === undefined) return false;
    return (keepOlder && item.sequence < first) ||
      (last !== undefined && item.sequence > last);
  });

  const pending: TimelineMessage[] = queue
    .filter((item) => !knownIDs.has(item.client_message_id))
    .map((item) => ({
      id: `local-${item.client_message_id}`,
      chat_id: chatId,
      sender_id: userId,
      client_message_id: item.client_message_id,
      sequence: Number.MAX_SAFE_INTEGER,
      type: 'text',
      body: item.body,
      reply_to_id: item.reply_to_id,
      created_at: item.created_at,
      local_status: oldByID.get(item.client_message_id)?.local_status === 'failed' ? 'failed' : 'sending'
    }));

  return [...retained, ...fetched, ...pending].sort(
    (a, b) => a.sequence - b.sequence || a.created_at.localeCompare(b.created_at)
  );
}
