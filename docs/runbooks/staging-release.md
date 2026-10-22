# Runbook: staging release и rollback

## Предварительные условия

- DNS `STAGING_HOST` указывает на staging host, TCP 80/443 доступны для ACME;
- PostgreSQL, ClickHouse и внутренние сервисы не имеют host ports;
- файлы secrets принадлежат deployment user, имеют mode `0600` и находятся вне репозитория;
- `PLATFORM_IMAGE`, `DEMO_IMAGE` и `WEB_IMAGE` заданы immutable digest, не `latest`;
- перед релизом CI для того же commit SHA полностью зелёный.

Каталог `STAGING_SECRETS_DIR` содержит `postgres_password`, `database_url`, `jwt_secret`, `clickhouse_password`, `demo_partner_key`. `database_url` использует hostname `postgres`, TLS-настройки выбранного staging PostgreSQL и тот же пароль, что `postgres_password`. JWT secret — не менее 32 случайных символов. Partner key — не менее 32 случайных байт в URL-safe представлении.

## Первый запуск

```bash
cp deploy/staging/.env.example deploy/staging/.env
docker compose --env-file deploy/staging/.env -f deploy/staging/compose.yml --profile release run --rm migrations up
docker compose --env-file deploy/staging/.env -f deploy/staging/compose.yml --profile release run --rm partner-bootstrap
docker compose --env-file deploy/staging/.env -f deploy/staging/compose.yml --profile demo up -d --wait
```

Migration job читает database URL из Docker secret. Bootstrap создаёт новый owner credential из secret, пишет audit record и отзывает известный legacy key. Само приложение читает DB, JWT и ClickHouse secrets через `_FILE` и не получает их значениями в container environment.

## Release

1. Зафиксировать текущие image digests и migration version.
2. Выполнить encrypted backup и сохранить checksum/metadata.
3. Запустить `migrations up` отдельным release job до обновления приложений. Миграции должны оставаться forward-compatible с предыдущей версией API.
4. Обновить immutable digests и `APP_VERSION` в `.env`.
5. Выполнить `docker compose ... --profile demo up -d --wait --remove-orphans`.
6. Проверить `/health/ready`, HTTPS/HSTS, synthetic quote → create → partner status и Grafana SLO.
7. Наблюдать 5xx, p95, PostgreSQL saturation и outbox oldest age не менее 15 минут.

## Rollback

1. Вернуть предыдущие application image digests и выполнить `up -d --wait`. Не откатывать схему автоматически.
2. Если старая версия несовместима с уже применённой схемой, остановить трафик и следовать заранее проверенному migration plan; `down` на заполненной production-like БД без письменного решения запрещён.
3. При повреждении данных восстановить backup в новую БД по runbook PostgreSQL, проверить counts/version, затем переключить `database_url` secret и пересоздать API.
4. Зафиксировать время, причину, digests, migration version и результат rollback в incident/release record.

## Проверки изоляции

`docker compose ps` должен показывать host bindings только у ingress 80/443. PostgreSQL 5432, ClickHouse 8123/9000, API 8080 и web 8080 доступны только во внутренних Docker networks. `/metrics` не проксируется публичным ingress.
