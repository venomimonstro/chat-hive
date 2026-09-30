'use client';

import { useEffect, useState } from 'react';
import { fetchMediaBlob } from '../lib/media';

export function AuthenticatedImage({ src, alt, className }: { src: string; alt: string; className?: string }) {
  const [url, setURL] = useState('');
  const [failed, setFailed] = useState(false);

  useEffect(() => {
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
  }, [src]);

  if (failed) return <div className={className} role="img" aria-label={alt}>Изображение недоступно</div>;
  if (!url) return <div className={className} aria-hidden="true" />;
  return <img className={className} src={url} alt={alt} loading="lazy" decoding="async" />;
}
