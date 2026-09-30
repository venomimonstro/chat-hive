'use client';

export type DataSaverPreference = 'auto' | 'save' | 'maximum';
export type EffectiveDataMode = 'normal' | 'save' | 'maximum';

const STORAGE_KEY = 'chat:data-saver';
const EVENT_NAME = 'chat:data-saver-changed';

type NetworkInformationLike = {
  saveData?: boolean;
  effectiveType?: string;
};

function networkInfo(): NetworkInformationLike | null {
  if (typeof navigator === 'undefined') return null;
  return (navigator as Navigator & { connection?: NetworkInformationLike }).connection ?? null;
}

export function getDataSaverPreference(): DataSaverPreference {
  if (typeof window === 'undefined') return 'auto';
  const value = window.localStorage.getItem(STORAGE_KEY);
  return value === 'save' || value === 'maximum' ? value : 'auto';
}

export function setDataSaverPreference(value: DataSaverPreference) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(STORAGE_KEY, value);
  window.dispatchEvent(new CustomEvent(EVENT_NAME, { detail: value }));
}

export function getEffectiveDataMode(): EffectiveDataMode {
  const preference = getDataSaverPreference();
  if (preference === 'maximum') return 'maximum';
  if (preference === 'save') return 'save';
  const connection = networkInfo();
  if (connection?.saveData) return 'save';
  const type = String(connection?.effectiveType ?? '').toLowerCase();
  if (type === 'slow-2g' || type === '2g') return 'save';
  return 'normal';
}

export function subscribeDataMode(listener: (mode: EffectiveDataMode) => void) {
  if (typeof window === 'undefined') return () => undefined;
  const update = () => listener(getEffectiveDataMode());
  window.addEventListener(EVENT_NAME, update);
  window.addEventListener('online', update);
  window.addEventListener('offline', update);
  const connection = networkInfo() as (NetworkInformationLike & { addEventListener?: (type: string, callback: () => void) => void; removeEventListener?: (type: string, callback: () => void) => void }) | null;
  connection?.addEventListener?.('change', update);
  return () => {
    window.removeEventListener(EVENT_NAME, update);
    window.removeEventListener('online', update);
    window.removeEventListener('offline', update);
    connection?.removeEventListener?.('change', update);
  };
}
