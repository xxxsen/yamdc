.PHONY: build test build-image build-web-image run-dev-docker stop-dev-docker

BIN ?= yamdc
BACKEND_IMAGE ?= xxxsen/yamdc:latest
WEB_IMAGE ?= xxxsen/yamdc-web:latest

build:
	go build -o $(BIN) ./cmd/yamdc

test:
	go test ./...

build-image:
	docker build -t $(BACKEND_IMAGE) .

build-web-image:
	docker build -t $(WEB_IMAGE) -f web/Dockerfile ./web

run-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml up --build -d

stop-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml down
