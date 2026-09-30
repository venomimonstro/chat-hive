# CHAT Production Release Checklist

Релиз запрещён, если любой пункт категории BLOCKER не выполнен.

## 1. BLOCKER — Security

- [ ] Нет известных Critical vulnerabilities.
- [ ] Нет необъяснённых/непринятых High vulnerabilities.
- [ ] Admin/Owner privileged accounts используют passkey/MFA согласно политике.
- [ ] Production secrets отсутствуют в Git, frontend bundle и логах.
- [ ] Session rotation/revocation и refresh-token replay detection работают.
- [ ] Object-level authorization проверен для chats/messages/groups/posts/media/admin.
- [ ] `/api/*` не кэшируется публичными/shared caches.
- [ ] WebSocket проверяет origin + one-time ticket + session membership.
- [ ] Media upload ограничивает размер/формат/пиксели и не публикует upload автоматически.
- [ ] Admin audit/security events append-only.
- [ ] Incident runbook актуален.

## 2. BLOCKER — Data / migrations

- [ ] Migration sequence уникальна, без дублирующихся номеров/создания одних таблиц дважды.
- [ ] Clean install на пустой БД проходит все миграции.
- [ ] Rollback-путь задокументирован для последнего schema change.
- [ ] Backup создан и checksum проверен.
- [ ] Restore выполнен на отдельной БД и проверен.
- [ ] Media backup/restore проверен.

## 3. BLOCKER — Production configuration

- [ ] `CHAT_ENV=production`.
- [ ] `CHAT_DATABASE_URL` задан явно и не использует dev credentials.
- [ ] `CHAT_WEB_ORIGIN` HTTPS.
- [ ] `CHAT_MAGIC_LINK_BASE_URL` HTTPS.
- [ ] SMTP production sender настроен и домен не `example.com`.
- [ ] Yandex OAuth redirect HTTPS, если интеграция включена.
- [ ] Cookie Secure включён.
- [ ] PostgreSQL/Redis/NATS не опубликованы напрямую в Internet.
- [ ] Media storage writable только нужным процессом.

## 4. BLOCKER — Messaging

- [ ] Idempotency: повтор одного `client_message_id` не создаёт дубль.
- [ ] Offline outbox переживает reload/offline и досылает тот же ID.
- [ ] Reconnect не теряет сообщения; REST sync восстанавливает состояние после WebSocket gap.
- [ ] Read cursor не может перейти дальше существующего sequence.
- [ ] Stranger request не даёт незнакомцу неограниченно писать до accept.
- [ ] Block прекращает direct interaction и исключает рекомендации.

## 5. BLOCKER — Performance / availability

- [ ] Health/readiness endpoints проверены.
- [ ] HTTP load test зафиксировал p50/p95/p99 и error rate.
- [ ] WebSocket connection/reconnect scenario протестирован.
- [ ] Reconnect storm не вызывает мгновенный self-DDoS благодаря backoff+jitter.
- [ ] DB indexes проверены на основных list/history/feed запросах.
- [ ] Деградация сохраняет Messaging раньше Feed/Discovery.

## 6. BLOCKER — Trust & Safety

- [ ] Report → Case → Decision → Enforcement работает без auto-ban по одной жалобе.
- [ ] Public community/channel не попадает в каталог до moderation approval.
- [ ] Admin content access case-scoped и аудируется.
- [ ] Group owner/admin moderation не даёт доступа к чужим группам.
- [ ] Anti-spam/request inbox/rate limits включены.
- [ ] Политические/религиозные убеждения не используются как hidden risk score.

## 7. Product quality

- [ ] Signup → onboarding → first follow/community проходит без тупиков.
- [ ] Profile → message/follow/post работает.
- [ ] Create Hub не содержит недоступных действий.
- [ ] Discover feedback («больше/меньше/скрыть») сохраняется.
- [ ] Low Data Mode реально блокирует/уменьшает media download.
- [ ] Public permalink открывается корректно.
- [ ] Mobile navigation не имеет dead routes.

## 8. Release record

Перед релизом записать:
- commit SHA;
- migration version;
- backup ID/checksum;
- дата restore-test;
- результаты load test;
- известные non-blocking issues;
- ответственный за rollback.

Релиз считается состоявшимся только после post-deploy smoke check: login, profile, direct message, group message, post, Discover, report, admin moderation и media read.
