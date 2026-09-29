'use client';

import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, Surface } from '../../components/ui';
import { completeOnboarding, getProfile, Interest, listInterests } from '../../lib/api';

export function OnboardingForm() {
  const router = useRouter();
  const [interests, setInterests] = useState<Interest[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [bio, setBio] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    Promise.all([listInterests(), getProfile()])
      .then(([items, profile]) => {
        setInterests(items);
        if (profile?.completed) {
          router.replace('/');
          return;
        }
        if (profile) {
          setUsername(profile.username ?? '');
          setDisplayName(profile.display_name ?? '');
          setBio(profile.bio ?? '');
          setSelected(profile.interests ?? []);
        }
      })
      .catch(() => setError('Не удалось загрузить данные. Обновите страницу.'))
      .finally(() => setLoading(false));
  }, [router]);

  const canSubmit = useMemo(() => {
    return /^[a-z0-9_]{3,32}$/.test(username) && displayName.trim().length > 0 && selected.length >= 3 && selected.length <= 12;
  }, [displayName, selected.length, username]);

  function toggleInterest(slug: string) {
    setSelected((current) => current.includes(slug) ? current.filter((item) => item !== slug) : current.length < 12 ? [...current, slug] : current);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!canSubmit || saving) return;
    setSaving(true);
    setError('');
    try {
      await completeOnboarding({
        username: username.toLowerCase().trim(),
        display_name: displayName.trim(),
        bio: bio.trim(),
        interests: selected
      });
      router.replace('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось сохранить профиль');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <Surface className="onboardingCard"><div className="authSpinner" /><p>Подготавливаем профиль…</p></Surface>;
  }

  return (
    <Surface className="onboardingCard">
      <div className="onboardingHeader">
        <span className="eyebrow">ПЕРВЫЙ ЗАПУСК</span>
        <h1>Соберите свой профиль</h1>
        <p>Имя, адрес профиля и интересы нужны, чтобы CHAT сразу показал релевантных людей и сообщества.</p>
      </div>

      <form className="onboardingForm" onSubmit={submit}>
        <div className="onboardingGrid">
          <label>
            <span>Имя</span>
            <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} maxLength={80} required placeholder="Ольга" />
          </label>
          <label>
            <span>Username</span>
            <div className="usernameField"><span>@</span><input value={username} onChange={(e) => setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))} maxLength={32} required placeholder="olga" /></div>
          </label>
        </div>

        <label>
          <span>О себе <small>{bio.length}/240</small></span>
          <textarea value={bio} onChange={(e) => setBio(e.target.value)} maxLength={240} rows={3} placeholder="Коротко о себе — можно оставить пустым" />
        </label>

        <fieldset>
          <legend>Интересы <small>Выберите от 3 до 12</small></legend>
          <div className="interestGrid">
            {interests.map((interest) => {
              const active = selected.includes(interest.slug);
              return <button key={interest.slug} type="button" className={`interestChip${active ? ' isSelected' : ''}`} aria-pressed={active} onClick={() => toggleInterest(interest.slug)}>{interest.label_ru}</button>;
            })}
          </div>
        </fieldset>

        {error ? <div className="authError" role="alert">{error}</div> : null}
        <div className="onboardingFooter">
          <span>{selected.length} выбрано</span>
          <Button type="submit" disabled={!canSubmit || saving}>{saving ? 'Сохраняем…' : 'Начать пользоваться CHAT'}</Button>
        </div>
      </form>
    </Surface>
  );
}
