# PostgreSQL backup и restore drill

Обновлено: 22 октября 2026 года.

## Цели пилота

- RPO: не более 24 часов при ежедневном полном backup; перед релизом и рискованной миграцией создаётся дополнительный backup.
- RTO: не более 60 минут от решения о восстановлении до успешного synthetic check.
- Retention: 14 ежедневных копий и не менее трёх ежемесячных копий во внешнем object storage.
- Backup считается существующим только после успешного расшифрования, checksum verification и restore drill в отдельную БД.

## Требования

На backup runner должны быть совместимые с сервером `pg_dump`/`pg_restore`, `age`, `sha256sum` и `tar`. Приватный age identity хранится в secret manager вне БД, контейнеров, Git и backup bucket. В job передаётся только публичный recipient.

## Создание backup

```bash
export DATABASE_URL='postgres://.../restaurant_delivery?sslmode=require'
export BACKUP_AGE_RECIPIENT='age1...'
export BACKUP_RETENTION_DAYS=14
scripts/backup-postgres.sh /secure/backups
```

Скрипт создаёт consistent custom-format `pg_dump`, manifest с UTC-временем и версией клиента, считает SHA-256, шифрует bundle через age и только затем атомарно публикует файл `postgres-*.tar.age`. Незашифрованные временные файлы удаляются при любом завершении.

После создания файл необходимо загрузить в versioned object storage с server-side encryption, retention/immutability policy и отдельными credentials только на запись. Локальная директория не является резервным хранилищем.

## Восстановление

Всегда создавайте новую пустую БД. Не восстанавливайте поверх работающей production-БД.

```bash
export RESTORE_DATABASE_URL='postgres://.../restaurant_delivery_restore?sslmode=require'
export RESTORE_AGE_IDENTITY_FILE='/run/secrets/backup-age-identity'
export ALLOW_DATABASE_RESTORE=true
scripts/restore-postgres.sh /secure/backups/postgres-YYYYMMDDTHHMMSSZ.tar.age
```

Restore job сначала расшифровывает bundle во временный каталог, проверяет SHA-256 и TOC архива, затем выполняет `pg_restore` в одной транзакции. Любая ошибка оставляет target непригодным; target удаляется и создаётся заново перед повтором.

## Проверка перед переключением трафика

1. Проверить последнюю чистую версию `schema_migrations`.
2. Сравнить counts и контрольные агрегаты ресторанов, меню, заказов и событий с manifest/последним отчётом.
3. Запустить API на восстановленной БД и дождаться readiness.
4. Выполнить synthetic регистрацию → quote → create → status без внешних побочных эффектов.
5. Зафиксировать начало, окончание, размер копии, фактические RPO/RTO и ответственного.
6. Переключить приложение только после письменного решения incident commander; старую БД оставить read-only до завершения проверки.

## Автоматическая проверка

CI после backend E2E создаёт одноразовый age identity, шифрует backup заполненной тестовой БД, восстанавливает его в новую БД и сравнивает counts ключевых таблиц, включая partner credentials/audit, и migration version. Одноразовый ключ и обе БД удаляются вместе с CI stack.

## Расписание

- ежедневно: encrypted backup и проверка возможности прочитать manifest;
- еженедельно: автоматический restore в изолированную БД и synthetic check;
- ежемесячно: ручной drill по этому runbook с измерением RPO/RTO;
- перед миграцией: on-demand backup и подтверждение rollback/forward-fix стратегии.
