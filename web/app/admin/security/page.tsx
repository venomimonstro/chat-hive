'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Surface } from '../../../components/ui';
import {
  acknowledgeSecurityAlert, getAdminPrincipal, getSecuritySummary, listPlatformFlags, listSecurityAlerts,
  listSecurityEvents, PlatformFlag, SecurityAlert, SecurityEvent, SecuritySummary, setPlatformFlag
} from '../../../lib/admin';

const FILTERS = ['', 'critical', 'high', 'medium', 'low', 'info'] as const;

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export default function SecurityAdminPage() {
  const router = useRouter();
  const [roles, setRoles] = useState<string[]>([]);
  const [events, setEvents] = useState<SecurityEvent[]>([]);
  const [alerts, setAlerts] = useState<SecurityAlert[]>([]);
  const [summary, setSummary] = useState<SecuritySummary | null>(null);
  const [flags, setFlags] = useState<PlatformFlag[]>([]);
  const [changingFlag, setChangingFlag] = useState('');
  const [severity, setSeverity] = useState('');
  const [selected, setSelected] = useState<SecurityEvent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [acknowledging, setAcknowledging] = useState<number | null>(null);

  async function bootstrap() {
    setError('');
    try {
      const principal = await getAdminPrincipal();
      if (!principal) { router.replace('/'); return; }
      if (!principal.roles.some((role) => role === 'security' || role === 'owner')) { router.replace('/admin/moderation'); return; }
      setRoles(principal.roles);
      const [items, openAlerts, securitySummary, platformFlags] = await Promise.all([
        listSecurityEvents(''),
        listSecurityAlerts('open'),
        getSecuritySummary(),
        listPlatformFlags()
      ]);
      setEvents(items);
      setAlerts(openAlerts);
      setSummary(securitySummary);
      setFlags(platformFlags);
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') { router.replace('/login'); return; }
      setError(err instanceof Error ? err.message : 'Не удалось загрузить Security Plane');
    } finally { setLoading(false); }
  }

  async function loadEvents(filter: string) {
    setError('');
    try {
      const items = await listSecurityEvents(filter);
      setEvents(items);
      if (selected && !items.some((item) => item.id === selected.id)) setSelected(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить security events');
    }
  }

  async function changeFlag(flag: PlatformFlag) {
    const nextEnabled = !flag.enabled;
    const reason = window.prompt(`Причина изменения ${flag.key}`);
    if (!reason?.trim()) return;
    setChangingFlag(flag.key);
    setError('');
    try {
      await setPlatformFlag(flag.key, nextEnabled, reason.trim());
      setFlags(await listPlatformFlags());
      setAlerts(await listSecurityAlerts('open'));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось изменить operational control');
    } finally {
      setChangingFlag('');
    }
  }

  async function acknowledge(alert: SecurityAlert) {
    const note = window.prompt('Кратко зафиксируйте, что проверено и какое действие принято:');
    if (!note?.trim()) return;
    setAcknowledging(alert.event_id);
    setError('');
    try {
      await acknowledgeSecurityAlert(alert.event_id, note.trim());
      setAlerts((current) => current.filter((item) => item.event_id !== alert.event_id));
      const [items, securitySummary] = await Promise.all([listSecurityEvents(severity), getSecuritySummary()]);
      setEvents(items);
      setSummary(securitySummary);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось подтвердить alert');
    } finally {
      setAcknowledging(null);
    }
  }

  useEffect(() => { void bootstrap(); }, []);
  useEffect(() => { if (!loading) void loadEvents(severity); }, [severity]);

  return (
    <main className="adminShell">
      <aside className="adminSidebar">
        <a className="adminBrand" href="/admin/moderation">CHAT <span>Admin</span></a>
        <nav><a href="/admin/moderation">Модерация</a><a className="isActive" href="/admin/security">Безопасность</a><a href="/">Вернуться в CHAT</a></nav>
        <div className="adminRoles">{roles.map((role) => <span key={role}>{role}</span>)}</div>
      </aside>
      <section className="adminContent">
        <header className="adminHeader"><div><span>SECURITY PLANE</span><h1>Security events</h1></div><strong>{summary?.critical_15m ?? 0}</strong></header>

        <section className="adminAlertQueue" aria-label="Открытые security alerts">
          <div className="adminAlertQueueHeader">
            <div><span>INCIDENT QUEUE</span><h2>Open alerts</h2></div>
            <strong>{alerts.length}</strong>
          </div>
          {alerts.length ? (
            <div className="adminAlertList">
              {alerts.map((alert) => (
                <Surface className="adminAlertItem" key={alert.event_id}>
                  <div className="adminAlertHeadline">
                    <span className={`adminSeverity severity-${alert.severity}`}>{alert.severity}</span>
                    <time>{formatDate(alert.created_at)}</time>
                  </div>
                  <strong>{alert.event_type}</strong>
                  <p>{alert.subject_type}{alert.subject_id ? ` · ${alert.subject_id}` : ''}{alert.source_ip ? ` · ${alert.source_ip}` : ''}</p>
                  <button type="button" disabled={acknowledging === alert.event_id} onClick={() => void acknowledge(alert)}>
                    {acknowledging === alert.event_id ? 'Фиксируем…' : 'Проверено / acknowledge'}
                  </button>
                </Surface>
              ))}
            </div>
          ) : <Surface className="adminEmpty">Открытых high/critical alerts нет.</Surface>}
        </section>

        {summary ? (
          <section className="adminSecuritySummary" aria-label="Security summary">
            <Surface className="adminMetric"><span>Critical · 15м</span><strong>{summary.critical_15m}</strong></Surface>
            <Surface className="adminMetric"><span>High · 15м</span><strong>{summary.high_15m}</strong></Surface>
            <Surface className="adminMetric"><span>Critical/High · 1ч</span><strong>{summary.critical_1h + summary.high_1h}</strong></Surface>
            <Surface className="adminMetric"><span>Events · 24ч</span><strong>{summary.events_24h}</strong></Surface>
            <Surface className="adminSecurityTop">
              <h2>Типы событий · 1ч</h2>
              {summary.top_event_types_1h.length ? summary.top_event_types_1h.map((item) => <div key={item.key}><span>{item.key}</span><strong>{item.count}</strong></div>) : <p>Событий нет.</p>}
            </Surface>
            <Surface className="adminSecurityTop">
              <h2>Источники · 1ч</h2>
              {summary.top_source_ips_1h.length ? summary.top_source_ips_1h.map((item) => <div key={item.key}><span>{item.key}</span><strong>{item.count}</strong></div>) : <p>IP-сигналов нет.</p>}
            </Surface>
          </section>
        ) : null}

        <div className="adminSecurityFilters" aria-label="Фильтр важности">
          {FILTERS.map((item) => <button type="button" key={item || 'all'} className={severity === item ? 'isActive' : ''} onClick={() => setSeverity(item)}>{item || 'all'}</button>)}
        </div>
        {loading ? <p className="adminState">Загружаем Security Plane…</p> : null}
        {error ? <p className="adminError" role="alert">{error}</p> : null}
        <div className="adminGrid">
          <div className="adminCaseList">
            {events.map((item) => (
              <button type="button" key={item.id} className={`adminCase ${selected?.id === item.id ? 'isSelected' : ''}`} onClick={() => setSelected(item)}>
                <div><span className={`adminSeverity severity-${item.severity}`}>{item.severity}</span><time>{formatDate(item.created_at)}</time></div>
                <strong>{item.event_type}</strong>
                <p>{item.subject_type}{item.subject_id ? ` · ${item.subject_id}` : ''}{item.source_ip ? ` · ${item.source_ip}` : ''}</p>
              </button>
            ))}
            {!loading && events.length === 0 ? <Surface className="adminEmpty">Security events по выбранному фильтру отсутствуют.</Surface> : null}
          </div>
          <div className="adminCasePanel">
            {selected ? <Surface className="adminCaseDetails">
              <span className={`adminSeverity severity-${selected.severity}`}>{selected.severity}</span>
              <h2>{selected.event_type}</h2>
              <dl>
                <div><dt>Время</dt><dd>{formatDate(selected.created_at)}</dd></div>
                <div><dt>User</dt><dd>{selected.user_id ?? '—'}</dd></div>
                <div><dt>Session</dt><dd>{selected.session_id ?? '—'}</dd></div>
                <div><dt>IP</dt><dd>{selected.source_ip ?? '—'}</dd></div>
                <div><dt>Subject</dt><dd>{selected.subject_type || '—'} {selected.subject_id}</dd></div>
              </dl>
              <p className="adminNotice">Security feed содержит технические сигналы и не предоставляет доступ к содержимому пользовательских сообщений.</p>
              <pre className="adminSecurityMetadata">{JSON.stringify(selected.metadata ?? {}, null, 2)}</pre>
            </Surface> : <Surface className="adminEmpty">Выберите security event.</Surface>}
          </div>
        </div>
      </section>
    </main>
  );
}
