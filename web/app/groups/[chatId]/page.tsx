'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { AppShell } from '../../../components/AppShell';
import { Avatar, Button, Surface } from '../../../components/ui';
import {
  Group, GroupMember, createGroupInvite, getGroup, leaveGroup, listGroupMembers,
  removeGroupMember, setGroupMemberRole
} from '../../../lib/api';
import { transferGroupOwnership } from '../../../lib/groupActions';

export default function GroupPage() {
  const params = useParams<{ chatId: string }>();
  const router = useRouter();
  const chatId = params.chatId;
  const [group, setGroup] = useState<Group | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [invite, setInvite] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [busyUser, setBusyUser] = useState('');

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
      setInvite(`${window.location.origin}/join/${encodeURIComponent(token)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать приглашение');
    }
  }

  async function changeRole(member: GroupMember) {
    setBusyUser(member.user_id);
    try {
      await setGroupMemberRole(chatId, member.username, member.role === 'admin' ? 'member' : 'admin');
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось изменить роль');
    } finally {
      setBusyUser('');
    }
  }

  async function transfer(member: GroupMember) {
    if (!window.confirm(`Передать владение группой пользователю @${member.username}? Вы станете обычным участником.`)) return;
    setBusyUser(member.user_id);
    try {
      await transferGroupOwnership(chatId, member.username);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось передать владение');
    } finally {
      setBusyUser('');
    }
  }

  async function remove(member: GroupMember) {
    if (!window.confirm(`Удалить @${member.username} из группы?`)) return;
    setBusyUser(member.user_id);
    try {
      await removeGroupMember(chatId, member.username);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось удалить участника');
    } finally {
      setBusyUser('');
    }
  }

  async function leave() {
    if (!group || group.role === 'owner') return;
    if (!window.confirm('Выйти из группы?')) return;
    try {
      await leaveGroup(chatId);
      router.replace('/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось выйти из группы');
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
              {group.role !== 'owner' ? <Button variant="ghost" type="button" onClick={leave}>Выйти</Button> : null}
            </div>
            {group.role === 'owner' ? <p className="groupOwnerHint">Перед выходом владелец обязан передать группу другому участнику.</p> : null}
            {invite ? <div className="groupInviteBox"><strong>Ссылка действует 7 дней</strong><br />{invite}</div> : null}

            <div className="groupMembers">
              <h3>Участники</h3>
              {members.map((member) => (
                <div className="groupMember" key={member.user_id}>
                  <Avatar name={member.display_name || member.username} size="sm" />
                  <div className="groupMemberCopy"><strong>{member.display_name || member.username}</strong><span>@{member.username} · {member.role}</span></div>
                  {group.role === 'owner' && member.role !== 'owner' ? (
                    <div className="groupMemberActions">
                      <button type="button" disabled={busyUser === member.user_id} onClick={() => void transfer(member)}>Передать группу</button>
                      <button type="button" disabled={busyUser === member.user_id} onClick={() => void changeRole(member)}>{member.role === 'admin' ? 'Снять admin' : 'Сделать admin'}</button>
                      <button className="danger" type="button" disabled={busyUser === member.user_id} onClick={() => void remove(member)}>Удалить</button>
                    </div>
                  ) : null}
                  {group.role === 'admin' && member.role === 'member' ? (
                    <div className="groupMemberActions"><button className="danger" type="button" disabled={busyUser === member.user_id} onClick={() => void remove(member)}>Удалить</button></div>
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
