import type { Metadata } from 'next';
import { WelcomeClient } from './WelcomeClient';
import './welcome.css';

export const metadata: Metadata = {
 title: 'CHAT — Личные сообщения',
 description: 'Простой мессенджер для личной переписки. Регистрация по почте, настройки аккаунта и быстрые сообщения.',
 openGraph: { title:'CHAT — Личные сообщения',description:'Простой мессенджер для личной переписки.',type:'website' }
};

export default function WelcomePage() { return <WelcomeClient />; }
