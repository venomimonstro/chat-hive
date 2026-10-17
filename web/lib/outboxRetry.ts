export type RetryState = { failures: number; nextRetryAt: number };

// The UI flushes on a five-second clock. Exponential backoff with a jitter
// prevents a stalled API from receiving every user's full offline queue at
// the same instant. No message is ever removed by this schedule.
export function nextOutboxRetry(previous: RetryState | undefined, now: number, jitter = Math.random()): RetryState {
  const failures = Math.min(30, (previous?.failures ?? 0) + 1);
  const baseDelay = Math.min(300_000, 5_000 * 2 ** Math.min(failures - 1, 6));
  const boundedJitter = Math.max(0, Math.min(1, Number.isFinite(jitter) ? jitter : 0));
  return { failures, nextRetryAt: now + Math.round(baseDelay * (0.8 + 0.2 * boundedJitter)) };
}

export function readyForOutboxRetry(state: RetryState | undefined, now: number): boolean {
  return !state || now >= state.nextRetryAt;
}
