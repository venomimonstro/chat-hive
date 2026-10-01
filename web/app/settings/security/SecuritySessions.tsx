'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { DeviceSession, listSessions, revokeSession } from '../../../lib/api';
import { deleteAccount, downloadAccountExport } from '../../../lib/accountData';

const DELETE_PHRASE = 'DELETE MY CHAT ACCOUNT';

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
  const [exporting, setExporting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmation, setConfirmation] = useState('');

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

  async function exportData() {
    if (exporting) return;
    setExporting(true);
    setError('');
    try {
      await downloadAccountExport();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось скачать данные');
    } finally {
      setExporting(false);
    }
  }

  async function removeAccount() {
    if (deleting || confirmation !== DELETE_PHRASE) return;
    setDeleting(true);
    setError('');
    try {
      await deleteAccount(confirmation);
      window.location.replace('/welcome');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось удалить аккаунт');
      setDeleting(false);
    }
  }

  return (
    <AppShell active="Я">
      <section className="securityScreen">
        <header className="securityHeader">
          <span className="eyebrow">БЕЗОПАСНОСТЬ И ДАННЫЕ</span>
          <h1>Ваш аккаунт</h1>
          <p>Управляйте активными входами и данными, которые относятся к вашему аккаунту CHAT.</p>
        </header>

        {error ? <div className="authError" role="alert">{error}</div> : null}

        <div className="securitySectionHeader">
          <div><h2>Ваши устройства</h2><p>Незнакомую сессию можно завершить сразу.</p></div>
        </div>
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

        <div className="securitySectionHeader securityDataHeader">
          <div><h2>Ваши данные</h2><p>Экспорт содержит профиль, ваши публикации, созданные сообщения, связи, сообщества и историю сессий.</p></div>
        </div>
        <Surface className="securityDataCard">
          <div><strong>Скачать данные</strong><p>Получите машиночитаемую копию данных вашего аккаунта в JSON.</p></div>
          <Button variant="secondary" disabled={exporting} onClick={() => void exportData()}>{exporting ? 'Готовим…' : 'Скачать'}</Button>
        </Surface>

        <Surface className="securityDangerCard">
          <div>
            <strong>Удалить аккаунт</strong>
            <p>Идентичности, интересы, социальные связи и активные сессии будут удалены или отозваны. Некоторые записи могут сохраняться в обезличенном виде для целостности сервиса и обязательного хранения.</p>
          </div>
          <label className="securityDeleteConfirm">
            <span>Для подтверждения введите <code>{DELETE_PHRASE}</code></span>
            <input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" spellCheck={false} />
          </label>
          <Button variant="danger" disabled={deleting || confirmation !== DELETE_PHRASE} onClick={() => void removeAccount()}>{deleting ? 'Удаляем…' : 'Удалить аккаунт'}</Button>
        </Surface>
      </section>
    </AppShell>
  );
}
