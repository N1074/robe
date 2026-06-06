.PHONY: run google-auth build fmt test vet check health db-up db-down db-logs db-psql

run:
	go run ./cmd/robe-server

google-auth:
	go run ./cmd/robe-google-auth

build:
	mkdir -p bin
	go build -o bin/robe-server ./cmd/robe-server

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

vet:
	go vet ./...

check: fmt test vet

health:
	curl -fsS http://localhost:8080/health

db-up:
	docker compose up -d robe-db

db-down:
	docker compose down

db-logs:
	docker compose logs -f robe-db

db-psql:
	docker compose exec robe-db psql -U robe -d robe
