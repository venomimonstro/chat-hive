'use client';

import { useEffect, useState } from 'react';
import { AppShell } from '../../../components/AppShell';
import { DataSaverPreference, getDataSaverPreference, getEffectiveDataMode, setDataSaverPreference } from '../../../lib/dataSaver';

const options: Array<{ value: DataSaverPreference; title: string; text: string }> = [
  { value: 'auto', title: 'Автоматически', text: 'CHAT включает экономию на медленной сети или когда браузер просит экономить трафик.' },
  { value: 'save', title: 'Экономия', text: 'Медиа загружаются только по необходимости и без агрессивной предзагрузки.' },
  { value: 'maximum', title: 'Максимальная экономия', text: 'Изображения не скачиваются, пока вы явно не нажмёте на них.' }
];

export default function TrafficSettingsPage() {
  const [preference, setPreference] = useState<DataSaverPreference>('auto');
  const [effective, setEffective] = useState('normal');

  useEffect(() => {
    setPreference(getDataSaverPreference());
    setEffective(getEffectiveDataMode());
  }, []);

  function choose(value: DataSaverPreference) {
    setDataSaverPreference(value);
    setPreference(value);
    setEffective(getEffectiveDataMode());
  }

  return (
    <AppShell active="Я">
      <header className="screenHeader">
        <div><span className="eyebrow">НАСТРОЙКИ</span><h1>Трафик</h1></div>
        <a href="/me" aria-label="Назад">←</a>
      </header>
      <main className="trafficSettings">
        <div className="trafficStatus"><strong>Текущий режим</strong><span>{effective === 'maximum' ? 'Максимальная экономия' : effective === 'save' ? 'Экономия' : 'Обычный'}</span></div>
        <section className="trafficOptions" aria-label="Режим экономии трафика">
          {options.map((option) => (
            <button type="button" className={preference === option.value ? 'isSelected' : ''} key={option.value} onClick={() => choose(option.value)}>
              <span className="trafficRadio" aria-hidden="true">{preference === option.value ? '●' : '○'}</span>
              <span><strong>{option.title}</strong><small>{option.text}</small></span>
            </button>
          ))}
        </section>
        <p className="trafficHint">Текстовые сообщения и очередь отправки продолжают работать независимо от режима. Максимальная экономия влияет прежде всего на изображения и фоновые загрузки.</p>
      </main>
    </AppShell>
  );
}
