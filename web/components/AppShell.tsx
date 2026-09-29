import type { ReactNode } from 'react';

export function AppShell({ children, active = 'Чаты', wide = false }: { children: ReactNode; active?: 'Чаты' | 'Открыть' | 'Я'; wide?: boolean }) {
  return (
    <main className={`appShell ${wide ? 'appShell--wide' : ''}`}>
      <section className={`appDevice ${wide ? 'appDevice--wide' : ''}`} aria-label="CHAT">
        <div className="appContent">{children}</div>
        <nav className="appNav" aria-label="Основная навигация">
          <a className={`appNav__item ${active === 'Чаты' ? 'isActive' : ''}`} href="/">
            <span aria-hidden="true">●</span><b>Чаты</b>
          </a>
          <a className={`appNav__item ${active === 'Открыть' ? 'isActive' : ''}`} href="/discover">
            <span aria-hidden="true">✦</span><b>Открыть</b>
          </a>
          <button className="appNav__create" type="button" aria-label="Создать публикацию, группу или канал">+</button>
          <a className={`appNav__item ${active === 'Я' ? 'isActive' : ''}`} href="/me">
            <span aria-hidden="true">◉</span><b>Я</b>
          </a>
        </nav>
      </section>

      {!wide ? (
        <aside className="appStory" aria-label="Принципы CHAT">
          <span className="eyebrow">CHAT</span>
          <h2>Найди своих.</h2>
          <p>Мессенджер остаётся главным экраном. Discovery и создание контента расширяют общение, но не превращают CHAT в перегруженный портал.</p>
          <div className="principles"><span>Быстро</span><span>Просто</span><span>Без лишнего</span></div>
        </aside>
      ) : null}
    </main>
  );
}
