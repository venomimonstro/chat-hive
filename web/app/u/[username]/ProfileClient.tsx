'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Avatar, Button, Surface } from '../../../components/ui';
import {
  ensureDirectChat, getAccessToken, getPublicProfile, PublicProfile, refreshSession, setBlock, setFollow
} from '../../../lib/api';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type ProfilePost = {
  id: string;
  kind: 'thought' | 'photo' | 'post';
  body: string;
  replies_count: number;
  reactions_count: number;
  created_at: string;
};

async function loadPosts(username: string): Promise<ProfilePost[] | null> {
  let access = getAccessToken();
  if (!access) access = (await refreshSession())?.access_token ?? null;
  if (!access) return null;
  let response = await fetch(`${API_BASE}/api/v1/profiles/${encodeURIComponent(username)}/posts?limit=30`, {
    credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${access}` }
  });
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) return null;
    response = await fetch(`${API_BASE}/api/v1/profiles/${encodeURIComponent(username)}/posts?limit=30`, {
      credentials: 'include', cache: 'no-store', headers: { Accept: 'application/json', Authorization: `Bearer ${refreshed.access_token}` }
    });
  }
  if (!response.ok) throw new Error('Не удалось загрузить публикации');
  const payload = await response.json() as { items: ProfilePost[] };
  return payload.items;
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export function ProfileClient({ username }: { username: string }) {
  const router = useRouter();
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [posts, setPosts] = useState<ProfilePost[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [nextProfile, nextPosts] = await Promise.all([getPublicProfile(username), loadPosts(username)]);
      setProfile(nextProfile);
      setPosts(nextPosts ?? []);
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

  async function messageUser() {
    if (!profile || profile.is_self || profile.is_blocked || busy) return;
    setBusy(true);
    try {
      const chat = await ensureDirectChat(profile.username);
      router.push(`/?chat=${encodeURIComponent(chat.chat_id)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось открыть диалог');
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
                  <Button onClick={messageUser} disabled={busy || profile.is_blocked} variant="secondary">Написать</Button>
                  <Button onClick={toggleBlock} disabled={busy} variant="ghost">
                    {profile.is_blocked ? 'Разблокировать' : 'Заблокировать'}
                  </Button>
                </div>
              ) : <a className="uiButton uiButton--secondary" href="/settings/profile">Редактировать профиль</a>}
            </div>

            <div className="profileStats">
              <a href={`/u/${profile.username}/connections?tab=followers`}><strong>{profile.followers_count}</strong><span>подписчиков</span></a>
              <a href={`/u/${profile.username}/connections?tab=following`}><strong>{profile.following_count}</strong><span>подписок</span></a>
            </div>

            {profile.bio ? <p className="profileBio">{profile.bio}</p> : null}
            <div className="profileInterests">
              {profile.interests.map((item) => <span key={item}>{item}</span>)}
            </div>

            {error ? <div className="authError" role="alert">{error}</div> : null}

            <section className="profilePostList" aria-label="Публикации">
              <div className="profilePostHeading"><h2>Публикации</h2>{profile.is_self ? <a href="/create/post?kind=thought">Создать</a> : null}</div>
              {posts.length === 0 ? (
                <Surface className="profileEmptyContent">
                  <h2>Публикаций пока нет</h2>
                  <p>{profile.is_self ? 'Напишите первую мысль — она появится здесь и сможет попасть в Discover.' : 'Автор пока ничего не публиковал.'}</p>
                </Surface>
              ) : null}
              {posts.map((post) => (
                <a className="profilePostCard" href={`/post/${post.id}`} key={post.id}>
                  <div className="profilePostMeta"><span>{post.kind === 'thought' ? 'Мысль' : 'Пост'}</span><time>{formatDate(post.created_at)}</time></div>
                  <p>{post.body}</p>
                  <div className="profilePostStats"><span>♡ {post.reactions_count}</span><span>💬 {post.replies_count}</span></div>
                </a>
              ))}
            </section>
          </>
        ) : null}
      </section>
    </AppShell>
  );
}
