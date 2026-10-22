.PHONY: build test test-e2e backup-restore-drill lint fmt diagrams up down logs observability-up observability-down

build:
	go build ./...

test:
	go test ./...

test-e2e:
	@set -e; trap 'docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml down -v' EXIT; \
		docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml up -d --build --wait; \
		E2E_BASE_URL=http://127.0.0.1:18080 \
		E2E_DATABASE_URL='postgres://restaurant_test:restaurant_test@127.0.0.1:55432/restaurant_delivery_test?sslmode=disable' \
		go test -count=1 -tags=e2e ./tests/e2e; \
		bash scripts/e2e-resilience.sh; \
		docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml run --rm migrations \
			-path=/migrations -database='postgres://restaurant_test:restaurant_test@postgres:5432/restaurant_delivery_test?sslmode=disable' down 1; \
		docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml run --rm migrations \
			-path=/migrations -database='postgres://restaurant_test:restaurant_test@postgres:5432/restaurant_delivery_test?sslmode=disable' up; \
		docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml run --rm partner-fixtures; \
		curl --fail http://127.0.0.1:18080/api/v1/restaurants

backup-restore-drill:
	docker compose -p restaurant-delivery-e2e -f docker-compose.e2e.yml run --rm --build backup-restore-drill

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

diagrams:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/render-diagrams.ps1

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

observability-up:
	docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability up --build -d

observability-down:
	docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability down
