const chats = [
  { name: 'Маша', message: 'Встретимся вечером?', time: '12:48', unread: 2 },
  { name: 'Игровая группа', message: 'Сегодня собираемся в 21:00', time: '11:35', unread: 7 },
  { name: 'Алексей', message: 'Фото отправил', time: 'Вчера', unread: 0 }
];

export default function HomePage() {
  return (
    <main className="appShell">
      <section className="phoneFrame" aria-label="CHAT preview">
        <header className="topBar">
          <div>
            <span className="eyebrow">CHAT</span>
            <h1>Чаты</h1>
          </div>
          <button className="iconButton" type="button" aria-label="Поиск">⌕</button>
        </header>

        <div className="searchBox" role="search">Поиск людей и чатов</div>

        <section className="chatList" aria-label="Список чатов">
          {chats.map((chat) => (
            <article className="chatRow" key={chat.name}>
              <div className="avatar" aria-hidden="true">{chat.name.slice(0, 1)}</div>
              <div className="chatCopy">
                <div className="chatHeadline">
                  <strong>{chat.name}</strong>
                  <time>{chat.time}</time>
                </div>
                <div className="chatPreview">
                  <span>{chat.message}</span>
                  {chat.unread > 0 ? <span className="badge">{chat.unread}</span> : null}
                </div>
              </div>
            </article>
          ))}
        </section>

        <nav className="bottomNav" aria-label="Основная навигация">
          <a className="navItem active" href="#chats"><span>●</span><b>Чаты</b></a>
          <a className="navItem" href="#discover"><span>✦</span><b>Открыть</b></a>
          <button className="createButton" type="button" aria-label="Создать">+</button>
          <a className="navItem" href="#me"><span>◉</span><b>Я</b></a>
        </nav>
      </section>

      <section className="desktopIntro">
        <span className="eyebrow">CHAT / FOUNDATION</span>
        <h2>Найди своих.</h2>
        <p>Первый production-каркас: mobile-first интерфейс, четыре постоянные зоны и архитектура без перегруженного портала.</p>
        <div className="principles">
          <span>Быстро</span><span>Просто</span><span>Без лишнего</span>
        </div>
      </section>
    </main>
  );
}
