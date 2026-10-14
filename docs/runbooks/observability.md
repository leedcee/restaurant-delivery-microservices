# Runbook: базовая наблюдаемость

Локальный контур запускается командой `make observability-up`. Prometheus доступен на `http://localhost:9090`, Grafana — на `http://localhost:3000` с логином `admin` и паролем из `GRAFANA_ADMIN_PASSWORD` (`admin` по умолчанию только для локальной разработки). Loki, Tempo и OpenTelemetry Collector доступны только во внутренней сети Compose; работать с логами и traces следует через Grafana Explore.

## PlatformAPIDown

1. Проверить `http://localhost:8080/health/live` и `http://localhost:8080/health/ready`.
2. Проверить состояние контейнеров и последние логи `platform-api`.
3. Если liveness работает, а readiness нет, проверить PostgreSQL и строку подключения.
4. После восстановления убедиться, что Prometheus target `platform-api` имеет состояние `UP`.

## PlatformAPIHighErrorRate

1. На dashboard `Service Overview` определить маршруты и статусы с ростом ошибок.
2. Сопоставить интервал с JSON-логами API по `request_id`.
3. Если лог содержит `trace_id`, открыть связанную трассу через derived field `TraceID` в Loki.
4. Проверить доступность PostgreSQL и заполнение пула соединений.
5. Не отключать алерт до устранения причины; единичные клиентские 4xx в него не входят.

## OutboxDeliveryStalled

1. Проверить `outbox_backlog_events` и `outbox_oldest_event_age_seconds`.
2. Проверить доступность restaurant endpoint и логи worker по `event_id` и `order_id`.
3. Убедиться, что новые попытки выполняются и значение `attempts` растёт.
4. После восстановления дождаться уменьшения backlog до нуля.

## OutboxDeadLettersPresent

1. Получить список через `GET /partner/v1/outbox/dead-letters` с API-ключом нужного ресторана.
2. Устранить причину постоянной ошибки интеграции.
3. Переиграть событие через `POST /partner/v1/outbox/dead-letters/{eventID}/replay`.
4. Убедиться, что событие обработано один раз, а счётчик dead-letter уменьшился.

Метрики используют только ограниченные labels `method`, шаблон `route` и `status`. UUID, пользовательские данные, адреса, email, JWT и API-ключи в labels не добавляются.

## AnalyticsIngestionStalled

1. Проверить `analytics_backlog_events`, `analytics_ingestion_lag_seconds` и `analytics_dead_letter_events` на dashboard `Business Pilot`.
2. Проверить health ClickHouse через `http://localhost:8123/ping` и логи `clickhouse`/`platform-api`.
3. После восстановления убедиться, что backlog уменьшается; worker повторяет временные ошибки автоматически.
4. Для dead-letter события сначала устранить ошибку схемы или доступа, затем вернуть запись в очередь административным SQL с обнулением `dead_lettered_at`, `attempts` и `available_at = now()`.
