'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { getProfile } from '../../lib/api';

export default function MePage() {
  const router = useRouter();
  const [error, setError] = useState('');

  useEffect(() => {
    void getProfile()
      .then((profile) => {
        if (!profile) {
          router.replace('/login');
          return;
        }
        if (!profile.completed) {
          router.replace('/onboarding');
          return;
        }
        router.replace(`/u/${profile.username}`);
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Не удалось открыть профиль'));
  }, [router]);

  return (
    <AppShell active="Я">
      <div className="profileState">{error || 'Открываем ваш профиль…'}</div>
    </AppShell>
  );
}
