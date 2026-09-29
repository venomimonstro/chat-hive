import { AppShell } from '../components/AppShell';
import { Avatar, Badge, IconButton, SearchField } from '../components/ui';

const chats = [
  { name: 'Маша', message: 'Встретимся вечером?', time: '12:48', unread: 2 },
  { name: 'Игровая группа', message: 'Сегодня собираемся в 21:00', time: '11:35', unread: 7 },
  { name: 'Алексей', message: 'Фото отправил', time: 'Вчера', unread: 0 }
];

export default function HomePage() {
  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div>
          <span className="eyebrow">CHAT</span>
          <h1>Чаты</h1>
        </div>
        <IconButton type="button" aria-label="Открыть расширенный поиск">⌕</IconButton>
      </header>

      <div className="screenSearch">
        <SearchField aria-label="Поиск людей и чатов" placeholder="Поиск людей и чатов" />
      </div>

      <section className="chatList" aria-label="Список чатов">
        {chats.map((chat) => (
          <article className="chatRow" key={chat.name}>
            <Avatar name={chat.name} />
            <div className="chatCopy">
              <div className="chatHeadline">
                <strong>{chat.name}</strong>
                <time>{chat.time}</time>
              </div>
              <div className="chatPreview">
                <span>{chat.message}</span>
                {chat.unread > 0 ? <Badge>{chat.unread}</Badge> : null}
              </div>
            </div>
          </article>
        ))}
      </section>
    </AppShell>
  );
}
