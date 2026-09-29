'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { refreshSession } from '../../../lib/api';

export function YandexComplete() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState('');

  useEffect(() => {
    if (params.get('error')) {
      setError('Вход через Яндекс не завершён. Попробуйте ещё раз.');
      return;
    }
    let active = true;
    refreshSession()
      .then((session) => {
        if (!active) return;
        if (!session) {
          setError('Не удалось создать сессию CHAT.');
          return;
        }
        router.replace('/onboarding');
      })
      .catch(() => active && setError('Не удалось завершить вход через Яндекс.'));
    return () => { active = false; };
  }, [params, router]);

  if (error) {
    return (
      <div className="authCallbackState">
        <div className="authMark authMark--error">!</div>
        <h1>Вход не завершён</h1>
        <p>{error}</p>
        <a className="uiButton uiButton--primary" href="/login">Вернуться ко входу</a>
      </div>
    );
  }

  return (
    <div className="authCallbackState" role="status" aria-live="polite">
      <div className="authSpinner" aria-hidden="true" />
      <h1>Подключаем Яндекс ID…</h1>
      <p>Создаём защищённую сессию CHAT.</p>
    </div>
  );
}
