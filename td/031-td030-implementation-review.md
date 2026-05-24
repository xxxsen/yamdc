# td/030 实施 Review

Review 时间：2026-05-24

Review 范围：

- 当前工程：`/home/sen/work/yamdc`
- 上轮 review 文档：`/home/sen/work/yamdc/td/030-td029-implementation-review.md`
- 参考工程路径：`/home/sen/work/fire-manager`
- 跨仓库集成对象：`/home/sen/work/yamdc-plugin`、`/home/sen/work/yamdc-script`
- 本轮重点：复核 td/030 中 P1/P2/P3 是否被正确关闭，并检查本次修复是否引入新的工程化 / FE / BE / UIUX 问题。

已执行检查：

- `go test ./internal/web` 通过。
- `cd web && npm run lint` 通过。
- `cd web && npm run test` 通过，65 个测试文件、1054 个用例通过。
- 宿主机直接执行 `scripts/devcontainer/run-e2e-test.sh`：被 `require-devcontainer.sh` 拦截，返回 code 1，未启动 8080 / 3000。
- 宿主机直接执行 `scripts/devcontainer/run-integration-test.sh`：被 `require-devcontainer.sh` 拦截，返回 code 1，未启动 8080。
- `make -n e2e-test`：发现会先执行 `npm ci` / `playwright install`，再执行 `scripts/devcontainer/run-e2e-test.sh`。

未执行：

- 未在宿主机运行 `make e2e-test`，因为当前 Make 依赖顺序会先污染宿主机依赖 / Playwright 缓存，再进入 devcontainer guard。
- 未在 devcontainer 内运行 `make e2e-test`，当前 review 仅能在宿主机环境验证 guard 与静态实现。

## 已确认关闭的问题

### CORS 默认 wildcard 行为已按约束修复

涉及文件：

- `internal/web/router.go`
- `internal/web/router_test.go`

确认结果：

- `YAMDC_ALLOWED_ORIGINS` 为空或 trim 后为空时，`loadAllowedOrigins()` 返回空切片，用于表示 wildcard 模式。
- wildcard 模式下统一写 `Access-Control-Allow-Origin: *`。
- wildcard 模式下 `OPTIONS` 返回 204，并且不拦截任意 Origin 的状态变更请求。
- 显式配置 `YAMDC_ALLOWED_ORIGINS` 后才启用白名单模式。
- 单测覆盖了 wildcard GET / POST / OPTIONS、白名单命中、白名单拒绝、空 Origin 等路径。

本项通过。

### E2E API 契约中的数组返回结构已修复

涉及文件：

- `web/e2e/04-library.spec.ts`
- `web/e2e/06-media-library.spec.ts`
- `web/e2e/07-media-library-sync.spec.ts`
- `web/e2e/00-api-helper-contract.spec.ts`

确认结果：

- `GET /api/library` 已按 `LibraryItem[]` 断言，不再使用错误的 `data.items`。
- `GET /api/media-library` 已按 `MediaLibraryItem[]` 断言，不再使用错误的 `data.items`。
- `GET /api/media-library/sync/logs` 已按 `SyncLogEntry[] | null` 断言，不再使用错误的 `env.data?.items`。
- 新增 `00-api-helper-contract.spec.ts`，明确验证 `apiGet<T>()` 返回 envelope 的 `data` 本身。

本项通过。

### 上传 32 MiB 边界已修复

涉及文件：

- `internal/web/jobs_routes.go`
- `internal/web/jobs_routes_test.go`

确认结果：

- 文件大小上限仍为 `maxUploadImageBytes = 32 << 20`。
- multipart request body 上限放宽到 `maxUploadImageBytes + maxUploadMultipartOverheadBytes`。
- 文件本身仍通过 `header.Size` 与 `len(data)` 做严格 32 MiB 校验。
- Go 单测覆盖了：
  - 文件刚好 32 MiB 成功；
  - 文件 32 MiB + 1 返回 413；
  - multipart header 较大但文件未超过 32 MiB 时成功。

本项通过。

### E2E 覆盖已明显扩展

涉及文件：

- `web/e2e/01-processing.spec.ts`
- `web/e2e/02-review.spec.ts`
- `web/e2e/03-review-assets.spec.ts`
- `web/e2e/04-library.spec.ts`
- `web/e2e/06-media-library.spec.ts`
- `web/e2e/09-plugin-editor.spec.ts`
- `scripts/devcontainer/seed-e2e-fixtures.sh`
- `scripts/devcontainer/seed-e2e-db.sh`

确认结果：

- 新增 scan / save / media library / review DB fixture seed。
- Processing 覆盖 scan -> jobs 列表 -> logs / 异常路径。
- Review 覆盖 reviewing fixture、详情打开、标题自动保存落库、异常协议稳定性。
- Review assets 覆盖 UI 上传、前端超限拦截、poster crop。
- Library 覆盖 NFO PATCH 持久化、variant 切换、poster/cover 上传、fanart 删除。
- Media library 覆盖 sync、日志弹窗、详情弹窗、排序请求。
- Plugin editor 覆盖 XPath inspector 右键菜单与 Copy XPath。

本项从覆盖设计看已补齐。实际 E2E 是否全绿仍必须在 devcontainer 内运行 `make e2e-test` 确认。

### 工程注释中的临时编号和外部模板语义已清理

涉及文件：

- `Makefile`
- `.devcontainer/docker-compose.yml`
- `AGENTS.md`

确认结果：

- `Makefile` 中不再出现 `029-fe-be-uiux-review`。
- `.devcontainer/docker-compose.yml` 中不再描述“对齐 fire-manager 模板”。
- `AGENTS.md` 仍聚焦当前工程本身，未引入外部工程路径或后续工程化目标。

本项通过。

## P1：CI integration-test / e2e-test 会被 devcontainer guard 直接打断

涉及文件：

- `.github/workflows/pr-check.yml`
- `scripts/devcontainer/require-devcontainer.sh`
- `scripts/devcontainer/run-integration-test.sh`
- `scripts/devcontainer/run-e2e-test.sh`
- `Makefile`

问题：

当前 workflow 在 `ubuntu-latest` runner 上直接执行：

- `make integration-test`
- `make e2e-test`

但 `run-integration-test.sh` 和 `run-e2e-test.sh` 开头都调用了 `require-devcontainer.sh`，该 guard 只接受 `YAMDC_DEVCONTAINER=1`。GitHub Actions job 没有设置该变量，也没有进入 devcontainer，因此两个新增 CI job 会失败。

触发条件：

1. 提交 PR 触发 `.github/workflows/pr-check.yml`。
2. `integration-test` job 执行 `make integration-test`。
3. `scripts/devcontainer/run-integration-test.sh` 调用 `require-devcontainer.sh`。
4. 因 `YAMDC_DEVCONTAINER` 未设置，脚本直接 exit 1。

`e2e-test` job 同理，但还有一个额外问题：它会先执行 `e2e-install`，再进入 guard。见下一条 P1。

影响：

- PR CI 必定红。
- td/030 要求把 integration / E2E 作为质量闸口；当前实现会让闸口不可用。

确定修复方案：

- `require-devcontainer.sh` 支持两个明确模式：
  - `YAMDC_DEVCONTAINER=1`：本地 devcontainer 内允许执行。
  - `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1`：仅给 CI runner 使用，表示当前非 devcontainer 环境是一次性隔离环境，可以启动 8080 / 3000。
- `.github/workflows/pr-check.yml` 的 `integration-test` 与 `e2e-test` 运行步骤显式设置：
  - `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1`
- guard 文案必须区分：
  - 本地宿主机未设置允许变量时拒绝；
  - CI 显式允许变量存在时放行。

测试建议：

- 宿主机执行 `scripts/devcontainer/run-integration-test.sh`：仍必须失败。
- 宿主机执行 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1 scripts/devcontainer/run-integration-test.sh`：允许启动 backend，用于模拟 CI runner。
- GitHub Actions 中 `integration-test` / `e2e-test` job 必须至少跑到真实测试阶段，不能在 guard 处退出。

## P1：Docker Compose devcontainer 内不一定能拿到 `YAMDC_DEVCONTAINER=1`

涉及文件：

- `.devcontainer/devcontainer.json`
- `.devcontainer/docker-compose.yml`
- `scripts/devcontainer/require-devcontainer.sh`

问题：

当前只在 `.devcontainer/devcontainer.json` 的 `containerEnv` 中设置了：

```json
"YAMDC_DEVCONTAINER": "1"
```

但本项目 devcontainer 使用的是 `dockerComposeFile`。VS Code Dev Containers 官方文档对 Docker Compose 场景的建议是：容器级环境变量应该写到 Compose 的 `environment`，`devcontainer.json` 中只支持面向 VS Code 及相关子进程的 `remoteEnv`。参考：<https://code.visualstudio.com/remote/advancedcontainers/environment-variables>

因此，当前 guard 依赖的 `YAMDC_DEVCONTAINER=1` 没有被写入 `.devcontainer/docker-compose.yml` 的 service environment。用 Docker Compose / devcontainer CLI 进入容器后，`make e2e-test`、`make integration-test` 存在被 guard 误拒的风险。

触发条件：

1. 通过 `.devcontainer/docker-compose.yml` 启动 devcontainer。
2. 在容器内执行 `make integration-test` 或 `make e2e-test`。
3. shell 环境没有 `YAMDC_DEVCONTAINER=1`。
4. `require-devcontainer.sh` 误判为宿主机，直接 exit 1。

影响：

- 本地 devcontainer 内的 integration / E2E 入口可能不可用。
- 这会和“必须在 devcontainer 内执行”的目标直接冲突：进入了 devcontainer 也跑不起来。

确定修复方案：

- 将 `YAMDC_DEVCONTAINER: "1"` 添加到 `.devcontainer/docker-compose.yml` 的 `services.dev.environment`。
- `devcontainer.json` 中可保留 `remoteEnv`，但不应依赖 `containerEnv` 作为 Docker Compose 场景的唯一来源。
- 最稳妥配置：
  - `.devcontainer/docker-compose.yml`：设置 `YAMDC_DEVCONTAINER: "1"`，保证所有容器进程可见。
  - `.devcontainer/devcontainer.json`：如需 VS Code 终端额外继承，可设置 `remoteEnv.YAMDC_DEVCONTAINER=${containerEnv:YAMDC_DEVCONTAINER}`。

测试建议：

- `make devcontainer-up`
- `make devcontainer-shell`
- 容器内执行 `echo $YAMDC_DEVCONTAINER`，必须输出 `1`。
- 容器内执行 `scripts/devcontainer/require-devcontainer.sh`，必须返回 0。
- 容器内执行 `make integration-test`，必须通过 guard 并启动 backend。

## P1：`make e2e-test` 在 guard 前会先安装依赖 / Playwright，仍会污染宿主机

涉及文件：

- `Makefile`
- `scripts/devcontainer/require-devcontainer.sh`
- `scripts/devcontainer/run-e2e-test.sh`

问题：

当前 Makefile：

```make
e2e-test: e2e-install
	scripts/devcontainer/run-e2e-test.sh
```

`make -n e2e-test` 展示实际顺序为：

```text
cd web && npm ci --prefer-offline --no-audit --no-fund
cd web && PLAYWRIGHT_BROWSERS_PATH=/home/sen/work/yamdc/.cache/ms-playwright npx playwright install --with-deps chromium
touch web/node_modules/.playwright-install-stamp
scripts/devcontainer/run-e2e-test.sh
```

这说明宿主机直接执行 `make e2e-test` 时，如果 stamp 不存在，会先运行 `npm ci` 和 `npx playwright install --with-deps chromium`，然后才进入 `run-e2e-test.sh` 的 devcontainer guard。

触发条件：

1. 宿主机 checkout 一个干净仓库，`web/node_modules/.playwright-install-stamp` 不存在。
2. 用户误执行 `make e2e-test`。
3. Make 先执行 `e2e-install`：
   - 写宿主机 `web/node_modules`；
   - 写宿主机 `.cache/ms-playwright`；
   - `--with-deps` 还可能尝试安装系统依赖。
4. 之后才执行 `run-e2e-test.sh` 并被 guard 拦截。

影响：

- td/030 明确要求宿主机执行 `make e2e-test` 必须立即失败，且不得污染宿主机。
- 当前实现仍会在 guard 前污染宿主机依赖和 Playwright 缓存。

确定修复方案：

- 新增 Make target：

```make
.PHONY: require-devcontainer
require-devcontainer:
	scripts/devcontainer/require-devcontainer.sh
```

- 将所有会安装依赖或启动进程的目标放到 guard 后：

```make
$(PLAYWRIGHT_STAMP): require-devcontainer web/package.json web/package-lock.json
	cd web && npm ci --prefer-offline --no-audit --no-fund
	cd web && PLAYWRIGHT_BROWSERS_PATH=$(PLAYWRIGHT_BROWSERS_PATH) npx playwright install --with-deps chromium
	@touch $(PLAYWRIGHT_STAMP)

e2e-install: require-devcontainer $(PLAYWRIGHT_STAMP)

e2e-test: require-devcontainer e2e-install
	scripts/devcontainer/run-e2e-test.sh
```

- `devcontainer-bootstrap` 也必须依赖 `require-devcontainer`，避免用户在宿主机误执行 `make devcontainer-bootstrap`。
- 配合上一条 P1，CI runner 使用 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1` 显式放行。

测试建议：

- 删除 stamp 后在宿主机执行 `make e2e-test`：必须在任何 `npm ci` / `npx playwright install` 前失败。
- 宿主机执行 `make -n e2e-test`：第一条命令必须是 `scripts/devcontainer/require-devcontainer.sh`。
- devcontainer 内执行 `make e2e-install`：必须正常安装 Playwright。
- CI 中设置 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1` 后执行 `make e2e-test`：必须允许安装并进入 Playwright。

## P2：本轮产生的 Playwright HTML report 不应提交

涉及路径：

- `web/playwright-report/index.html`

问题：

当前工作区存在未跟踪的 `web/playwright-report/`。这是 Playwright 运行产物，不是源码、测试或 review 文档。

触发条件：

1. 本地或 devcontainer 内运行 Playwright。
2. Playwright 生成 `web/playwright-report/index.html`。
3. 若后续使用 `git add -A`，该报告会被误提交。

确定修复方案：

- 删除当前未跟踪的 `web/playwright-report/`。
- 在 `.gitignore` 中补齐：

```gitignore
web/playwright-report/
web/test-results/
```

测试建议：

- 执行 `git check-ignore -v web/playwright-report/index.html`，必须被 `.gitignore` 命中。
- 执行 `git status --short`，不应出现 `web/playwright-report/`。

## 本轮结论

td/030 中的业务修复大部分已经落实，但 devcontainer / CI 入口仍存在 P1 级工程化错误：

- CI integration / E2E 会被 guard 打断。
- Docker Compose devcontainer 内的 `YAMDC_DEVCONTAINER` 来源不可靠。
- `make e2e-test` 会在 guard 前污染宿主机依赖和 Playwright 缓存。

因此本轮 review 结论为：未通过。后续必须先修复上述 P1，再重新运行：

- `go test ./internal/web`
- `cd web && npm run lint`
- `cd web && npm run test`
- 宿主机 guard 验证
- devcontainer 内 `make integration-test`
- devcontainer 内 `make e2e-test`
