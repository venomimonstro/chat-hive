import { Suspense } from 'react';
import { AuthCallback } from './AuthCallback';

export default function AuthCallbackPage() {
  return (
    <main className="authShell authShell--centered">
      <Suspense fallback={<div className="authCallbackState"><h1>CHAT</h1><p>Подготавливаем вход…</p></div>}>
        <AuthCallback />
      </Suspense>
    </main>
  );
}
