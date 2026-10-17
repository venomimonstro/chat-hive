'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { DeviceSession, listSessions, revokeSession } from '../../../lib/api';
import { deleteAccount, downloadAccountExport } from '../../../lib/accountData';
import { deletePasskey, listPasskeys, PasskeyInfo, registerPasskey } from '../../../lib/passkeys';
import { useLanguage } from '../../../lib/language';
const securityEN: Record<string,string> = {
  "БЕЗОПАСНОСТЬ И ДАННЫЕ": "SECURITY & DATA",
  "Ваш аккаунт": "Your account",
  "Управляйте активными входами и данными, которые относятся к вашему аккаунту CHAT.": "Manage your active sessions and data associated with your CHAT account.",
  "Быстрый вход по биометрии или PIN устройства. Email остаётся резервным способом восстановления.": "Sign in using your device biometrics or PIN. Email remains an account recovery option.",
  "Добавляем…": "Adding…",
  "Добавить passkey": "Add passkey",
  "Добавлен ": "Added ",
  "использован ": "used ",
  "Удаляем…": "Removing…",
  "Удалить": "Remove",
  "Passkey ещё не добавлен": "No passkey registered",
  "Добавьте passkey на доверенном устройстве, чтобы входить без ссылки из почты.": "Add a passkey on a trusted device to sign in without email links.",
  "Ваши устройства": "Your devices",
  "Незнакомую сессию можно завершить сразу.": "You can revoke any unfamiliar device session.",
  "Загружаем активные сессии…": "Loading sessions…",
  "Текущая": "Current",
  "IP не определён": "IP unknown",
  "активность ": "last active ",
  "Подробнее": "Details",
  "User-Agent не передан": "User agent not provided",
  "Завершаем…": "Revoking…",
  "Выйти": "Sign out",
  "Завершить": "Revoke",
  "Ваши данные": "Your data",
  "Экспорт содержит профиль, ваши публикации, созданные сообщения, связи, сообщества и историю сессий.": "Export includes your profile, messages, account relationships and sessions.",
  "Скачать данные": "Download your data",
  "Получите машиночитаемую копию данных вашего аккаунта в JSON.": "Get a machine-readable JSON copy of your account data.",
  "Готовим…": "Preparing…",
  "Скачать": "Download",
  "Удалить аккаунт": "Delete account",
  "Идентичности, интересы, социальные связи и активные сессии будут удалены или отозваны. Некоторые записи могут сохраняться в обезличенном виде для целостности сервиса и обязательного хранения.": "Your profile and active sessions will be deleted or revoked. Some records may remain anonymized for service integrity and mandatory retention.",
  "Для подтверждения введите ": "Type to confirm ",
  "Не удалось загрузить устройства": "Could not load devices",
  "Не удалось завершить сессию": "Could not revoke session",
  "Не удалось добавить passkey": "Could not add passkey",
  "Удалить этот passkey? Вход по нему перестанет работать.": "Remove this passkey? It will no longer work for sign-in.",
  "Не удалось удалить passkey": "Could not remove passkey",
  "Не удалось скачать данные": "Could not download data",
  "Не удалось удалить аккаунт": "Could not delete account",
  "Старая сессия": "Legacy session",
  "Браузер": "Browser"
};


const DELETE_PHRASE = 'DELETE MY CHAT ACCOUNT';

function authMethodLabel(method: DeviceSession['auth_method']) {
  switch (method) {
    case 'passkey': return 'Passkey';
    case 'yandex': return 'Яндекс ID';
    case 'email': return 'Email';
    default: return 'Старая сессия';
  }
}

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
  const { language, t } = useLanguage();
  const tr = (russian: string) => language === 'en' ? (securityEN[russian] ?? russian) : russian;
  const locale = language === 'en' ? 'en-US' : 'ru-RU';
  const router = useRouter();
  const [items, setItems] = useState<DeviceSession[]>([]);
  const [currentId, setCurrentId] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [exporting, setExporting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmation, setConfirmation] = useState('');
  const [passkeys, setPasskeys] = useState<PasskeyInfo[]>([]);
  const [passkeyBusy, setPasskeyBusy] = useState('');

  async function load() {
    try {
      const [payload, passkeyItems] = await Promise.all([listSessions(), listPasskeys()]);
      setItems(payload.items);
      setCurrentId(payload.current_session_id);
      setPasskeys(passkeyItems);
    } catch (err) {
      setError(err instanceof Error ? err.message : tr("Не удалось загрузить устройства"));
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
      setError(err instanceof Error ? err.message : tr("Не удалось завершить сессию"));
    } finally {
      setBusy('');
    }
  }

  async function addPasskey() {
    if (passkeyBusy) return;
    setPasskeyBusy('add');
    setError('');
    try {
      await registerPasskey();
      setPasskeys(await listPasskeys());
    } catch (err) {
      setError(err instanceof Error ? err.message : tr("Не удалось добавить passkey"));
    } finally {
      setPasskeyBusy('');
    }
  }

  async function removePasskey(passkey: PasskeyInfo) {
    if (passkeyBusy) return;
    if (!window.confirm(tr("Удалить этот passkey? Вход по нему перестанет работать."))) return;
    setPasskeyBusy(passkey.credential_id);
    setError('');
    try {
      await deletePasskey(passkey.credential_id);
      setPasskeys((current) => current.filter((item) => item.credential_id !== passkey.credential_id));
    } catch (err) {
      setError(err instanceof Error ? err.message : tr("Не удалось удалить passkey"));
    } finally {
      setPasskeyBusy('');
    }
  }

  async function exportData() {
    if (exporting) return;
    setExporting(true);
    setError('');
    try {
      await downloadAccountExport();
    } catch (err) {
      setError(err instanceof Error ? err.message : tr("Не удалось скачать данные"));
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
      setError(err instanceof Error ? err.message : tr("Не удалось удалить аккаунт"));
      setDeleting(false);
    }
  }

  return (
    <AppShell active="Я">
      <section className="securityScreen">
        <header className="securityHeader">
          <span className="eyebrow">{tr("БЕЗОПАСНОСТЬ И ДАННЫЕ")}</span>
          <h1>{tr("Ваш аккаунт")}</h1>
          <p>{tr("Управляйте активными входами и данными, которые относятся к вашему аккаунту CHAT.")}</p>
        </header>

        {error ? <div className="authError" role="alert">{error}</div> : null}

        <div className="securitySectionHeader">
          <div><h2>Passkeys</h2><p>{tr("Быстрый вход по биометрии или PIN устройства. Email остаётся резервным способом восстановления.")}</p></div>
          <Button variant="secondary" disabled={passkeyBusy === 'add'} onClick={() => void addPasskey()}>
            {passkeyBusy === 'add' ? tr("Добавляем…") : tr("Добавить passkey")}
          </Button>
        </div>
        <div className="sessionList">
          {passkeys.length ? passkeys.map((passkey) => (
            <Surface className="sessionCard" key={passkey.credential_id}>
              <div className="sessionIcon" aria-hidden="true">◇</div>
              <div className="sessionCopy">
                <div className="sessionTitle"><strong>{passkey.label || 'Passkey'}</strong></div>
                <small>{tr('Добавлен ')}{new Date(passkey.created_at).toLocaleString(locale)}{passkey.last_used_at ? ` · ${tr("использован ")}${new Date(passkey.last_used_at).toLocaleString(locale)}` : ''}</small>
              </div>
              <Button variant="secondary" disabled={passkeyBusy === passkey.credential_id} onClick={() => void removePasskey(passkey)}>
                {passkeyBusy === passkey.credential_id ? tr("Удаляем…") : tr("Удалить")}
              </Button>
            </Surface>
          )) : <Surface className="securityDataCard"><div><strong>{tr("Passkey ещё не добавлен")}</strong><p>{tr("Добавьте passkey на доверенном устройстве, чтобы входить без ссылки из почты.")}</p></div></Surface>}
        </div>

        <div className="securitySectionHeader">
          <div><h2>{tr("Ваши устройства")}</h2><p>{tr("Незнакомую сессию можно завершить сразу.")}</p></div>
        </div>
        {loading ? <div className="securityLoading">{tr("Загружаем активные сессии…")}</div> : null}

        <div className="sessionList">
          {items.map((session) => {
            const isCurrent = session.session_id === currentId;
            return (
              <Surface className="sessionCard" key={session.session_id}>
                <div className="sessionIcon" aria-hidden="true">{isCurrent ? '●' : '○'}</div>
                <div className="sessionCopy">
                  <div className="sessionTitle">
                    <strong>{tr(deviceLabel(session.user_agent))}</strong>
                    {isCurrent ? <span>{tr("Текущая")}</span> : null}
                  </div>
                  <small>{tr(authMethodLabel(session.auth_method))} · {session.last_ip || tr("IP не определён")} · {tr('активность ')}{new Date(session.last_seen_at).toLocaleString(locale)}</small>
                  <details>
                    <summary>{tr("Подробнее")}</summary>
                    <p>{session.user_agent || tr("User-Agent не передан")}</p>
                  </details>
                </div>
                <Button variant={isCurrent ? 'danger' : 'secondary'} disabled={busy === session.session_id} onClick={() => terminate(session)}>
                  {busy === session.session_id ? tr("Завершаем…") : isCurrent ? tr("Выйти") : tr("Завершить")}
                </Button>
              </Surface>
            );
          })}
        </div>

        <div className="securitySectionHeader securityDataHeader">
          <div><h2>{tr("Ваши данные")}</h2><p>{tr("Экспорт содержит профиль, ваши публикации, созданные сообщения, связи, сообщества и историю сессий.")}</p></div>
        </div>
        <Surface className="securityDataCard">
          <div><strong>{tr("Скачать данные")}</strong><p>{tr("Получите машиночитаемую копию данных вашего аккаунта в JSON.")}</p></div>
          <Button variant="secondary" disabled={exporting} onClick={() => void exportData()}>{exporting ? tr("Готовим…") : tr("Скачать")}</Button>
        </Surface>

        <Surface className="securityDangerCard">
          <div>
            <strong>{tr("Удалить аккаунт")}</strong>
            <p>{tr("Идентичности, интересы, социальные связи и активные сессии будут удалены или отозваны. Некоторые записи могут сохраняться в обезличенном виде для целостности сервиса и обязательного хранения.")}</p>
          </div>
          <label className="securityDeleteConfirm">
            <span>{tr('Для подтверждения введите ')}<code>{DELETE_PHRASE}</code></span>
            <input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" spellCheck={false} />
          </label>
          <Button variant="danger" disabled={deleting || confirmation !== DELETE_PHRASE} onClick={() => void removeAccount()}>{deleting ? tr("Удаляем…") : tr("Удалить аккаунт")}</Button>
        </Surface>
      </section>
    </AppShell>
  );
}
