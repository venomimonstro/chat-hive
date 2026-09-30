'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { Group, joinGroupByInvite } from '../../../lib/api';

export default function JoinGroupPage() {
  const params = useParams<{ token: string }>();
  const token = params.token;
  const [joining, setJoining] = useState(false);
  const [group, setGroup] = useState<Group | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!token) setError('Некорректная ссылка приглашения');
  }, [token]);

  async function join() {
    if (!token || joining) return;
    setJoining(true);
    setError('');
    try {
      const joined = await joinGroupByInvite(token);
      setGroup(joined);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Приглашение недействительно');
    } finally {
      setJoining(false);
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">CHAT</span><h1>Приглашение</h1></div>
      </header>
      <div className="createPageBody">
        <Surface className="createFormCard">
          {group ? (
            <>
              <h2>Вы в группе «{group.title}»</h2>
              <p>{group.description || 'Теперь можно открыть чат и начать общение.'}</p>
              <a className="uiButton uiButton--primary uiButton--full" href={`/groups/${group.chat_id}`}>Открыть группу</a>
            </>
          ) : (
            <>
              <h2>Вас пригласили в группу CHAT</h2>
              <p>После вступления группа появится среди ваших чатов.</p>
              {error ? <div className="authError" role="alert">{error}</div> : null}
              <Button type="button" fullWidth disabled={joining || !token} onClick={join}>{joining ? 'Вступаем…' : 'Вступить в группу'}</Button>
            </>
          )}
        </Surface>
      </div>
    </AppShell>
  );
}
