.PHONY: build test run-dev-docker stop-dev-docker

BIN ?= yamdc

build:
	go build -o $(BIN) ./cmd/yamdc

test:
	go test ./...

run-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml up --build -d

stop-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml down
