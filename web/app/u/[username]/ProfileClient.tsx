'use client';

import { useEffect, useState } from 'react';
import { AppShell } from '../../../components/AppShell';
import { Avatar, Button, Surface } from '../../../components/ui';
import { getPublicProfile, PublicProfile, setBlock, setFollow } from '../../../lib/api';

export function ProfileClient({ username }: { username: string }) {
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      setProfile(await getPublicProfile(username));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Профиль недоступен');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, [username]);

  async function toggleFollow() {
    if (!profile || busy) return;
    setBusy(true);
    try {
      await setFollow(profile.username, !profile.is_following);
      setProfile({
        ...profile,
        is_following: !profile.is_following,
        followers_count: Math.max(0, profile.followers_count + (profile.is_following ? -1 : 1))
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Действие недоступно');
    } finally {
      setBusy(false);
    }
  }

  async function toggleBlock() {
    if (!profile || busy) return;
    const next = !profile.is_blocked;
    setBusy(true);
    try {
      await setBlock(profile.username, next);
      setProfile({
        ...profile,
        is_blocked: next,
        is_following: next ? false : profile.is_following,
        followers_count: next && profile.is_following ? Math.max(0, profile.followers_count - 1) : profile.followers_count
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Действие недоступно');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell active="Я">
      <section className="profileScreen">
        {loading ? <div className="profileState">Загружаем профиль…</div> : null}
        {!loading && error && !profile ? <div className="profileState"><h1>Профиль недоступен</h1><p>{error}</p></div> : null}
        {profile ? (
          <>
            <div className="profileHero">
              <Avatar name={profile.display_name || profile.username} size="lg" />
              <div className="profileIdentity">
                <h1>{profile.display_name || profile.username}</h1>
                <span>@{profile.username}</span>
              </div>
              {!profile.is_self ? (
                <div className="profileActions">
                  <Button onClick={toggleFollow} disabled={busy || profile.is_blocked} variant={profile.is_following ? 'secondary' : 'primary'}>
                    {profile.is_following ? 'Вы подписаны' : 'Подписаться'}
                  </Button>
                  <Button onClick={toggleBlock} disabled={busy} variant="ghost">
                    {profile.is_blocked ? 'Разблокировать' : 'Заблокировать'}
                  </Button>
                </div>
              ) : <Button variant="secondary">Редактировать профиль</Button>}
            </div>

            <div className="profileStats">
              <div><strong>{profile.followers_count}</strong><span>подписчиков</span></div>
              <div><strong>{profile.following_count}</strong><span>подписок</span></div>
            </div>

            {profile.bio ? <p className="profileBio">{profile.bio}</p> : null}
            <div className="profileInterests">
              {profile.interests.map((item) => <span key={item}>{item}</span>)}
            </div>

            {error ? <div className="authError" role="alert">{error}</div> : null}

            <Surface className="profileEmptyContent">
              <h2>Публикаций пока нет</h2>
              <p>Посты, фото и ответы появятся здесь после Sprint 11.</p>
            </Surface>
          </>
        ) : null}
      </section>
    </AppShell>
  );
}
