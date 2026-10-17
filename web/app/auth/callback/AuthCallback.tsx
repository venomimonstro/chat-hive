'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { completeEmailLogin } from '../../../lib/api';
import { useLanguage } from '../../../lib/language';

export function AuthCallback() {
  const { language } = useLanguage();
  const en = language === 'en';
  const params = useSearchParams();
  const router = useRouter();
  const [error, setError] = useState('');

  useEffect(() => {
    const token = params.get('token');
    if (!token) {
      setError('Ссылка для входа некорректна.');
      return;
    }

    let active = true;
    completeEmailLogin(token)
      .then(() => {
        if (active) router.replace('/onboarding');
      })
      .catch((err) => {
        if (active) setError(err instanceof Error ? err.message : 'Не удалось войти');
      });

    return () => { active = false; };
  }, [params, router]);

  if (error) {
    return (
      <div className="authCallbackState">
        <div className="authMark authMark--error">!</div>
        <h1>{en ? 'Sign-in failed' : 'Не удалось войти'}</h1>
        <p>{error}</p>
        <a className="uiButton uiButton--primary" href="/login">{en ? 'Get a new link' : 'Получить новую ссылку'}</a>
      </div>
    );
  }

  return (
    <div className="authCallbackState" role="status" aria-live="polite">
      <div className="authSpinner" aria-hidden="true" />
      <h1>{en ? 'Signing in…' : 'Входим в CHAT…'}</h1>
      <p>{en ? 'Verifying your one-time link and creating a secure session.' : 'Проверяем одноразовую ссылку и создаём защищённую сессию.'}</p>
    </div>
  );
}
