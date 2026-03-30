.PHONY: build test backend-build backend-test backend-check web-install web-lint web-build web-check ci-check build-image build-web-image run-dev-docker stop-dev-docker

BIN ?= yamdc
BACKEND_IMAGE ?= xxxsen/yamdc:latest
WEB_IMAGE ?= xxxsen/yamdc-web:latest
GO_TEST_PKGS ?= ./cmd/... ./internal/...

build:
	go build -o $(BIN) ./cmd/yamdc

test:
	go test $(GO_TEST_PKGS)

backend-build: build

backend-test: test

backend-check: backend-build backend-test

web-install:
	cd web && npm ci

web-lint:
	cd web && npm run lint

web-build:
	cd web && npm run build

web-check: web-install web-lint web-build

ci-check: backend-check web-check

build-image:
	docker build -t $(BACKEND_IMAGE) .

build-web-image:
	docker build -t $(WEB_IMAGE) -f web/Dockerfile ./web

run-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml up --build -d

stop-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml down
