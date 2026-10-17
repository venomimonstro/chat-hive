'use client';
import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Button } from '../../../components/ui';
import { completeOnboarding, getProfile } from '../../../lib/api';
import { useLanguage } from '../../../lib/language';

export default function EditProfilePage() {
  const router=useRouter();
  const {t}=useLanguage();
  const [username,setUsername]=useState('');
  const [displayName,setDisplayName]=useState('');
  const [bio,setBio]=useState('');
  const [loading,setLoading]=useState(true);
  const [loadFailed,setLoadFailed]=useState(false);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  useEffect(()=>{
    let active=true;
    void getProfile().then(profile=>{
      if(!active)return;
      if(!profile.completed){router.replace('/onboarding');return}
      setUsername(profile.username);setDisplayName(profile.display_name);setBio(profile.bio);
    }).catch(err=>{if(active){setError(err instanceof Error?err.message:t.accountError);setLoadFailed(true);}})
      .finally(()=>{if(active)setLoading(false)});
    return ()=>{active=false};
  },[router]);
  async function save(event:FormEvent) {
    event.preventDefault();
    if(busy||loadFailed)return;
    setBusy(true);setError('');
    try {
      await completeOnboarding({username:username.trim(),display_name:displayName.trim(),bio:bio.trim(),interests:[]});
      router.replace('/me');
    } catch(err){setError(err instanceof Error?err.message:t.accountError)}
    finally {setBusy(false)}
  }
  return <AppShell active="Я">
    <section className="coreSettings">
      <a href="/me" className="coreBack">← {t.account}</a>
      <h1>{t.editProfile}</h1>
      {loading ? <p>{t.loading}</p> : loadFailed ?
        <Button onClick={()=>window.location.reload()}>{t.retryLoad}</Button> :
        <form className="onboardingForm profileEditForm" onSubmit={save}>
          <label><span>{t.displayName}</span><input value={displayName} maxLength={80} required
            onChange={e=>setDisplayName(e.target.value)}/></label>
          <label><span>{t.username}</span><div className="usernameField"><span>@</span>
            <input value={username} maxLength={32} required
              onChange={e=>setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g,''))}/></div></label>
          <label><span>{t.about}</span><textarea rows={3} maxLength={240} value={bio} onChange={e=>setBio(e.target.value)}/></label>
          <Button type="submit" fullWidth disabled={busy||!displayName.trim()||!/^[a-z0-9_]{3,32}$/.test(username)}>
            {busy?t.saving:t.saveChanges}</Button>
        </form>
      }
      {error?<p className="authError" role="alert">{error}</p>:null}
    </section>
  </AppShell>;
}
