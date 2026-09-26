.PHONY: help fmt test race vet build run wordlist compose-up compose-down compose-reset

help:
	@printf '%s\n' \
		'make fmt          Format Go source' \
		'make test         Run package tests' \
		'make race         Run tests with the race detector' \
		'make vet          Run go vet' \
		'make build        Build the API binary' \
		'make run          Start the API locally' \
		'make wordlist     Regenerate the embedded profanity word lists' \
		'make compose-up   Start PostgreSQL and the API with Docker Compose' \
		'make compose-down Stop Docker Compose services' \
		'make compose-reset Stop services and remove the development database'

fmt:
	gofmt -w cmd internal

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -o bin/threadman-api ./cmd/server

run:
	go run ./cmd/server

# Regenerates internal/security/wordlist from the upstream bad-words lists.
wordlist:
	go run ./cmd/wordlist
	gofmt -w internal/security/wordlist

compose-up:
	docker compose up --build

compose-down:
	docker compose down

compose-reset:
	docker compose down -v
