# Performance baseline — 22 октября 2026

Первый автоматизированный baseline выполнен workflow `Load test`, profile `average`, на GitHub-hosted Ubuntu runner для commit `41a031a6a5e1fcac819cc424cfb64dfd304f8f89`. API, PostgreSQL и demo restaurant работали в одной Docker-сети; поэтому результат подтверждает отсутствие внутренних bottleneck на малой/средней нагрузке, но не заменяет замер на staging host с TLS и production-like ресурсами.

## Нагрузка

- длительность основного окна: 5 минут;
- browse: 5 iterations/s, каждый делает параллельные list restaurants + menu;
- quote: 1 request/s;
- create: 1 request / 4 s, то есть 15 заказов/мин и в пределах per-IP rate limit;
- подготовлено 18 VU, dropped iterations отсутствуют.

## Результат

| Метрика | Значение | Gate |
|---|---:|---:|
| HTTP requests | 3383 | информационно |
| Средняя скорость HTTP | 11.27 req/s | информационно |
| Iterations | 1878 | информационно |
| HTTP failure rate | 0% | < 0.5% |
| Business failure rate | 0% | < 0.5% |
| Все HTTP p95 | 1.46 ms | информационно |
| Quote p95 | 1.49 ms | < 300 ms |
| Create p95 | 5.35 ms | < 500 ms |
| Create max | 14.80 ms | информационно |

Все checks и thresholds прошли. Build стенда занял около 82 секунд, сам k6-профиль — 302 секунды. После теста compose stack и volumes удалены.

## Ограничения и следующий замер

Следующий обязательный baseline выполняется после фактического staging deployment: через публичный TLS ingress, с фиксированными CPU/RAM limits, PostgreSQL pool/locks, host CPU/RAM и outbox lag. Затем запускаются `peak`, `stress` и `degraded`; результаты сравниваются только при одинаковых ресурсах и версии приложения.

JSON artifact теперь формируется пользовательским `handleSummary` и содержит только агрегированные метрики/thresholds. Setup data, access tokens, адреса и идентификаторы заказов в artifact не включаются.
