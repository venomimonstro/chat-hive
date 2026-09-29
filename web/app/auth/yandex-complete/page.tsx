import { Suspense } from 'react';
import { YandexComplete } from './YandexComplete';

export default function YandexCompletePage() {
  return (
    <main className="authShell authShell--centered">
      <Suspense fallback={<div className="authCallbackState"><h1>CHAT</h1><p>Завершаем вход…</p></div>}>
        <YandexComplete />
      </Suspense>
    </main>
  );
}
