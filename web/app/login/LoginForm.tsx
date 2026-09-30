'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { Button, Surface } from '../../components/ui';
import { startEmailLogin, startYandexLogin } from '../../lib/api';
import { rememberReturnTo } from '../../lib/returnTo';

export function LoginForm() {
  const params = useSearchParams();
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<'idle' | 'sending' | 'sent' | 'yandex'>('idle');
  const [error, setError] = useState('');

  useEffect(() => {
    rememberReturnTo(params.get('next'));
  }, [params]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (status === 'sending' || status === 'yandex') return;
    setStatus('sending');
    setError('');
    try {
      await startEmailLogin(email);
      setStatus('sent');
    } catch (err) {
      setStatus('idle');
      setError(err instanceof Error ? err.message : 'Не удалось отправить ссылку');
    }
  }

  async function loginWithYandex() {
    if (status === 'sending' || status === 'yandex') return;
    setStatus('yandex');
    setError('');
    try {
      const authorizationURL = await startYandexLogin();
      window.location.assign(authorizationURL);
    } catch (err) {
      setStatus('idle');
      setError(err instanceof Error ? err.message : 'Вход через Яндекс временно недоступен');
    }
  }

  if (status === 'sent') {
    return (
      <Surface className="authCard">
        <div className="authMark">✓</div>
        <span className="eyebrow">CHAT</span>
        <h1>Проверьте почту</h1>
        <p>Мы отправили одноразовую ссылку для входа на <strong>{email}</strong>.</p>
        <Button variant="secondary" fullWidth onClick={() => setStatus('idle')}>Изменить email</Button>
      </Surface>
    );
  }

  return (
    <Surface className="authCard">
      <span className="eyebrow">CHAT</span>
      <h1>Войти в CHAT</h1>
      <p>Без пароля и SMS. Укажите email — отправим защищённую одноразовую ссылку.</p>
      <form className="authForm" onSubmit={submit}>
        <label>
          <span>Email</span>
          <input
            type="email"
            autoComplete="email"
            inputMode="email"
            required
            maxLength={254}
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="name@example.com"
          />
        </label>
        {error ? <div className="authError" role="alert">{error}</div> : null}
        <Button type="submit" fullWidth disabled={status === 'sending' || status === 'yandex'}>
          {status === 'sending' ? 'Отправляем…' : 'Продолжить'}
        </Button>
      </form>
      <div className="authDivider"><span>или</span></div>
      <Button variant="secondary" fullWidth disabled={status === 'sending' || status === 'yandex'} onClick={loginWithYandex}>
        {status === 'yandex' ? 'Открываем Яндекс…' : 'Войти через Яндекс ID'}
      </Button>
      <small>Продолжая, вы принимаете правила сервиса и политику конфиденциальности.</small>
    </Surface>
  );
}
