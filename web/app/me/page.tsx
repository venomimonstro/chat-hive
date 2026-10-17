'use client';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Button } from '../../components/ui';
import { getProfile, listSessions, revokeSession, type Profile } from '../../lib/api';
import { useLanguage } from '../../lib/language';

export default function AccountPage() {
  const router = useRouter();
  const {t} = useLanguage();
  const [profile,setProfile] = useState<Profile|null>(null);
  const [loading,setLoading] = useState(true);
  const [error,setError] = useState('');
  const [busy,setBusy] = useState(false);
  useEffect(()=>{
    let active=true;
    void getProfile().then(value=>{
      if(!active)return;
      if(!value.completed){router.replace('/onboarding');return;}
      setProfile(value);
    }).catch(err=>{if(active)setError(err instanceof Error?err.message:t.accountError)})
      .finally(()=>{if(active)setLoading(false)});
    return ()=>{active=false};
  },[router]);
  async function logout(){
    if(busy)return;
    setBusy(true);setError('');
    try {
      const current=await listSessions();
      await revokeSession(current.current_session_id,true);
      window.location.replace('/login');
    } catch(e){setError(e instanceof Error?e.message:t.accountError);setBusy(false)}
  }
  return <AppShell active="Я">
    <section className="coreSettings">
      <a href="/" className="coreBack">← {t.chats}</a>
      <h1>{t.account}</h1>
      {loading ? <p>{t.loading}</p>: null}
      {profile ? <>
        <div className="coreAccountIdentity">
          <div className="coreAccountAvatar" aria-hidden="true">{profile.display_name.slice(0,1).toUpperCase()}</div>
          <div><strong>{profile.display_name}</strong><span>@{profile.username}</span></div>
        </div>
        <div className="coreSettingsList">
          <a href="/settings/profile"><span>◯</span>{t.editProfile}<b aria-hidden="true">›</b></a>
          <a href="/settings/language"><span>文</span>{t.language}<b aria-hidden="true">›</b></a>
          <a href="/settings/security"><span>◇</span>{t.security}<b aria-hidden="true">›</b></a>
          <a href="/requests"><span>✉</span>{t.messageRequests}<b aria-hidden="true">›</b></a>
        </div>
        <Button variant="ghost" disabled={busy} onClick={()=>void logout()}>{t.logOut}</Button>
      </>: null}
      {error ? <p className="authError" role="alert">{error}</p>:null}
    </section>
  </AppShell>;
}
