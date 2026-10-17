'use client';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, Surface } from '../../components/ui';
import { completeOnboarding, getProfile } from '../../lib/api';
import { useLanguage } from '../../lib/language';

export function OnboardingForm() {
  const router = useRouter();
  const { t } = useLanguage();
  const [username,setUsername] = useState('');
  const [displayName,setDisplayName] = useState('');
  const [bio,setBio] = useState('');
  const [loading,setLoading] = useState(true);
  const [saving,setSaving] = useState(false);
  const [error,setError] = useState('');
  const [loadFailed,setLoadFailed] = useState(false);
  useEffect(() => {
    let active=true;
    void getProfile().then(profile => {
      if (!active) return;
      if (profile.completed) { router.replace('/'); return; }
      setUsername(profile.username ?? '');
      setDisplayName(profile.display_name ?? '');
      setBio(profile.bio ?? '');
    }).catch(() => { if(active){setError(t.accountError);setLoadFailed(true);} })
      .finally(() => {if(active)setLoading(false)});
    return () => {active=false};
  }, [router]);
  const valid=useMemo(() => /^[a-z0-9_]{3,32}$/.test(username) && !!displayName.trim()
    && Array.from(displayName.trim()).length <= 80,[username,displayName]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!valid || saving || loadFailed) return;
    setSaving(true);setError('');
    try {
      await completeOnboarding({username:username.trim(),display_name:displayName.trim(),bio:bio.trim(),interests:[]});
      router.replace('/');
    } catch(err) {setError(err instanceof Error?err.message:t.accountError)}
    finally {setSaving(false)}
  }
  if (loading) return <Surface className="onboardingCard"><p>{t.preparing}</p></Surface>;
  if (loadFailed) return <Surface className="onboardingCard"><p role="alert">{error}</p><Button onClick={()=>window.location.reload()}>{t.retryLoad}</Button></Surface>;
  return (
    <Surface className="onboardingCard coreOnboarding">
      <div className="onboardingHeader">
        <span className="eyebrow">CHAT</span><h1>{t.setupProfile}</h1><p>{t.setupHint}</p>
      </div>
      <form className="onboardingForm" onSubmit={submit}>
        <label><span>{t.displayName}</span><input autoFocus required value={displayName} maxLength={80}
          onChange={e=>setDisplayName(e.target.value)} autoComplete="name" /></label>
        <label><span>{t.username}</span><div className="usernameField"><span>@</span>
          <input required value={username} minLength={3} maxLength={32} autoComplete="username"
            onChange={e=>setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g,''))} /></div></label>
        <label><span>{t.about}</span><textarea rows={3} value={bio} maxLength={240} onChange={e=>setBio(e.target.value)} /></label>
        {!valid && (username.length > 0 || displayName.length > 0) ? <p className="authError">{t.requiredName}</p>:null}
        {error ? <p className="authError" role="alert">{error}</p> : null}
        <Button type="submit" fullWidth disabled={!valid||saving}>{saving?t.saving:t.startUsing}</Button>
      </form>
    </Surface>
  );
}
