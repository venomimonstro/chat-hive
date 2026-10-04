'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Surface } from '../../../components/ui';
import { BetaReadiness, getAdminPrincipal, getBetaReadiness, getProductMetrics, ProductMetrics } from '../../../lib/admin';

const WINDOWS = [7, 30, 90] as const;

function percent(part: number, total: number) {
  if (!total) return '—';
  return `${Math.round((part / total) * 1000) / 10}%`;
}

export default function ProductAdminPage() {
  const router = useRouter();
  const [roles, setRoles] = useState<string[]>([]);
  const [days, setDays] = useState<7 | 30 | 90>(7);
  const [metrics, setMetrics] = useState<ProductMetrics | null>(null);
  const [readiness, setReadiness] = useState<BetaReadiness | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load(windowDays: 7 | 30 | 90) {
    setLoading(true);
    setError('');
    try {
      const principal = await getAdminPrincipal();
      if (!principal) { router.replace('/'); return; }
      if (!principal.roles.includes('owner')) { router.replace('/admin/moderation'); return; }
      setRoles(principal.roles);
      const [productMetrics, betaReadiness] = await Promise.all([
        getProductMetrics(windowDays),
        getBetaReadiness()
      ]);
      setMetrics(productMetrics);
      setReadiness(betaReadiness);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Не удалось загрузить продуктовые метрики');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(days); }, [days]);

  const cards = useMemo(() => {
    if (!metrics) return [];
    return [
      ['Регистрации', metrics.registrations, 'новые аккаунты'],
      ['Onboarding', metrics.onboarding_completed, percent(metrics.onboarding_completed, metrics.registrations)],
      ['Первое сообщение', metrics.first_message_users, percent(metrics.first_message_users, metrics.registrations)],
      ['Первый пост', metrics.first_post_users, percent(metrics.first_post_users, metrics.registrations)],
      ['Активные', metrics.active_users, 'уникальные пользователи'],
      ['Создали follow', metrics.follow_creators, 'уникальные инициаторы']
    ] as const;
  }, [metrics]);

  return (
    <main className="adminShell">
      <aside className="adminSidebar">
        <a className="adminBrand" href="/admin/product">CHAT <span>Admin</span></a>
        <nav>
          <a href="/admin/moderation">Модерация</a>
          <a href="/admin/security">Security</a>
          <a className="isActive" href="/admin/product">Product</a>
          <a href="/">Вернуться в CHAT</a>
        </nav>
        <div className="adminRoles">{roles.map((role) => <span key={role}>{role}</span>)}</div>
      </aside>

      <section className="adminContent">
        <header className="adminHeader">
          <div><span>PRODUCT SIGNALS</span><h1>Activation & retention</h1></div>
          <strong>{days}д</strong>
        </header>

        <div className="adminSecurityFilters" aria-label="Период">
          {WINDOWS.map((value) => (
            <button key={value} className={days === value ? 'isActive' : ''} onClick={() => setDays(value)}>
              {value} дней
            </button>
          ))}
        </div>

        {loading ? <p className="adminState">Считаем метрики…</p> : null}
        {error ? <p className="adminError" role="alert">{error}</p> : null}

        {readiness ? (
          <section className={`adminReadiness ${readiness.internal_ready ? 'isReady' : 'isBlocked'}`}>
            <Surface>
              <div className="adminReadinessHeader">
                <div><span>PUBLIC BETA READINESS</span><h2>{readiness.internal_ready ? 'Внутренние блокеры закрыты' : 'Запуск заблокирован'}</h2></div>
                <strong>{readiness.internal_ready ? 'READY' : 'BLOCKED'}</strong>
              </div>
              <div className="adminReadinessGrid">
                <div><span>Без passkey</span><strong>{readiness.privileged_without_passkey}</strong></div>
                <div><span>High/Critical alerts</span><strong>{readiness.open_high_critical_alerts}</strong></div>
                <div><span>Critical moderation</span><strong>{readiness.open_critical_cases}</strong></div>
                <div><span>Paused features</span><strong>{readiness.disabled_features.length}</strong></div>
              </div>
              {readiness.disabled_features.length ? <p>Приостановлено: {readiness.disabled_features.join(', ')}</p> : null}
              <p>Внешние обязательные gates: {readiness.external_required.join(', ')}.</p>
            </Surface>
          </section>
        ) : null}

        {metrics ? (
          <>
            <section className="adminProductGrid">
              {cards.map(([title, value, note]) => (
                <Surface className="adminMetric adminProductMetric" key={title}>
                  <span>{title}</span>
                  <strong>{value}</strong>
                  <small>{note}</small>
                </Surface>
              ))}
            </section>

            <section className="adminProductRetention">
              <Surface className="adminSecurityTop">
                <span>RETENTION</span>
                <h2>D1</h2>
                <strong>{percent(metrics.d1_retained, metrics.d1_eligible)}</strong>
                <p>{metrics.d1_retained} из {metrics.d1_eligible} eligible users вернулись на следующий день.</p>
              </Surface>
              <Surface className="adminSecurityTop">
                <span>RETENTION</span>
                <h2>D7</h2>
                <strong>{percent(metrics.d7_retained, metrics.d7_eligible)}</strong>
                <p>{metrics.d7_retained} из {metrics.d7_eligible} eligible users активны на 7-й день.</p>
              </Surface>
            </section>

            <section className="adminProductFunnel">
              <Surface>
                <span>ACQUISITION</span>
                <h2>Public → Login → Signup</h2>
                <dl>
                  <div><dt>Public views</dt><dd>{metrics.public_views}</dd></div>
                  <div><dt>Login starts</dt><dd>{metrics.login_starts} · {percent(metrics.login_starts, metrics.public_views)}</dd></div>
                  <div><dt>Completed signup events</dt><dd>{metrics.signup_completed_events} · {percent(metrics.signup_completed_events, metrics.login_starts)}</dd></div>
                  <div><dt>Invite joins</dt><dd>{metrics.invite_joins}</dd></div>
                </dl>
              </Surface>
              <Surface>
                <span>TRUST & OPERATIONS</span>
                <h2>Moderation pressure</h2>
                <strong className="adminProductBig">{metrics.open_moderation_cases}</strong>
                <p>Открытые/reviewing кейсы. Рост должен сравниваться с ростом активной аудитории, а не сам по себе.</p>
              </Surface>
            </section>

            <p className="adminNotice">
              D1/D7 считаются по durable активности CHAT: messages, posts, follows и session activity. Acquisition использует first-party growth events без сторонних трекеров и содержимого сообщений.
            </p>
          </>
        ) : null}
      </section>
    </main>
  );
}
