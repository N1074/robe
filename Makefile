.PHONY: run fmt test vet check

run:
	go run ./cmd/robe-server

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

vet:
	go vet ./...

check: fmt test vet
