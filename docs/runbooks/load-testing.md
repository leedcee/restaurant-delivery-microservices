# Runbook: нагрузочное тестирование

## Профили

Единый сценарий `tests/load/platform.js` содержит четыре профиля:

| Профиль | Назначение | Нагрузка |
|---|---|---|
| `smoke` | проверка сценария и release gate | 5 последовательных browse → quote → create |
| `average` | базовый capacity baseline | 5 browse/s, 1 quote/s, 15 create/min в течение 5 минут |
| `peak` | ожидаемый краткий пик | рост browse до 50/s, quote/create остаются внутри per-IP rate limit |
| `stress` | поиск точки насыщения read path | рост browse до 200/s при контролируемом write path |
| `degraded` | оформление при недоступном ресторане | demo restaurant остановлен, заказ должен сохраниться, outbox — расти без 5xx |

Create и quote намеренно не превышают встроенный rate limit одного IP. Для распределённого теста через staging ingress генераторы должны иметь разные исходящие IP либо должен быть реализован доверенный proxy-aware limiter; отключать защиту в production нельзя.

## Release thresholds

- `http_req_failed < 0.5%`;
- business checks failed `< 0.5%`;
- quote p95 `< 300 ms`;
- create p95 `< 500 ms`.

Smoke блокирует обычный CI. Более длинные профили запускаются вручную workflow `Load test` или по расписанию для `average`. Результат k6 сохраняется как санитизированный JSON summary artifact без setup data, токенов и бизнес-идентификаторов.

Первый CI baseline зафиксирован в [отчёте от 22 октября 2026](../performance-baseline-2026-10-22.md).

## Локальный запуск

```bash
docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml up -d --build --wait
docker run --rm --network restaurant-delivery-e2e_default \
  -v "$PWD/tests/load:/scripts:ro" grafana/k6:2.3.0 \
  run -e BASE_URL=http://platform-api:8080 -e PROFILE=smoke /scripts/platform.js
```

Для `degraded` перед запуском остановить `demo-restaurant`. После теста проверить `outbox_backlog_events`, возраст oldest event, отсутствие dead-letter до исчерпания retry и успешную доставку после восстановления ресторана.

## Фиксация baseline

В отчёте запуска сохранить commit SHA, профиль, ресурсы runner/host, p50/p95/p99, throughput, error rate, dropped iterations, PostgreSQL pool/locks, outbox backlog и максимальный CPU/RAM. Результаты с разных машин нельзя сравнивать без указания ресурсов.
