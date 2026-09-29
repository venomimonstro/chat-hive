'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { completeEmailLogin } from '../../../lib/api';

export function AuthCallback() {
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
        <h1>Не удалось войти</h1>
        <p>{error}</p>
        <a className="uiButton uiButton--primary" href="/login">Получить новую ссылку</a>
      </div>
    );
  }

  return (
    <div className="authCallbackState" role="status" aria-live="polite">
      <div className="authSpinner" aria-hidden="true" />
      <h1>Входим в CHAT…</h1>
      <p>Проверяем одноразовую ссылку и создаём защищённую сессию.</p>
    </div>
  );
}
