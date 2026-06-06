.PHONY: run google-auth build fmt test vet check health

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
