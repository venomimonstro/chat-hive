'use client';

import { FormEvent, useState } from 'react';
import { AppShell } from '../../../components/AppShell';
import { Button, Surface } from '../../../components/ui';
import { createGroup } from '../../../lib/api';

export default function NewGroupPage() {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError('');
    try {
      const group = await createGroup({ title, description });
      window.location.replace(`/groups/${group.chat_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать группу');
      setSaving(false);
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">CHAT / ГРУППА</span><h1>Новая группа</h1></div>
      </header>
      <div className="createPageBody">
        <Surface className="createFormCard">
          <form onSubmit={submit} className="createForm">
            <label>
              <span>Название</span>
              <input autoFocus required minLength={2} maxLength={80} value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Например, Наш университет" />
            </label>
            <label>
              <span>Описание</span>
              <textarea rows={4} maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="О чём эта группа" />
            </label>
            {error ? <div className="authError" role="alert">{error}</div> : null}
            <Button type="submit" fullWidth disabled={saving || title.trim().length < 2}>{saving ? 'Создаём…' : 'Создать группу'}</Button>
          </form>
        </Surface>
      </div>
    </AppShell>
  );
}
