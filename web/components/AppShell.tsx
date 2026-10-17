'use client';

import type { ReactNode } from 'react';
import { RealtimeBridge } from './RealtimeBridge';
import { useLanguage } from '../lib/language';

export function AppShell({ children, active = 'Чаты', wide = false }: {
  children: ReactNode; active?: 'Чаты' | 'Открыть' | 'Я'; wide?: boolean
}) {
  const { t } = useLanguage();
  return (
    <main className={`appShell coreShell ${wide ? 'appShell--wide' : ''}`}>
      <RealtimeBridge />
      <section className={`appDevice ${wide ? 'appDevice--wide' : ''}`} aria-label="CHAT">
        <div className="coreTopbar">
          <a className="coreBrand" href="/" aria-label="CHAT — chats">
            <span className="coreBrandIcon">C</span><strong>CHAT</strong>
            <span className="coreBrandBeta">beta</span>
          </a>
          <div className="coreTopActions">
            <a href="/requests">{t.messageRequests}</a>
            <a href="/settings/language" aria-label={t.language}>RU / EN</a>
          </div>
        </div>
        <div className="appContent">{children}</div>
        <nav className="appNav" aria-label={t.main}>
          <a className={`appNav__item ${active === 'Чаты' ? 'isActive' : ''}`} href="/">
            <span aria-hidden="true">▤</span><b>{t.chats}</b>
          </a>
          <a className={`appNav__item ${active === 'Я' ? 'isActive' : ''}`} href="/me">
            <span aria-hidden="true">◉</span><b>{t.account}</b>
          </a>
        </nav>
      </section>
    </main>
  );
}
