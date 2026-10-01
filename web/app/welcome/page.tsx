import type { Metadata } from 'next';
import './welcome.css';

export const metadata: Metadata = {
  title: 'CHAT — Найди своих',
  description: 'Быстрый социальный мессенджер для общения, новых людей, публикаций и сообществ.',
  openGraph: {
    title: 'CHAT — Найди своих',
    description: 'Общайся без лишнего. Находи людей по интересам, публикуй мысли и фото, оставайся на связи даже при слабом интернете.',
    type: 'website'
  }
};

const features = [
  ['Быстрые чаты', 'Личные сообщения и группы остаются главным экраном. Никакой перегруженной ленты вместо общения.'],
  ['Найди своих', 'Люди и сообщества подбираются по интересам и общему контексту, а не случайной рулеткой.'],
  ['Шанс быть замеченным', 'Новая публикация получает тестовый органический охват даже у автора без большой аудитории.'],
  ['Связь не идеальна? Не страшно', 'Исходящие сообщения сохраняются локально и автоматически доставляются после восстановления сети.']
];

export default function WelcomePage() {
  return (
    <main className="welcomePage">
      <header className="welcomeHeader">
        <a className="welcomeBrand" href="/welcome" aria-label="CHAT">CHAT</a>
        <nav aria-label="Вход">
          <a className="welcomeLink" href="/login">Войти</a>
          <a className="welcomeButton welcomeButtonSmall" href="/login?mode=signup">Создать CHAT</a>
        </nav>
      </header>

      <section className="welcomeHero">
        <div className="welcomeHeroCopy">
          <span className="welcomeEyebrow">СОЦИАЛЬНЫЙ МЕССЕНДЖЕР</span>
          <h1>Найди своих.</h1>
          <p>Общайся с друзьями, находи людей по интересам, публикуй мысли и фотографии и развивай свою аудиторию — без перегруженного интерфейса.</p>
          <div className="welcomeActions">
            <a className="welcomeButton" href="/login?mode=signup">Создать CHAT</a>
            <a className="welcomeSecondary" href="/login">У меня уже есть аккаунт</a>
          </div>
          <div className="welcomeProof" aria-label="Принципы CHAT">
            <span>Минимализм</span><span>Скорость</span><span>Offline-first</span><span>Без рекламного мусора в чатах</span>
          </div>
        </div>
        <div className="welcomeDemo" aria-label="Пример интерфейса CHAT">
          <div className="welcomeDemoTop"><strong>Чаты</strong><span>✦</span></div>
          <div className="welcomeChat"><i>М</i><div><strong>Маша</strong><span>Встретимся вечером?</span></div><b>2</b></div>
          <div className="welcomeChat"><i>И</i><div><strong>Игровая группа</strong><span>Кто сегодня в онлайне?</span></div></div>
          <div className="welcomeChat"><i>А</i><div><strong>Алексей</strong><span>Отправлено ✓</span></div></div>
          <div className="welcomeOffline">Связь пропала · сообщение сохранено на устройстве</div>
        </div>
      </section>

      <section className="welcomeFeatures" aria-label="Возможности">
        {features.map(([title, text]) => (
          <article key={title}><h2>{title}</h2><p>{text}</p></article>
        ))}
      </section>

      <section className="welcomeFinal">
        <span>CHAT</span>
        <h2>Не ещё одна лента. Место, где появляются связи между людьми.</h2>
        <a className="welcomeButton" href="/login?mode=signup">Начать</a>
      </section>

      <footer className="welcomeFooter">
        <span>© CHAT</span>
        <div><a href="/login">Войти</a><a href="/welcome">О продукте</a></div>
      </footer>
    </main>
  );
}
