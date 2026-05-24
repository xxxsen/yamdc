.PHONY: build test test-coverage install-golangci-lint lint-go backend-build backend-test backend-check web-install web-lint web-knip web-test web-build web-check ci-check build-image build-web-image run-dev-docker stop-dev-docker plugin-integration-test ruleset-integration-test cross-repo-integration-test devcontainer-bootstrap devcontainer-up devcontainer-rebuild devcontainer-shell devcontainer-check dev-start dev-stop e2e-install e2e-test integration-test

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

# 跨仓库集成测试根目录. 默认指向开发者本地 checkout 出来的 yamdc-plugin /
# yamdc-script 仓库; CI 通过 env 覆盖到 actions/checkout 出来的实际路径.
YAMDC_PLUGIN_REPO ?= /home/sen/work/yamdc-plugin
YAMDC_SCRIPT_REPO ?= /home/sen/work/yamdc-script

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

# plugin-integration-test: 用 yamdc-plugin 仓库里的 case json 跑 plugin-test
# 子命令, 把每个 case 文件单独喂给 `yamdc plugin-test`. 如果仓库内只有一个
# default.json, 就只跑一次; 多 case 时 for 循环串行跑, 任一失败立即退出.
#
# 选 for-loop 而不是 xargs:
#   - 子命令需要写入 stderr 日志; xargs 错误传播跨平台行为不一致.
#   - 任一失败 set -e 立即停, 不会被并发噪声掩盖.
plugin-integration-test:
	@if [ ! -d "$(YAMDC_PLUGIN_REPO)/cases" ]; then \
		echo "[plugin-integration-test] missing $(YAMDC_PLUGIN_REPO)/cases — checkout xxxsen/yamdc-plugin first" >&2; \
		exit 1; \
	fi
	@set -eu; \
	cases=$$(find "$(YAMDC_PLUGIN_REPO)/cases" -maxdepth 1 -type f -name '*.json' | sort); \
	if [ -z "$$cases" ]; then \
		echo "[plugin-integration-test] no *.json cases under $(YAMDC_PLUGIN_REPO)/cases" >&2; \
		exit 1; \
	fi; \
	for case in $$cases; do \
		echo "[plugin-integration-test] $$case"; \
		GOCACHE=$(GOCACHE) go run ./cmd/yamdc plugin-test \
			--plugin="$(YAMDC_PLUGIN_REPO)" \
			--casefile="$$case" \
			--output=json; \
	done

# ruleset-integration-test: 用 yamdc-script 仓库的 cases + ruleset 跑
# ruleset-test 子命令. ruleset 目录是整个 ruleset 文件夹, 不是单一文件.
ruleset-integration-test:
	@if [ ! -d "$(YAMDC_SCRIPT_REPO)/cases" ]; then \
		echo "[ruleset-integration-test] missing $(YAMDC_SCRIPT_REPO)/cases — checkout xxxsen/yamdc-script first" >&2; \
		exit 1; \
	fi
	@if [ ! -d "$(YAMDC_SCRIPT_REPO)/ruleset" ]; then \
		echo "[ruleset-integration-test] missing $(YAMDC_SCRIPT_REPO)/ruleset" >&2; \
		exit 1; \
	fi
	@set -eu; \
	cases=$$(find "$(YAMDC_SCRIPT_REPO)/cases" -maxdepth 1 -type f -name '*.json' | sort); \
	if [ -z "$$cases" ]; then \
		echo "[ruleset-integration-test] no *.json cases under $(YAMDC_SCRIPT_REPO)/cases" >&2; \
		exit 1; \
	fi; \
	for case in $$cases; do \
		echo "[ruleset-integration-test] $$case"; \
		GOCACHE=$(GOCACHE) go run ./cmd/yamdc ruleset-test \
			--ruleset "$(YAMDC_SCRIPT_REPO)/ruleset" \
			--casefile="$$case" \
			--output=json; \
	done

# cross-repo-integration-test: 聚合, 先跑 plugin 再跑 ruleset.
# 任一失败 make 立即退出, 不会用最后一次的成功掩盖之前的失败.
cross-repo-integration-test: plugin-integration-test ruleset-integration-test

# ─────────────────────────────────────────────────────────────────────
# Devcontainer / dev server / integration & E2E test targets.
#
# 设计原则:
# - devcontainer-* 都从宿主机调 `devcontainer` CLI, 让 IDE / Make 行为
#   一致 (docker compose 由 .devcontainer/docker-compose.yml 单一来源).
# - dev-start / dev-stop / integration-test / e2e-test 默认必须在
#   devcontainer 内执行 (脚本入口有 require-devcontainer.sh guard),
#   避免 8080 / 3000 / playwright chromium 污染宿主机.
# - Playwright 装在 stamp 文件背后, 避免反复重新下载 chromium.
# ─────────────────────────────────────────────────────────────────────

DEVCONTAINER_CLI ?= devcontainer
PLAYWRIGHT_BROWSERS_PATH ?= $(CURDIR)/.cache/ms-playwright
PLAYWRIGHT_STAMP := web/node_modules/.playwright-install-stamp

# devcontainer-bootstrap: 容器内首次启动 (postCreateCommand) 跑一次, 把
# 后端 lint, 前端 npm ci, playwright 浏览器都装好.
devcontainer-bootstrap: install-golangci-lint web-install e2e-install

devcontainer-up:
	$(DEVCONTAINER_CLI) up --workspace-folder .

devcontainer-rebuild:
	$(DEVCONTAINER_CLI) up --workspace-folder . --remove-existing-container

devcontainer-shell:
	$(DEVCONTAINER_CLI) exec --workspace-folder . bash

# devcontainer-check: 在容器内跑完整 ci-check (后端 build/test/lint +
# 前端 install/lint/knip/test/build), 用作 CI 与本地的统一闸口.
devcontainer-check:
	$(DEVCONTAINER_CLI) exec --workspace-folder . make ci-check

# Stamp-driven Playwright install: 仅在 web 依赖变化时重装 chromium,
# 避免每次 e2e-test 都重下浏览器. 保留 --with-deps 兼容 CI 上需要装
# linux 系统包的场景.
$(PLAYWRIGHT_STAMP): web/package.json web/package-lock.json
	cd web && npm ci --prefer-offline --no-audit --no-fund
	cd web && PLAYWRIGHT_BROWSERS_PATH=$(PLAYWRIGHT_BROWSERS_PATH) npx playwright install --with-deps chromium
	@touch $(PLAYWRIGHT_STAMP)

e2e-install: $(PLAYWRIGHT_STAMP)

# dev-start / dev-stop: 启动 / 停止 backend (yamdc server) + 前端 dev.
# 进程组管理在 scripts/devcontainer/{start,stop}-dev.sh 里, 见脚本注释.
dev-start:
	scripts/devcontainer/start-dev.sh

dev-stop:
	scripts/devcontainer/stop-dev.sh

# integration-test: 后端 HTTP API 集成 smoke. 启停 backend 都在脚本内
# 用 trap 收尾, Makefile 不重复 stop 避免双重 stop.
integration-test:
	scripts/devcontainer/run-integration-test.sh

# e2e-test: Playwright Desktop Chrome 全套 (10 个 spec). 启 backend +
# frontend 后跑一遍, 任一失败 trap 收尾.
e2e-test: e2e-install
	scripts/devcontainer/run-e2e-test.sh
