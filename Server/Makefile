.PHONY: run test integration build migrate-up lint deps docker-up docker-down

deps:
	go mod tidy

run:
	go run ./cmd/server

test:
	go test ./... -race -count=1

integration:
	go test -tags=integration ./test/integration/... -race -count=1 -timeout 5m

build:
	go build -o bin/server ./cmd/server

migrate-up:
	@echo "Applying migrations/*.up.sql to PostgreSQL..."
	@for f in migrations/*.up.sql; do \
		echo "==> $$f"; \
		psql -h 127.0.0.1 -p 5432 -U blog -d blog -f $$f || exit 1; \
	done

lint:
	golangci-lint run ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
