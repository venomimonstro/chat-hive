'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, Surface } from '../../../components/ui';
import { decideModerationCase, getAdminPrincipal, listModerationCases, ModerationCase } from '../../../lib/admin';

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

export default function ModerationAdminPage() {
  const router = useRouter();
  const [cases, setCases] = useState<ModerationCase[]>([]);
  const [roles, setRoles] = useState<string[]>([]);
  const [selected, setSelected] = useState<ModerationCase | null>(null);
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setError('');
    try {
      const principal = await getAdminPrincipal();
      if (!principal) {
        router.replace('/');
        return;
      }
      setRoles(principal.roles);
      setCases(await listModerationCases());
    } catch (err) {
      if (err instanceof Error && err.message === 'Authentication required') {
        router.replace('/login');
        return;
      }
      setError(err instanceof Error ? err.message : 'Не удалось загрузить админку');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  const canDecide = roles.some((role) => ['senior_moderator', 'security', 'legal', 'owner'].includes(role));

  async function decide(event: FormEvent, decision: 'resolve' | 'dismiss') {
    event.preventDefault();
    if (!selected || !canDecide || reason.trim().length < 3 || busy) return;
    setBusy(true);
    setError('');
    try {
      await decideModerationCase(selected.id, decision, reason.trim());
      setCases((current) => current.filter((item) => item.id !== selected.id));
      setSelected(null);
      setReason('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось завершить кейс');
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="adminShell">
      <aside className="adminSidebar">
        <a className="adminBrand" href="/admin/moderation">CHAT <span>Admin</span></a>
        <nav><a className="isActive" href="/admin/moderation">Модерация</a><a href="/admin/security">Security</a>{roles.includes('owner') ? <a href="/admin/product">Product</a> : null}<a href="/">Вернуться в CHAT</a></nav>
        <div className="adminRoles">{roles.map((role) => <span key={role}>{role}</span>)}</div>
      </aside>
      <section className="adminContent">
        <header className="adminHeader"><div><span>TRUST & SAFETY</span><h1>Очередь модерации</h1></div><strong>{cases.length}</strong></header>
        {loading ? <p className="adminState">Загружаем очередь…</p> : null}
        {error ? <p className="adminError" role="alert">{error}</p> : null}
        <div className="adminGrid">
          <div className="adminCaseList">
            {cases.map((item) => (
              <button type="button" key={item.id} className={`adminCase ${selected?.id === item.id ? 'isSelected' : ''}`} onClick={() => { setSelected(item); setReason(''); }}>
                <div><span className={`adminSeverity severity-${item.severity}`}>{item.severity}</span><time>{formatDate(item.created_at)}</time></div>
                <strong>{item.target_type} · {item.report_count} жалоб</strong>
                <p>{item.reason || 'Без первичного описания'}</p>
              </button>
            ))}
            {!loading && cases.length === 0 ? <Surface className="adminEmpty">Открытых кейсов нет.</Surface> : null}
          </div>
          <div className="adminCasePanel">
            {selected ? (
              <Surface className="adminCaseDetails">
                <span className={`adminSeverity severity-${selected.severity}`}>{selected.severity}</span>
                <h2>{selected.target_type}</h2>
                <code>{selected.target_id}</code>
                <dl><div><dt>Жалоб</dt><dd>{selected.report_count}</dd></div><div><dt>Создан</dt><dd>{formatDate(selected.created_at)}</dd></div><div><dt>Статус</dt><dd>{selected.status}</dd></div></dl>
                <p className="adminNotice">На этом экране нет автоматического доступа к содержимому приватной переписки. Контент-доступ будет отдельным case-scoped действием с дополнительным аудитом.</p>
                {canDecide ? (
                  <form className="adminDecision">
                    <label><span>Основание решения</span><textarea value={reason} onChange={(e) => setReason(e.target.value)} maxLength={1000} rows={5} /></label>
                    <div><Button type="button" onClick={(event) => void decide(event as unknown as FormEvent, 'resolve')} disabled={busy || reason.trim().length < 3}>Подтвердить нарушение</Button><Button type="button" variant="secondary" onClick={(event) => void decide(event as unknown as FormEvent, 'dismiss')} disabled={busy || reason.trim().length < 3}>Отклонить жалобы</Button></div>
                  </form>
                ) : <p className="adminNotice">Ваша роль позволяет просматривать очередь, но не завершать кейсы.</p>}
              </Surface>
            ) : <Surface className="adminEmpty">Выберите кейс из очереди.</Surface>}
          </div>
        </div>
      </section>
    </main>
  );
}
