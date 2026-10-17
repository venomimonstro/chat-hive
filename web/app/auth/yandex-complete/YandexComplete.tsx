'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { refreshSession } from '../../../lib/api';
import { useLanguage } from '../../../lib/language';

export function YandexComplete() {
  const { language } = useLanguage();
  const en = language === 'en';
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
        <h1>{en ? 'Sign-in incomplete' : 'Вход не завершён'}</h1>
        <p>{error}</p>
        <a className="uiButton uiButton--primary" href="/login">{en ? 'Back to sign in' : 'Вернуться ко входу'}</a>
      </div>
    );
  }

  return (
    <div className="authCallbackState" role="status" aria-live="polite">
      <div className="authSpinner" aria-hidden="true" />
      <h1>{en ? 'Connecting Yandex ID…' : 'Подключаем Яндекс ID…'}</h1>
      <p>{en ? 'Creating a secure CHAT session.' : 'Создаём защищённую сессию CHAT.'}</p>
    </div>
  );
}
