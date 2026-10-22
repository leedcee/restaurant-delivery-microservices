# Threat model пилотного контура

Обновлено: 22 октября 2026 года.

## Область анализа

В модель входят browser frontend, platform API, PostgreSQL, demo restaurant, партнёрский API, outbox-доставка, SSE, ClickHouse и observability-контур. Реальные платёжный и курьерский провайдеры пока не входят в реализованный контур; требования к ним зафиксированы заранее.

Защищаемые активы: учётные записи и сессии, персональные данные и адреса, состав и стоимость заказа, остатки, партнёрские ключи, журнал событий, резервные копии и эксплуатационные секреты.

## Границы доверия

1. Браузер и публичный интернет не являются доверенными.
2. Внешний reverse proxy завершает TLS, но API повторно проверяет origin, authentication, authorization и входные данные.
3. Партнёрский API является отдельной границей доверия: ресторан не может читать или изменять данные другого ресторана.
4. PostgreSQL является источником истины; ClickHouse, Grafana, Loki и Tempo не участвуют в авторизации и принятии транзакционных решений.
5. Webhook от будущих провайдеров считается недоверенным до проверки подписи, срока сообщения и дедупликации через inbox.

## Основные угрозы и меры

| ID | Угроза | Риск | Реализованные меры | Остаточная задача |
|---|---|---:|---|---|
| TM-01 | Кража или повтор refresh token | высокий | HttpOnly/SameSite cookie, rotation, reuse detection, family revoke, logout | алерт на массовый reuse и runbook |
| TM-02 | Подбор пароля и истощение API | высокий | bcrypt, rate limits на auth/quote/create | распределённый limiter перед горизонтальным масштабированием |
| TM-03 | Запуск production с dev-секретом или HTTP cookie | критический | fail-closed validation для staging/production | secret manager и автоматическая ротация |
| TM-04 | Подмена цены, состава или ресторана | критический | server-side quote, транзакционная повторная проверка меню и остатков | срок действия quote и подпись версии каталога |
| TM-05 | Повтор запроса создаёт второй заказ | высокий | idempotency key и сохранённый response | документировать контракт каждого mutating endpoint |
| TM-06 | Межтенантный доступ ресторана | критический | ключ привязан к restaurant ID, запросы ограничены tenant ID | employee RBAC, key ID/expiry/rotation и audit trail |
| TM-07 | Потеря или повтор интеграционного события | высокий | transactional outbox, event ID, retry/backoff, dead letter, replay, receiver deduplication | inbox для внешних webhook |
| TM-08 | Утечка секрета или PII через telemetry | высокий | структурированные логи, запрет PII в labels, security scan | автоматические redaction tests и полный data inventory |
| TM-09 | Supply-chain уязвимость | высокий | npm/Trivy source, secret and image gates in CI | SBOM, digest pinning и подпись OCI images |
| TM-10 | XSS/clickjacking/MIME confusion | высокий | CSP, frame denial, nosniff, referrer и permissions policies | DAST в staging |
| TM-11 | SSE зависает или раскрывает чужие события | высокий | auth/tenant checks, Last-Event-ID, heartbeat, nginx buffering disabled | connection limits и SSE-specific metrics |
| TM-12 | Потеря или шифрование PostgreSQL | критический | транзакции и readiness | encrypted backup, restore drill, подтверждённые RPO/RTO |
| TM-13 | Поддельный payment/delivery webhook | критический | провайдеры ещё не подключены | HMAC/mTLS verification, timestamp window, inbox и reconciliation до подключения денег |

## Обязательные security gates

- PR не проходит при high/critical dependency или image vulnerability, найденном секрете либо high/critical misconfiguration.
- Staging и production не запускаются с коротким/default JWT secret, небезопасной session cookie или HTTP CORS origin.
- Публичный ingress принимает только TLS; PostgreSQL, ClickHouse и observability backends не публикуются наружу.
- Компрометация session или partner key имеет документированный путь отзыва без изменения данных вручную.
- До реальных денег обязательны webhook inbox, проверка подписи, idempotency, refund и reconciliation.

## Когда пересматривать

Модель обновляется при добавлении внешнего провайдера, новой роли, нового публичного endpoint, изменении границы сети или выделении модуля в отдельный сервис.
