import type { Metadata, Viewport } from 'next';
import './globals.css';
import './auth.css';
import './onboarding.css';
import './profile.css';
import './security.css';
import './messenger.css';
import './create.css';
import './discover.css';

export const metadata: Metadata = {
  title: 'CHAT — Найди своих',
  description: 'Быстрый социальный мессенджер для общения и поиска людей по интересам.'
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#ffffff' },
    { media: '(prefers-color-scheme: dark)', color: '#0b0d10' }
  ]
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
