import { LoginForm } from './LoginForm';

export default function LoginPage() {
  return (
    <main className="authShell">
      <section className="authIntro" aria-hidden="true">
        <span className="eyebrow">CHAT</span>
        <h2>Найди своих.</h2>
        <p>Быстрый социальный мессенджер без перегруженного интерфейса.</p>
      </section>
      <LoginForm />
    </main>
  );
}
