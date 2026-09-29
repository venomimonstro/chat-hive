'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { DeviceSession, listSessions, revokeSession } from '../../../lib/api';

function deviceLabel(userAgent: string) {
  const ua = userAgent.toLowerCase();
  if (ua.includes('android')) return 'Android';
  if (ua.includes('iphone') || ua.includes('ipad')) return 'iPhone / iPad';
  if (ua.includes('windows')) return 'Windows';
  if (ua.includes('mac os') || ua.includes('macintosh')) return 'Mac';
  if (ua.includes('linux')) return 'Linux';
  return 'Браузер';
}

export function SecuritySessions() {
  const router = useRouter();
  const [items, setItems] = useState<DeviceSession[]>([]);
  const [currentId, setCurrentId] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

  async function load() {
    try {
      const payload = await listSessions();
      setItems(payload.items);
      setCurrentId(payload.current_session_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить устройства');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  async function terminate(session: DeviceSession) {
    if (busy) return;
    const isCurrent = session.session_id === currentId;
    setBusy(session.session_id);
    setError('');
    try {
      await revokeSession(session.session_id, isCurrent);
      if (isCurrent) {
        router.replace('/login');
        return;
      }
      setItems((current) => current.filter((item) => item.session_id !== session.session_id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось завершить сессию');
    } finally {
      setBusy('');
    }
  }

  return (
    <AppShell active="Я">
      <section className="securityScreen">
        <header className="securityHeader">
          <span className="eyebrow">БЕЗОПАСНОСТЬ</span>
          <h1>Ваши устройства</h1>
          <p>Здесь отображаются активные входы в CHAT. Незнакомую сессию можно завершить сразу.</p>
        </header>

        {error ? <div className="authError" role="alert">{error}</div> : null}
        {loading ? <div className="securityLoading">Загружаем активные сессии…</div> : null}

        <div className="sessionList">
          {items.map((session) => {
            const isCurrent = session.session_id === currentId;
            return (
              <Surface className="sessionCard" key={session.session_id}>
                <div className="sessionIcon" aria-hidden="true">{isCurrent ? '●' : '○'}</div>
                <div className="sessionCopy">
                  <div className="sessionTitle">
                    <strong>{deviceLabel(session.user_agent)}</strong>
                    {isCurrent ? <span>Текущая</span> : null}
                  </div>
                  <small>{session.last_ip || 'IP не определён'} · активность {new Date(session.last_seen_at).toLocaleString('ru-RU')}</small>
                  <details>
                    <summary>Подробнее</summary>
                    <p>{session.user_agent || 'User-Agent не передан'}</p>
                  </details>
                </div>
                <Button variant={isCurrent ? 'danger' : 'secondary'} disabled={busy === session.session_id} onClick={() => terminate(session)}>
                  {busy === session.session_id ? 'Завершаем…' : isCurrent ? 'Выйти' : 'Завершить'}
                </Button>
              </Surface>
            );
          })}
        </div>
      </section>
    </AppShell>
  );
}
