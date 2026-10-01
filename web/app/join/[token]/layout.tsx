import type { Metadata } from 'next';
import type { ReactNode } from 'react';

export const metadata: Metadata = {
  title: 'Приглашение в CHAT',
  description: 'Безопасная ссылка приглашения в группу CHAT.',
  robots: {
    index: false,
    follow: false,
    nocache: true,
    googleBot: { index: false, follow: false, noimageindex: true }
  },
  referrer: 'no-referrer'
};

export default function InviteLayout({ children }: { children: ReactNode }) {
  return children;
}
