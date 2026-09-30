'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Avatar, Button, Surface } from '../../../components/ui';
import {
  Group, GroupMember, createGroupInvite, getGroup, listGroupMembers,
  removeGroupMember, setGroupMemberRole
} from '../../../lib/api';

export default function GroupPage() {
  const params = useParams<{ chatId: string }>();
  const chatId = params.chatId;
  const [group, setGroup] = useState<Group | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [invite, setInvite] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  async function reload() {
    try {
      const [nextGroup, nextMembers] = await Promise.all([getGroup(chatId), listGroupMembers(chatId)]);
      setGroup(nextGroup);
      setMembers(nextMembers);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить группу');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void reload(); }, [chatId]);

  async function makeInvite() {
    try {
      const token = await createGroupInvite(chatId, 168, 100);
      const base = window.location.origin;
      setInvite(`${base}/join/${encodeURIComponent(token)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать приглашение');
    }
  }

  async function changeRole(member: GroupMember) {
    try {
      await setGroupMemberRole(chatId, member.username, member.role === 'admin' ? 'member' : 'admin');
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось изменить роль');
    }
  }

  async function remove(member: GroupMember) {
    try {
      await removeGroupMember(chatId, member.username);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось удалить участника');
    }
  }

  return (
    <AppShell active="Чаты">
      <header className="screenHeader">
        <div><span className="eyebrow">CHAT / ГРУППА</span><h1>{group?.title ?? 'Группа'}</h1></div>
      </header>
      <div className="groupPageBody">
        {loading ? <div className="chatListState">Загружаем группу…</div> : null}
        {error ? <div className="messengerError" role="alert">{error}</div> : null}
        {group ? (
          <Surface className="groupCard">
            <h2>{group.title}</h2>
            <p>{group.description || 'Без описания'}</p>
            <p><strong>{group.members_count}</strong> участников · ваша роль: <strong>{group.role}</strong></p>
            <div className="groupActions">
              <a className="uiButton uiButton--primary" href={`/?chat=${encodeURIComponent(group.chat_id)}`}>Открыть чат</a>
              {(group.role === 'owner' || group.role === 'admin') ? <Button variant="secondary" type="button" onClick={makeInvite}>Пригласить</Button> : null}
            </div>
            {invite ? <div className="groupInviteBox"><strong>Ссылка действует 7 дней</strong><br />{invite}</div> : null}

            <div className="groupMembers">
              <h3>Участники</h3>
              {members.map((member) => (
                <div className="groupMember" key={member.user_id}>
                  <Avatar name={member.display_name || member.username} size="sm" />
                  <div className="groupMemberCopy"><strong>{member.display_name || member.username}</strong><span>@{member.username} · {member.role}</span></div>
                  {group.role === 'owner' && member.role !== 'owner' ? (
                    <div className="groupMemberActions">
                      <button type="button" onClick={() => void changeRole(member)}>{member.role === 'admin' ? 'Снять admin' : 'Сделать admin'}</button>
                      <button className="danger" type="button" onClick={() => void remove(member)}>Удалить</button>
                    </div>
                  ) : null}
                  {group.role === 'admin' && member.role === 'member' ? (
                    <div className="groupMemberActions"><button className="danger" type="button" onClick={() => void remove(member)}>Удалить</button></div>
                  ) : null}
                </div>
              ))}
            </div>
          </Surface>
        ) : null}
      </div>
    </AppShell>
  );
}
