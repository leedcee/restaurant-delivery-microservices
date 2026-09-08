.PHONY: build test test-e2e lint fmt diagrams up down logs

build:
	go build ./...

test:
	go test ./...

test-e2e:
	go test -count=1 -tags=e2e ./tests/e2e

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
