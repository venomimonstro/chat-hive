# CHAT Security Incident Runbook

Этот runbook используется при инцидентах production. Цель — быстро ограничить blast radius, сохранить доказательства и восстановить сервис без импровизации.

## 1. Severity

- **P0 Critical** — подтверждённая утечка данных, RCE, компрометация admin/control plane, signing/session key compromise, массовый account takeover, ransomware.
- **P1 High** — активная эксплуатация уязвимости без подтверждённой эксфильтрации, массовый fraud/abuse, частичная компрометация узла.
- **P2 Medium** — ограниченная атака, подозрительная активность, локальный ATO.
- **P3 Low** — сканирование, единичные неуспешные попытки.

## 2. Первые 15 минут

1. Назначить Incident Commander.
2. Зафиксировать время начала, источник сигнала и затронутые компоненты.
3. Не удалять логи и не перезапускать систему без причины.
4. Ограничить доступ атакующего минимальным изменением:
   - revoke session/token;
   - disable compromised admin;
   - block malicious IP/domain;
   - isolate affected container/node;
   - disable vulnerable feature через feature/edge controls.
5. Сохранить evidence: audit IDs, security events, request IDs, relevant DB rows, container/system logs.
6. Определить, затронуты ли персональные данные и требуется ли legal/compliance escalation.

## 3. Account takeover

- Revoke affected session(s).
- При refresh-token replay считать сессию скомпрометированной.
- Завершить все сессии пользователя при признаках массового захвата.
- Проверить изменения identity, username, email/provider bindings, admin roles.
- Уведомить пользователя после стабилизации, если это допустимо и необходимо.

## 4. Admin compromise

- Немедленно disable/revoke admin account and sessions.
- Проверить `admin_audit_events` начиная минимум за 24 часа до первого подозрительного события.
- Проверить role changes, data views/exports, moderation actions, security policy changes.
- Ротировать credentials, к которым мог получить доступ администратор.
- При признаках массовой выгрузки классифицировать как P0.

## 5. Database/data exfiltration

- Изолировать источник доступа, не уничтожая доказательства.
- Ротировать DB credentials и связанные secrets.
- Проверить audit/security logs и объём затронутых записей.
- Создать неизменяемый incident snapshot.
- Не выполнять restore поверх production до фиксации причины компрометации.
- Запустить legal/privacy incident workflow.

## 6. RCE / compromised container

- Изолировать контейнер/узел от internal network.
- Не считать соседние сервисы безопасными до проверки credentials/network paths.
- Ротировать secrets, доступные процессу.
- Пересобрать образ из доверенного source/revision, не «лечить» скомпрометированный контейнер вручную.
- Проверить persistence, cron/systemd, SSH keys и изменённые файлы host-системы.

## 7. DDoS / reconnect storm

Приоритет деградации:
1. Messaging
2. Authentication
3. Profiles
4. Feed
5. Discovery/analytics

Действия:
- включить/усилить edge rate limits;
- ограничить unauthenticated/WebSocket connection rate;
- временно уменьшить Feed/Discovery traffic;
- проверить, что клиенты используют exponential backoff + jitter;
- не отключать messaging первым.

## 8. Secret/signing key compromise

- Активировать резервный ключ/credential.
- Отозвать скомпрометированный ключ.
- Для session/signing compromise инвалидировать затронутые сессии.
- Ротировать downstream credentials, если ключ мог дать к ним доступ.
- Зафиксировать key ID и период потенциальной компрометации.

## 9. Malicious dependency / supply-chain

- Заморозить deploy.
- Определить версии и затронутые артефакты по lockfiles/SBOM.
- Откатиться на последнюю проверенную сборку только если схема данных совместима.
- Ротировать secrets, доступные скомпрометированной сборке.
- Не снимать incident до проверки persistence/exfiltration.

## 10. Recovery gate

Инцидент не считается закрытым, пока:
- причина установлена или изолирована;
- активный доступ атакующего прекращён;
- secrets ротированы при необходимости;
- integrity production проверена;
- backup/restore не требуется либо проверен;
- создан postmortem;
- заведены corrective actions с владельцами.

## 11. Postmortem

В течение 72 часов для P0/P1 документировать:
- timeline;
- root cause;
- blast radius;
- detection gap;
- containment/recovery;
- какие данные затронуты;
- почему существующие controls не остановили инцидент;
- конкретные изменения кода/архитектуры/процессов.

Postmortem не должен скрывать технические причины ради «красивого отчёта».
