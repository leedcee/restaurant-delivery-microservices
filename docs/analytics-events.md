# Аналитические события

Аналитика отделена от технической телеметрии. Prometheus хранит показатели работы сервисов, а ClickHouse получает обезличенные копии доменных событий через отдельный PostgreSQL outbox. Ошибка аналитического контура не откатывает заказ и не влияет на пользовательский API.

## Общий envelope

| Поле | Тип | Назначение |
|---|---|---|
| `event_id` | UUID | Идентификатор и ключ дедупликации события |
| `event_version` | UInt16 | Версия контракта |
| `event_name` | String | Стабильное имя события |
| `occurred_at` | DateTime64 UTC | Время доменного события |
| `ingested_at` | DateTime64 UTC | Время записи в ClickHouse |
| `trace_id` | String | Связь с технической трассировкой, если она была активна |
| `properties` | JSON-строка | Версионированные обезличенные свойства |

## События версии 1

`order_created` содержит `order_key`, `restaurant_id`, `total_minor`, `currency` и `status`.

`order_status_changed` содержит `order_key`, `restaurant_id`, `order_created_at`, `from_status`, `to_status`, `source` и контролируемый `reason_code`.

`order_key` — SHA-256 от внутреннего UUID заказа. Исходный `order_id`, `user_id`, email, адрес доставки, позиции заказа и свободный текст причины не экспортируются.

## Хранение и повторная доставка

- PostgreSQL `analytics_events` является журналом доставки с retry и dead-letter состоянием.
- ClickHouse использует `ReplacingMergeTree(ingested_at)` с сортировкой по `event_id`; dashboard-запросы используют `FINAL` для дедупликации повторной доставки.
- Данные разбиваются на месячные партиции и удаляются по TTL через 365 дней.
- Backlog, ingestion lag и dead-letter count публикуются в Prometheus.
- Изменение набора или смысла properties требует увеличения `event_version`.
