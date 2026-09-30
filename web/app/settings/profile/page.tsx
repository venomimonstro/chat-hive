'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button } from '../../../components/ui';
import { completeOnboarding, getProfile, Interest, listInterests } from '../../../lib/api';

export default function EditProfilePage() {
  const router = useRouter();
  const [interests, setInterests] = useState<Interest[]>([]);
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [bio, setBio] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    Promise.all([getProfile(), listInterests()])
      .then(([profile, options]) => {
        if (!profile) {
          router.replace('/login');
          return;
        }
        setUsername(profile.username);
        setDisplayName(profile.display_name);
        setBio(profile.bio);
        setSelected(profile.interests);
        setInterests(options);
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Не удалось загрузить профиль'))
      .finally(() => setLoading(false));
  }, []);

  function toggleInterest(slug: string) {
    setSelected((current) => current.includes(slug) ? current.filter((item) => item !== slug) : current.length < 12 ? [...current, slug] : current);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      const profile = await completeOnboarding({
        username: username.trim(), display_name: displayName.trim(), bio: bio.trim(), interests: selected
      });
      router.replace(`/u/${profile.username}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось сохранить профиль');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell active="Я">
      <header className="screenHeader">
        <div><span className="eyebrow">ПРОФИЛЬ</span><h1>Редактировать</h1></div>
        <a href="/me" aria-label="Закрыть">✕</a>
      </header>
      {loading ? <p className="profileState">Загружаем…</p> : (
        <form className="onboardingForm profileEditForm" onSubmit={submit}>
          <label><span>Username</span><input value={username} onChange={(event) => setUsername(event.target.value.toLowerCase())} maxLength={32} autoComplete="username" /></label>
          <label><span>Имя</span><input value={displayName} onChange={(event) => setDisplayName(event.target.value)} maxLength={80} /></label>
          <label><span>О себе</span><textarea value={bio} onChange={(event) => setBio(event.target.value)} maxLength={500} rows={4} /></label>
          <fieldset className="interestFieldset">
            <legend>Интересы <small>{selected.length}/12</small></legend>
            <div className="interestGrid">
              {interests.map((interest) => (
                <button type="button" className={selected.includes(interest.slug) ? 'isSelected' : ''} onClick={() => toggleInterest(interest.slug)} key={interest.slug}>
                  {interest.label_ru}
                </button>
              ))}
            </div>
          </fieldset>
          {selected.length < 3 ? <p className="authError">Выберите минимум 3 интереса.</p> : null}
          {error ? <p className="authError" role="alert">{error}</p> : null}
          <Button type="submit" fullWidth disabled={busy || selected.length < 3 || !username.trim() || !displayName.trim()}>{busy ? 'Сохраняем…' : 'Сохранить'}</Button>
        </form>
      )}
    </AppShell>
  );
}
