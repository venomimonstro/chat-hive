'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { getCurrentSession, getGroupInvitePreview, Group, GroupInvitePreview, joinGroupByInvite } from '../../../lib/api';
import { rememberReturnTo } from '../../../lib/returnTo';
import '../../welcome/welcome.css';

export function JoinClient() {
  const params = useParams<{ token: string }>();
  const token = typeof params.token === 'string' ? params.token : '';
  const [preview, setPreview] = useState<GroupInvitePreview | null>(null);
  const [authenticated, setAuthenticated] = useState(false);
  const [joining, setJoining] = useState(false);
  const [joined, setJoined] = useState<Group | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    if (!token) {
      setError('Некорректная ссылка приглашения');
      setLoading(false);
      return;
    }
    Promise.all([
      getGroupInvitePreview(token),
      getCurrentSession().catch(() => null)
    ]).then(([invite, session]) => {
      if (!active) return;
      setPreview(invite);
      setAuthenticated(Boolean(session));
    }).catch((err) => {
      if (!active) return;
      setError(err instanceof Error ? err.message : 'Приглашение недействительно или истекло');
    }).finally(() => active && setLoading(false));
    return () => { active = false; };
  }, [token]);

  function loginAndReturn() {
    const path = `/join/${encodeURIComponent(token)}`;
    rememberReturnTo(path);
    window.location.assign('/login');
  }

  async function join() {
    if (!token || joining) return;
    if (!authenticated) {
      loginAndReturn();
      return;
    }
    setJoining(true);
    setError('');
    try {
      const group = await joinGroupByInvite(token);
      setJoined(group);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось вступить в группу');
    } finally {
      setJoining(false);
    }
  }

  return (
    <main className="welcomePage">
      <header className="welcomeHeader">
        <a className="welcomeBrand" href="/welcome">CHAT</a>
        <nav><a className="welcomeLink" href="/welcome">О CHAT</a>{!authenticated ? <a className="welcomeButton welcomeButtonSmall" href="/login">Войти</a> : null}</nav>
      </header>

      <section className="inviteLanding">
        <div className="inviteCard">
          <span className="welcomeEyebrow">ПРИГЛАШЕНИЕ В CHAT</span>
          {loading ? <><h1>Проверяем приглашение…</h1><p>Это займёт секунду.</p></> : null}
          {!loading && error && !preview ? (
            <>
              <h1>Ссылка больше не работает</h1>
              <p>{error}</p>
              <a className="welcomeButton" href="/welcome">Открыть CHAT</a>
            </>
          ) : null}
          {!loading && preview && !joined ? (
            <>
              <h1>{preview.title}</h1>
              <p>{preview.description || 'Группа для общения в CHAT.'}</p>
              <div className="inviteMeta"><span>{preview.members_count} участников</span><span>Приватная переписка не показывается до вступления</span></div>
              {error ? <div className="inviteError" role="alert">{error}</div> : null}
              <button className="welcomeButton inviteAction" type="button" disabled={joining} onClick={() => void join()}>
                {joining ? 'Вступаем…' : authenticated ? 'Вступить в группу' : 'Войти и вступить'}
              </button>
              {!authenticated ? <p className="inviteHint">Аккаунт создаётся по email. После входа вы вернётесь прямо сюда.</p> : null}
            </>
          ) : null}
          {joined ? (
            <>
              <h1>Вы в группе</h1>
              <p>«{joined.title}» теперь находится среди ваших чатов.</p>
              <a className="welcomeButton" href={`/?chat=${encodeURIComponent(joined.chat_id)}`}>Открыть чат</a>
            </>
          ) : null}
        </div>
      </section>
    </main>
  );
}
