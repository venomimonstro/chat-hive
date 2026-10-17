'use client';

import { useEffect, useState } from 'react';
import { getCurrentSession, getLanguagePreference, saveLanguagePreference, type Language } from './api';

const KEY = 'chat-interface-language';
const EVENT = 'chat:language';
export type { Language };

export const labels = {
  ru: {
    chats:'Чаты',account:'Аккаунт', settings:'Настройки',newChat:'Новое сообщение',
    searchChats:'Поиск чатов', loadingChats:'Загружаем чаты…',
    emptyChats:'Пока нет переписок. Нажмите + и укажите имя пользователя.',
    username:'Имя пользователя', open:'Открыть', chooseChat:'Выберите переписку',
    chooseChatHint:'Или начните новую по имени пользователя.', loadingHistory:'Загружаем сообщения…',
    older:'Ранние сообщения', loading:'Загрузка…', removed:'Сообщение удалено',
    edited:'изменено', sending:'отправляем…', retry:'повторить', reply:'Ответить',
    edit:'Изменить', delete:'Удалить', editing:'Редактирование', answer:'Ответ',
    cancel:'Отменить', message:'Сообщение', save:'Сохранить', send:'Отправить',
    back:'Назад к чатам', create:'Начать переписку', email:'Электронная почта',
    login:'Войти в CHAT', loginIntro:'Введите почту — отправим одноразовую ссылку для безопасного входа.',
    continue:'Продолжить', sendingLink:'Отправляем…', checkMail:'Проверьте почту',
    sentLink:'Мы отправили одноразовую ссылку на', changeEmail:'Изменить почту',
    or:'или', passkey:'Войти с ключом доступа', yandex:'Войти через Яндекс ID',
    verifying:'Проверяем…', openingYandex:'Открываем Яндекс…',
    terms:'Продолжая, вы принимаете правила и политику конфиденциальности.',
    firstStart:'НАСТРОЙКА ПРОФИЛЯ', setupProfile:'Как вас называть?',
    setupHint:'Укажите имя и короткий адрес. Этого достаточно, чтобы начать общение.',
    displayName:'Имя', about:'О себе (необязательно)', startUsing:'Перейти к чатам',
    preparing:'Подготавливаем профиль…', retryLoad:'Повторить загрузку',
    saveChanges:'Сохранить изменения', profile:'Профиль', editProfile:'Изменить профиль',
    security:'Безопасность и устройства', language:'Язык интерфейса',
    languageHint:'Язык сохраняется в аккаунте и применяется на других устройствах.',
    russian:'Русский', english:'English', saved:'Настройки сохранены',
    saving:'Сохраняем…', logOut:'Выйти', data:'Данные аккаунта',
    messageRequests:'Запросы на переписку', privacy:'Приватность', main:'Главная',
    simpleMessaging:'Простое и защищённое общение.', later:'Остальные возможности появятся позже.',
    accountError:'Не удалось загрузить настройки.', retryAction:'Повторить',
    noItems:'Пока пусто', preferences:'Настройки языка', theme:'Оформление',
    requiredName:'Укажите имя и адрес профиля (от 3 до 32 символов).',
    goChats:'Вернуться к чатам'
  },
  en: {
    chats:'Chats',account:'Account',settings:'Settings',newChat:'New message',
    searchChats:'Search chats', loadingChats:'Loading chats…',
    emptyChats:'No conversations yet. Tap + and enter a username.',
    username:'Username', open:'Open', chooseChat:'Select a conversation',
    chooseChatHint:'Or start a new one with a username.', loadingHistory:'Loading messages…',
    older:'Earlier messages', loading:'Loading…', removed:'Message deleted',
    edited:'edited', sending:'sending…', retry:'retry', reply:'Reply',
    edit:'Edit', delete:'Delete', editing:'Editing', answer:'Replying to',
    cancel:'Cancel', message:'Message', save:'Save', send:'Send',
    back:'Back to chats', create:'Start a chat', email:'Email',
    login:'Sign in to CHAT', loginIntro:'Enter your email and we’ll send a secure one-time sign-in link.',
    continue:'Continue', sendingLink:'Sending…', checkMail:'Check your email',
    sentLink:'We sent a one-time sign-in link to', changeEmail:'Change email',
    or:'or', passkey:'Sign in with passkey', yandex:'Sign in with Yandex ID',
    verifying:'Verifying…', openingYandex:'Opening Yandex…',
    terms:'By continuing, you accept the terms and privacy policy.',
    firstStart:'YOUR PROFILE', setupProfile:'What should we call you?',
    setupHint:'Add your name and username. That’s all you need to start chatting.',
    displayName:'Display name', about:'About you (optional)', startUsing:'Go to chats',
    preparing:'Preparing your profile…', retryLoad:'Retry loading',
    saveChanges:'Save changes', profile:'Profile', editProfile:'Edit profile',
    security:'Security and devices', language:'Interface language',
    languageHint:'Your language is saved to your account and used on other devices.',
    russian:'Русский', english:'English', saved:'Settings saved',
    saving:'Saving…', logOut:'Log out', data:'Account data',
    messageRequests:'Message requests', privacy:'Privacy', main:'Home',
    simpleMessaging:'Simple, secure conversations.', later:'More features are coming later.',
    accountError:'Could not load settings.', retryAction:'Retry',
    noItems:'Nothing here yet', preferences:'Language settings', theme:'Appearance',
    requiredName:'Enter your name and a username (3–32 characters).',
    goChats:'Back to chats'
  }
} as const;

export function readLanguage(): Language {
  if (typeof window === 'undefined') return 'ru';
  try { return window.localStorage.getItem(KEY) === 'en' ? 'en' : 'ru'; }
  catch { return 'ru'; }
}
function applyLanguage(lang: Language) {
  if (typeof window === 'undefined') return;
  try { window.localStorage.setItem(KEY, lang); } catch {}
  document.documentElement.lang = lang;
  window.dispatchEvent(new CustomEvent<Language>(EVENT, { detail: lang }));
}

export function useLanguage() {
  const [language, setLanguage] = useState<Language>('ru');
  useEffect(() => {
    let active = true;
    let revision = 0;
    const sync = () => { if (active) setLanguage(readLanguage()); };
    const load = async () => {
      const thisRevision = ++revision;
      sync();
      try {
        // Short-lived bearer credentials intentionally live only in memory.
        // Restore them through the HttpOnly cookie before reading preferences.
        const session = await getCurrentSession();
        if (!active || thisRevision !== revision || !session) return;
        const fromAccount = await getLanguagePreference();
        if (active && thisRevision === revision) {
          applyLanguage(fromAccount);
          setLanguage(fromAccount);
        }
      } catch { /* Offline or logged out: retain the local language. */ }
    };
    void load();
    window.addEventListener(EVENT, sync);
    window.addEventListener('storage', sync);
    window.addEventListener('chat:auth-changed', load);
    return () => {
      active = false;
      revision++;
      window.removeEventListener(EVENT, sync);
      window.removeEventListener('storage', sync);
      window.removeEventListener('chat:auth-changed', load);
    };
  }, []);

  async function changeLanguage(next: Language) {
    const session = await getCurrentSession();
    if (session) {
      // Do not claim the account changed if the API write failed.
      await saveLanguagePreference(next);
    }
    applyLanguage(next);
    setLanguage(next);
  }
  return { language, t: labels[language], changeLanguage };
}
