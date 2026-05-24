# td/031 实施 Review

Review 时间：2026-05-24

Review 范围：

- 当前工程：`/home/sen/work/yamdc`
- 上轮 review 文档：`/home/sen/work/yamdc/td/031-td030-implementation-review.md`
- 参考工程路径：`/home/sen/work/fire-manager`
- 跨仓库集成对象：`/home/sen/work/yamdc-plugin`、`/home/sen/work/yamdc-script`
- 本轮重点：复核 td/031 中 3 个 P1 与 1 个 P2 是否关闭，并检查 CI / devcontainer / Make 入口是否仍存在阻断。

已执行检查：

- `go test ./internal/web` 通过。
- `cd web && npm run lint` 通过。
- `cd web && npm run test` 通过，65 个测试文件、1054 个用例通过。
- `make -n e2e-test` 第一条命令已变为 `scripts/devcontainer/require-devcontainer.sh`。
- 宿主机直接执行 `make e2e-test`：在 guard 处失败，未执行 `npm ci` / `npx playwright install`。
- `env YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1 scripts/devcontainer/require-devcontainer.sh` 返回 0。
- `env YAMDC_DEVCONTAINER=1 scripts/devcontainer/require-devcontainer.sh` 返回 0。
- `git check-ignore -v web/playwright-report/index.html web/test-results/foo.txt` 均命中 `web/.gitignore`。

未执行：

- 未在 devcontainer 内执行 `make integration-test` / `make e2e-test`。
- 未在 CI runner 内执行真实 workflow。

## 已确认关闭的问题

### Make guard 顺序已修复

涉及文件：

- `Makefile`
- `scripts/devcontainer/require-devcontainer.sh`

确认结果：

- `require-devcontainer` 已加入 `.PHONY`。
- `e2e-test` 的依赖顺序为 `require-devcontainer e2e-install`。
- `$(PLAYWRIGHT_STAMP)` 使用 order-only prerequisite `| require-devcontainer`，避免每次都重跑 `npm ci` / Playwright 下载。
- `devcontainer-bootstrap` 也先依赖 `require-devcontainer`，避免宿主机误执行时污染 `./bin`、`web/node_modules`、`.cache/ms-playwright`。
- 宿主机直接运行 `make e2e-test` 已在 guard 处失败，未进入依赖安装步骤。

本项通过。

### CI 放行变量已补齐

涉及文件：

- `.github/workflows/pr-check.yml`
- `scripts/devcontainer/require-devcontainer.sh`

确认结果：

- `require-devcontainer.sh` 现在接受：
  - `YAMDC_DEVCONTAINER=1`
  - `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1`
- workflow 中 `integration-test` 和 `e2e-test` step 均显式设置了：

```yaml
env:
  YAMDC_ALLOW_NON_DEVCONTAINER_TESTS: "1"
```

- 本地执行 `env YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1 scripts/devcontainer/require-devcontainer.sh` 已验证返回 0。

本项通过。

### Docker Compose devcontainer 环境变量已补齐

涉及文件：

- `.devcontainer/docker-compose.yml`
- `.devcontainer/devcontainer.json`

确认结果：

- `.devcontainer/docker-compose.yml` 的 `services.dev.environment` 已设置：

```yaml
YAMDC_DEVCONTAINER: "1"
```

- `.devcontainer/devcontainer.json` 中保留 `remoteEnv.YAMDC_DEVCONTAINER=${containerEnv:YAMDC_DEVCONTAINER}`，用于 VS Code 终端继承容器环境。
- 本地执行 `env YAMDC_DEVCONTAINER=1 scripts/devcontainer/require-devcontainer.sh` 已验证 guard 能放行。

本项通过。最终仍建议在 devcontainer 内执行一次 `echo $YAMDC_DEVCONTAINER` 与 `make integration-test` 做实机确认。

### Playwright 产物 ignore 已补齐

涉及文件：

- `web/.gitignore`

确认结果：

- `web/.gitignore` 已加入：

```gitignore
/playwright-report
/test-results
```

- `git check-ignore -v web/playwright-report/index.html web/test-results/foo.txt` 均命中。

本项通过。

## P1：e2e CI 缺少 `sqlite3`，会在 seed DB 阶段失败

涉及文件：

- `.github/workflows/pr-check.yml`
- `scripts/devcontainer/run-e2e-test.sh`
- `scripts/devcontainer/seed-e2e-db.sh`
- `.devcontainer/Dockerfile`

问题：

`scripts/devcontainer/run-e2e-test.sh` 的流程是：

1. `seed-e2e-fixtures.sh`
2. `start-dev.sh`
3. `seed-e2e-db.sh`
4. `npx playwright test`

其中 `seed-e2e-db.sh` 明确依赖 `sqlite3`：

```bash
if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "[seed-e2e-db] fatal: 需要 sqlite3 命令; devcontainer 镜像应预装" >&2
  exit 1
fi
```

devcontainer 镜像已在 `.devcontainer/Dockerfile` 安装 `sqlite3`，integration-test CI job 也安装了 `sqlite3`。但 e2e-test CI job 目前只安装：

```bash
sudo apt-get install -y --no-install-recommends jq curl ffmpeg
```

没有安装 `sqlite3`。

触发条件：

1. PR 触发 `.github/workflows/pr-check.yml`。
2. `e2e-test` job 设置 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1` 后执行 `make e2e-test`。
3. `run-e2e-test.sh` 启动 backend 后调用 `seed-e2e-db.sh`。
4. CI runner 上没有 `sqlite3`。
5. `seed-e2e-db.sh` 输出 `fatal: 需要 sqlite3 命令` 并 exit 1。

影响：

- e2e CI 仍然不可用。
- td/031 中“CI 放行变量”问题虽已修复，但真实 e2e job 会在下一阶段失败。

确定修复方案：

- 修改 `.github/workflows/pr-check.yml` 的 e2e runtime deps step：

```yaml
- name: Install runtime deps (jq / curl / ffmpeg / sqlite3)
  run: |
    sudo apt-get update
    sudo apt-get install -y --no-install-recommends jq curl ffmpeg sqlite3
```

- 可选但建议：在 e2e step 前增加一次显式诊断：

```yaml
- name: Check e2e runtime deps
  run: |
    command -v sqlite3
    command -v ffmpeg
```

测试建议：

- 本地静态验证：
  - `rg -n "sqlite3" .github/workflows/pr-check.yml` 应在 e2e job runtime deps 中命中。
- CI 实机验证：
  - e2e job 必须跑过 `seed-e2e-db.sh`，进入 `npx playwright test` 阶段。
- devcontainer 实机验证：
  - 容器内执行 `make e2e-test`。

## 本轮结论

td/031 的本地宿主机 guard、Make 顺序、CI 放行变量、Compose 环境变量、Playwright 产物 ignore 均已落实。

但 e2e CI 缺少 `sqlite3`，会导致 `seed-e2e-db.sh` 在真实 CI 中失败。因此本轮 review 结论为：未通过。修复范围很小，只需补齐 e2e job runtime dependency，并在 devcontainer / CI 中跑一次真实 E2E 验证。
