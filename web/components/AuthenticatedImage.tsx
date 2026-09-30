'use client';

import { useEffect, useState } from 'react';
import { fetchMediaBlob } from '../lib/media';
import { EffectiveDataMode, getEffectiveDataMode, subscribeDataMode } from '../lib/dataSaver';

export function AuthenticatedImage({ src, alt, className }: { src: string; alt: string; className?: string }) {
  const [url, setURL] = useState('');
  const [failed, setFailed] = useState(false);
  const [mode, setMode] = useState<EffectiveDataMode>('normal');
  const [manualLoad, setManualLoad] = useState(false);

  useEffect(() => {
    setMode(getEffectiveDataMode());
    return subscribeDataMode((next) => setMode(next));
  }, []);

  useEffect(() => {
    if (mode === 'maximum' && !manualLoad) {
      setURL('');
      setFailed(false);
      return;
    }
    let active = true;
    let objectURL = '';
    setFailed(false);
    fetchMediaBlob(src)
      .then((blob) => {
        if (!active) return;
        objectURL = URL.createObjectURL(blob);
        setURL(objectURL);
      })
      .catch(() => active && setFailed(true));
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [src, mode, manualLoad]);

  if (mode === 'maximum' && !manualLoad) {
    return <button type="button" className={`${className ?? ''} dataSaverPlaceholder`} onClick={() => setManualLoad(true)} aria-label={`Загрузить изображение: ${alt}`}><span>Изображение не загружено</span><small>Нажмите, чтобы открыть</small></button>;
  }
  if (failed) return <div className={className} role="img" aria-label={alt}>Изображение недоступно</div>;
  if (!url) return <div className={className} aria-hidden="true" />;
  return <img className={className} src={url} alt={alt} loading={mode === 'save' ? 'lazy' : 'eager'} decoding="async" />;
}
