'use client';

import { FormEvent, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { AppShell } from '../../components/AppShell';
import { Button, Surface } from '../../components/ui';
import { ReportCategory, ReportTargetType, submitReport } from '../../lib/reports';

const categories: { value: ReportCategory; label: string }[] = [
  { value: 'spam', label: 'Спам' },
  { value: 'scam', label: 'Мошенничество' },
  { value: 'harassment', label: 'Преследование / оскорбления' },
  { value: 'threats', label: 'Угрозы' },
  { value: 'illegal_content', label: 'Противоправный контент' },
  { value: 'sexual_content', label: 'Недопустимый сексуальный контент' },
  { value: 'impersonation', label: 'Выдаёт себя за другого' },
  { value: 'malware', label: 'Вредоносная ссылка / файл' },
  { value: 'other', label: 'Другое' }
];

function validTarget(value: string | null): value is ReportTargetType {
  return value === 'user' || value === 'message' || value === 'post' || value === 'group';
}

export default function ReportPage() {
  const router = useRouter();
  const search = useSearchParams();
  const rawType = search.get('type');
  const targetType = validTarget(rawType) ? rawType : null;
  const targetId = search.get('id') ?? '';
  const [category, setCategory] = useState<ReportCategory>('spam');
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState('');
  const valid = useMemo(() => Boolean(targetType && /^[0-9a-f-]{36}$/i.test(targetId)), [targetType, targetId]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!valid || !targetType || busy) return;
    setBusy(true);
    setError('');
    try {
      await submitReport({ target_type: targetType, target_id: targetId, category, note: note.trim() });
      setSent(true);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') {
        router.replace('/login');
        return;
      }
      setError(err instanceof Error ? err.message : 'Не удалось отправить жалобу');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell active="Я">
      <header className="screenHeader">
        <div><span className="eyebrow">TRUST & SAFETY</span><h1>Пожаловаться</h1></div>
        <button className="plainIconButton" type="button" onClick={() => router.back()} aria-label="Назад">←</button>
      </header>
      <main className="reportPage">
        {!valid ? (
          <Surface className="reportCard"><h2>Объект жалобы недоступен</h2><p>Откройте жалобу из профиля, публикации, группы или сообщения.</p></Surface>
        ) : sent ? (
          <Surface className="reportCard"><h2>Жалоба отправлена</h2><p>Мы объединим её с другими сигналами и рассмотрим в moderation workflow. Жалоба сама по себе не приводит к автоматической блокировке.</p><Button onClick={() => router.back()}>Вернуться</Button></Surface>
        ) : (
          <Surface className="reportCard">
            <form className="reportForm" onSubmit={submit}>
              <label><span>Причина</span><select value={category} onChange={(e) => setCategory(e.target.value as ReportCategory)}>{categories.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
              <label><span>Комментарий <small>{note.length}/1000</small></span><textarea value={note} onChange={(e) => setNote(e.target.value)} maxLength={1000} rows={5} placeholder="Коротко опишите, что произошло. Не добавляйте лишние персональные данные." /></label>
              {error ? <p className="messengerError" role="alert">{error}</p> : null}
              <Button type="submit" fullWidth disabled={busy}>{busy ? 'Отправляем…' : 'Отправить жалобу'}</Button>
            </form>
          </Surface>
        )}
      </main>
    </AppShell>
  );
}
