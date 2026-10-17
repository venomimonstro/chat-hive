# CHAT Hive — первый коммерческий запуск (CORE)

## Пользовательский объём
Разрешено: регистрация и вход (одноразовая ссылка, Yandex ID, passkey при настройке), настройка имени и username, выбор русского/английского языка (RU по умолчанию), безопасность аккаунта, экспорт/удаление данных, **только личные текстовые переписки**, запросы от новых контактов и блокировка пользователя.

Отключено: группы, каналы, сообщества, публикации, лента, поиск по интересам, массовые рассылки, изображения, вложения, реакции, уведомления Push и социальные функции. Код будущих функций сохраняется в репозитории, но запросы от клиентов в режиме `core` отклоняются сервером. Для обычного пользователя скрыты все ссылки на эти функции.

## Включение безопасного режима
Сервер: `CHAT_LAUNCH_MODE=core`. Это значение **по умолчанию**, а `full` в production требует отдельного подтверждения. Web: то же значение в runtime для страницы, либо не задавать. Проверяйте оба контейнера — web и API — после деплоя.

Новый профиль больше не требует выбирать интересы. Минимум: корректный username (3–32 символа) и имя. Язык хранится в `user_preferences` (миграция `000027_user_preferences`), отдается только владельцу по защищённым маршрутам и применяется к интерфейсу. Токен доступа теперь находится только в памяти вкладки; долговременная сессия восстанавливается с HttpOnly/Secure/SameSite=Strict refresh-cookie.

## Важнейшие сценарии приёмки
1. Новый пользователь получает email-ссылку, регистрируется, заполняет имя и username и входит в интерфейс без вкладок с будущими функциями.
2. Повторный вход с другого устройства; язык профиля синхронизируется RU/EN.
3. Два пользователя создают прямой чат, отправляют текст, отправляют повтор с тем же client_message_id без дубля, получают сообщения через REST и WebSocket.
4. Незнакомый пользователь не может неограниченно писать до подтверждения контакта, а блокировка прекращает контакт.
5. Пользователь может просмотреть устройства, завершить другую или текущую сессию, экспортировать и удалить данные.
6. Переключение чатов и нестабильная сеть не теряют queued messages; ручная повторная отправка доступна.
7. Любые вызовы будущих API и прямые URL закрытых страниц недоступны в режиме core.
8. На мобильном устройстве нет горизонтального скролла, кнопки ≥44px, фокус клавиатуры виден, сообщения читаются при 320px.
9. Сервис выдерживает согласованную нагрузку на действующем сервере, восстановление БД и медиа проходит из реального backup, RPO/RTO измерены.
10. Независимый пентест, анализ GDPR/152-ФЗ/согласий и актуальных договоров, проверка хранений и удаления данных, политика обработки персональных данных и реквизиты оператора согласованы до открытия коммерческого доступа.

## Исполняемая техническая приёмка

Настройте production-сервер и отдельные тестовые аккаунты A и B. Для E2E задать секретные переменные окружения (`CHAT_CORE_ACCESS_TOKEN_A`, `CHAT_CORE_ACCESS_TOKEN_B`, `CHAT_CORE_USERNAME_B`, `CHAT_CORE_URL=https://...`, `CHAT_CORE_SMOKE_CONFIRM=I_AM_USING_DEDICATED_TEST_USERS`). Test отправляет реальное сообщение в B и, при необходимости, принимает запрос; **не использовать реальные клиентские аккаунты**.

```bash
bash ops/release/prepare-lockfile.sh
# После успешного npm ci, typecheck/build сохранить настоящий web/package-lock.json в Git
# И обновить код на сервере до этого коммита
CHAT_ENV=production CHAT_LAUNCH_MODE=core bash ops/core/gate.sh
```

Гейт запускает полный build/test/migration gate, затем нагружает действующий Alpha-хост и проверяет изолированное восстановление, затем выполняет два пользовательских E2E сценария. Дополнительно нужны `CHAT_CORE_PENTEST_REPORT`, `CHAT_CORE_LEGAL_SIGNOFF`, `CHAT_CORE_EXTERNAL_ATTESTATION` (валидируемый JSON с SHA-256 документов и release SHA). Он блокирует выпуск при их отсутствии. Отдельно необходимы переменные для `ops/alpha/gate.sh` (см. `docs/CLOSED_ALPHA_RUNBOOK.md`).

## Ключевые риски

- **Не E2E-шифрование:** сообщения сервером хранятся и обрабатываются в открытом виде внутри доверенной инфраструктуры. Не заявлять пользователям защиту от самого оператора до внедрения проверенного end-to-end протокола.
- **Недостаточна проверка CSP:** Next.js сейчас использует `script-src 'unsafe-inline'`; риск XSS требует независимого тестирования и последующего CSP с nonce/hash.
- **Резервные копии:** проверены сценарии отказов на уровне скриптов, но шифрование вне сервера, off-site копии, ротация и восстановление по реальным данным ещё нужно документально подтвердить.
- **Нагрузочные требования:** нельзя гарантировать стабильность, задержки и устойчивость к DDoS без результатов на реальном хосте, настройки rate limits и сетевых фильтров.
- **Соответствие требованиям законодательства:** зависит от юрисдикции, места хранения данных, оператора сервиса, email/SMTP провайдера, пользовательских соглашений. Нужна юридическая экспертиза.

**Вывод:** source-level режим `core` подготовлен; реальная коммерческая готовность наступает только после успешного `ops/core/gate.sh`, независимого пентеста и подписанного акта приёмки. 

## Шифрование для внешнего резервного хранения

Архивы `postgres.dump` и `media.tar.gz` внутри резервного каталога по умолчанию **не зашифрованы**. Поэтому не выкладывайте их напрямую на внешние диски/в облако. Для экспорта используйте `age` с публичным ключом восстановления; приватный ключ держите на отдельном доверенном устройстве:

```bash
export CHAT_BACKUP_AGE_RECIPIENT='age1...'
export CHAT_ENCRYPTED_BACKUP_DIR=/secure/offsite-staging
bash ops/backup/export-encrypted.sh /srv/chat-hive/backups/<snapshot>
```

Перенесите `.age` и `.sha256` в независимое хранилище с ограниченными правами и версионированием. Восстановите копию в отдельной среде, используя приватный ключ, после чего проведите `ops/backup/drill.sh`. Для финального `ops/core/gate.sh` задайте `CHAT_CORE_ENCRYPTED_BACKUP_FILE` и `CHAT_CORE_ENCRYPTED_BACKUP_SHA256`. Контрольная сумма подтверждает целостность копии, но не её расшифровываемость — это доказывает только реальный тест восстановления на независимом узле.

## Final audit hardening — 2026-10-17

Additional changes after initial core-launch implementation:

- `CHAT_LAUNCH_MODE=core` registers *only* launch-needed public HTTP handlers. Group, channel, media, feed, search and notification endpoints do not exist in the core routing tree; the independent allowlist still blocks accidental exposure. Media-storage configuration is unnecessary in text-only mode.
- Authorizations for authentication routes use an explicit method/path allowlist, not a broad `/auth/*` wildcard. Public social profiles and reactions are rejected; contact blocking remains allowed.
- Browser UI removes group and reaction handling. The Russian/English preference reloads through a refresh-cookie-backed session after the page is refreshed. User bearer tokens remain in tab memory, not `sessionStorage`.
- Account export is staged privately before the server sends success headers. A failed export cannot return truncated JSON with HTTP 200. The temporary file has mode 0600 and is deleted after delivery; single-request size limit is 256 MiB.
- POST/PUT authentication, onboarding and messaging decoders reject multiple JSON values and unexpected fields. Deletion of accounts also rejects trailing data.
- The core two-user smoke checks that `GET /api/v1` reports `launch_mode: core` before making test writes. Even if the wrong deployment configuration disables the route filter, the smoke will refuse release.

**Explicit unverified areas:** frontend `package-lock.json` is missing; the entire production build/typecheck and backend race suite have not run in this editing environment (Go toolchain local is older than the required module toolchain and network/DNS access is unavailable). Hardware Passkeys, real HTTP/CDN/browser compatibility, load characteristics, backup recovery, XSS/pentesting, operational key management, compliance, and accessibility require external sign-off. Therefore there is **no release approval** yet; do not bypass `ops/core/gate.sh`.
