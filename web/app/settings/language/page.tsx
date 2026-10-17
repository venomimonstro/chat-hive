'use client';

import { useState } from 'react';
import { AppShell } from '../../../components/AppShell';
import { Button } from '../../../components/ui';
import { useLanguage, type Language } from '../../../lib/language';

export default function LanguageSettingsPage() {
  const { language, t, changeLanguage } = useLanguage();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  async function change(next: Language) {
    if (saving || next === language) return;
    setSaving(true); setSaved(false); setError('');
    try { await changeLanguage(next); setSaved(true); }
    catch { setError(t.accountError); }
    finally { setSaving(false); }
  }
  return (
    <AppShell active="Я">
      <section className="coreSettings">
        <a href="/me" className="coreBack">← {t.account}</a>
        <h1>{t.language}</h1>
        <p>{t.languageHint}</p>
        <div className="coreLanguageOptions" role="group" aria-label={t.language}>
          <button type="button" aria-pressed={language === 'ru'} disabled={saving}
            className={language === 'ru' ? 'isSelected' : ''} onClick={() => void change('ru')}>Русский {language === 'ru' ? '✓' : ''}</button>
          <button type="button" aria-pressed={language === 'en'} disabled={saving}
            className={language === 'en' ? 'isSelected' : ''} onClick={() => void change('en')}>English {language === 'en' ? '✓' : ''}</button>
        </div>
        {error ? <p className="authError" role="alert">{error}</p> : null}
        {saved ? <p className="coreSaved" role="status">{t.saved}</p> : null}
        <a href="/" className="coreTextLink">{t.goChats} →</a>
      </section>
    </AppShell>
  );
}
