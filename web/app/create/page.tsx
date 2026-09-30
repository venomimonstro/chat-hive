import { AppShell } from '../../components/AppShell';

export default function CreatePage() {
  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">CHAT</span><h1>Создать</h1></div>
      </header>
      <section className="createHub">
        <a href="/groups/new" className="createHubCard">
          <span className="createHubIcon">◎</span>
          <div><strong>Группа</strong><p>Общий чат с участниками, ролями и приглашениями.</p></div>
        </a>
        <div className="createHubCard isDisabled" aria-disabled="true">
          <span className="createHubIcon">✦</span>
          <div><strong>Мысль</strong><p>Короткая публичная публикация. Появится в Sprint 11.</p></div>
        </div>
        <div className="createHubCard isDisabled" aria-disabled="true">
          <span className="createHubIcon">▣</span>
          <div><strong>Фото / пост</strong><p>Публикации с изображениями будут подключены после media pipeline.</p></div>
        </div>
      </section>
    </AppShell>
  );
}
