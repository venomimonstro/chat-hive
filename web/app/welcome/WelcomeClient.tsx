'use client';
import { useLanguage } from '../../lib/language';

export function WelcomeClient() {
 const { language } = useLanguage();
 const en=language==='en';
 return <main className="welcomePage coreWelcome">
   <header className="welcomeHeader">
     <a href="/welcome" className="welcomeBrand" aria-label="CHAT">CHAT</a>
     <nav><a className="welcomeLink" href="/login">{en?'Sign in':'Войти'}</a>
     <a className="welcomeButton welcomeButtonSmall" href="/login?mode=signup">{en?'Create account':'Создать аккаунт'}</a></nav>
   </header>
   <section className="welcomeHero">
     <div className="welcomeHeroCopy">
       <span className="welcomeEyebrow">{en?'MESSAGING, WITHOUT DISTRACTIONS':'ОБЩЕНИЕ БЕЗ ЛИШНЕГО'}</span>
       <h1>{en?'Stay in touch.':'Будьте на связи.'}</h1>
       <p>{en
         ? 'Personal chats, a simple interface and account security. CHAT begins with the essentials; more features will follow after testing.'
         : 'Личные сообщения, простой интерфейс и защита аккаунта. Сначала самое важное — остальные возможности появятся после испытаний.'}</p>
       <div className="welcomeActions">
         <a className="welcomeButton" href="/login?mode=signup">{en?'Get started':'Начать общаться'}</a>
         <a className="welcomeSecondary" href="/login">{en?'I have an account':'У меня уже есть аккаунт'}</a>
       </div>
       <div className="welcomeProof">
         <span>{en?'Messages':'Сообщения'}</span><span>{en?'Account settings':'Настройки'}</span>
         <span>{en?'Offline message queue':'Очередь без сети'}</span>
       </div>
     </div>
     <div className="welcomeDemo" aria-label={en?'Chat preview':'Пример переписки'}>
       <div className="welcomeDemoTop"><strong>{en?'Messages':'Сообщения'}</strong><span>CHAT</span></div>
       <div className="welcomeChat"><i>А</i><div><strong>{en?'Alex':'Алексей'}</strong><span>{en?'See you later?':'Увидимся вечером?'}</span></div></div>
       <div className="welcomeChat"><i>М</i><div><strong>{en?'Maria':'Мария'}</strong><span>{en?'Sounds good!':'Отлично, договорились!'}</span></div></div>
       <div className="welcomeOffline">{en?'A message is kept on your device until it can be sent.':'Сообщение сохраняется на устройстве до отправки.'}</div>
     </div>
   </section>
   <footer className="welcomeFooter">
     <span>© CHAT</span>
     <a href="/login">{en?'Sign in':'Войти'}</a>
   </footer>
 </main>;
}
