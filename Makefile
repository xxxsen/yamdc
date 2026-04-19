.PHONY: build test test-coverage install-golangci-lint lint-go backend-build backend-test backend-check web-install web-lint web-knip web-test web-build web-check ci-check build-image build-web-image run-dev-docker stop-dev-docker

BIN ?= yamdc
BACKEND_IMAGE ?= xxxsen/yamdc:latest
WEB_IMAGE ?= xxxsen/yamdc-web:latest
GO_TEST_PKGS ?= ./cmd/... ./internal/...
GOBIN ?= $(CURDIR)/bin
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.11.4
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint
GO_COVERAGE_THRESHOLD ?= 95

build:
	GOCACHE=$(GOCACHE) go build -o $(BIN) ./cmd/yamdc

test:
	GOCACHE=$(GOCACHE) go test -race $(GO_TEST_PKGS)

test-coverage:
	GOCACHE=$(GOCACHE) bash scripts/check-go-coverage.sh $(GO_COVERAGE_THRESHOLD)

install-golangci-lint:
	GOBIN=$(GOBIN) GOCACHE=$(GOCACHE) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint-go:
	GOCACHE=$(GOCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(GOLANGCI_LINT) run --config .golangci.yml ./cmd/... ./internal/...

backend-build: build

backend-test: test-coverage

backend-check: backend-build backend-test lint-go

web-install:
	cd web && npm ci

web-lint:
	cd web && npm run lint

# web-knip: 扫死代码 / 僵尸依赖 / 未解析 import. eslint 只看单文件,
# knip 看跨文件 export 是否被 import, 两者互补。详见 web/knip.json。
web-knip:
	cd web && npm run knip

web-test:
	cd web && npm run test:coverage

web-build:
	cd web && npm run build

web-check: web-install web-lint web-knip web-test web-build

ci-check: backend-check web-check

build-image:
	docker build -t $(BACKEND_IMAGE) .

build-web-image:
	docker build -t $(WEB_IMAGE) -f web/Dockerfile ./web

run-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml up --build -d

stop-dev-docker:
	UID=$$(id -u) GID=$$(id -g) docker compose -f docker/docker-compose.yml down
