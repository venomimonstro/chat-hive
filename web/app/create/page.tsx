import { AppShell } from '../../components/AppShell';

export default function CreatePage() {
  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">CHAT</span><h1>Создать</h1></div>
      </header>
      <section className="createHub">
        <a href="/create/post?kind=thought" className="createHubCard">
          <span className="createHubIcon">✦</span>
          <div><strong>Мысль</strong><p>Короткая публикация до 700 символов для живого разговора.</p></div>
        </a>
        <a href="/create/post?kind=photo" className="createHubCard">
          <span className="createHubIcon">▣</span>
          <div><strong>Фото</strong><p>До 10 изображений с безопасной серверной обработкой и подписью.</p></div>
        </a>
        <a href="/create/post?kind=post" className="createHubCard">
          <span className="createHubIcon">≡</span>
          <div><strong>Пост</strong><p>Развёрнутый текст для профиля и ленты Discover.</p></div>
        </a>
        <a href="/groups/new" className="createHubCard">
          <span className="createHubIcon">◎</span>
          <div><strong>Группа</strong><p>Общий чат с участниками, ролями и приглашениями.</p></div>
        </a>
      </section>
    </AppShell>
  );
}
