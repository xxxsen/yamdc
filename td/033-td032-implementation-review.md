# td/032 实施 Review

Review 时间：2026-05-25

Review 范围：

- 当前工程：`/home/sen/work/yamdc`
- 上轮 review 文档：`/home/sen/work/yamdc/td/032-td031-implementation-review.md`
- 参考工程路径：`/home/sen/work/fire-manager`
- 跨仓库集成对象：`/home/sen/work/yamdc-plugin`、`/home/sen/work/yamdc-script`
- 本轮重点：复核 td/032 中 e2e CI 缺少 `sqlite3` 的 P1 是否关闭，并确认没有引入新的 CI / devcontainer / Make 入口问题。

已执行检查：

- `rg -n "sqlite3|Check e2e runtime deps|Install runtime deps|YAMDC_ALLOW_NON_DEVCONTAINER_TESTS" .github/workflows/pr-check.yml`：确认 integration / e2e 两个 job 都能看到对应依赖与 CI 放行变量。
- `env YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1 make -n e2e-test`：确认 CI 放行后执行顺序为 guard -> `npm ci` -> Playwright install -> e2e 脚本。
- `make -n e2e-test`：确认第一条命令仍是 `scripts/devcontainer/require-devcontainer.sh`。
- `go test ./internal/web` 通过。
- `cd web && npm run lint` 通过。
- `cd web && npm run test` 通过，65 个测试文件、1054 个用例通过。
- `git diff --check` 通过。

未执行：

- 未在 GitHub Actions runner 内执行真实 workflow。
- 未在 devcontainer 内执行 `make e2e-test`。

## 已确认关闭的问题

### e2e CI 缺少 `sqlite3` 已修复

涉及文件：

- `.github/workflows/pr-check.yml`

确认结果：

`e2e-test` job 的 runtime deps step 已从：

```yaml
sudo apt-get install -y --no-install-recommends jq curl ffmpeg
```

修正为：

```yaml
sudo apt-get install -y --no-install-recommends jq curl ffmpeg sqlite3
```

同时新增了显式依赖诊断：

```yaml
command -v sqlite3
command -v ffmpeg
command -v jq
command -v curl
```

这能确保 `scripts/devcontainer/run-e2e-test.sh` 调用 `scripts/devcontainer/seed-e2e-db.sh` 前，CI runner 已具备 `sqlite3` 命令，不会再在 seed DB 阶段因缺命令直接失败。

本项通过。

### CI 放行变量仍保持正确

涉及文件：

- `.github/workflows/pr-check.yml`
- `scripts/devcontainer/require-devcontainer.sh`
- `Makefile`

确认结果：

- `integration-test` step 设置了 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS: "1"`。
- `e2e-test` step 设置了 `YAMDC_ALLOW_NON_DEVCONTAINER_TESTS: "1"`。
- `env YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1 make -n e2e-test` 能通过 Make guard 并展示后续 e2e 安装 / 启动命令。
- 未设置放行变量时，`make -n e2e-test` 的第一条命令仍是 `scripts/devcontainer/require-devcontainer.sh`，防止宿主机实际执行时先污染依赖。

本项通过。

## Review 结果

本轮未发现新的 P0 / P1 / P2 问题。

剩余验证项：

- 需要在真实 GitHub Actions 中确认 `e2e-test` job 能跑过 `seed-e2e-db.sh` 并进入 Playwright 阶段。
- 需要在 devcontainer 内执行一次 `make e2e-test`，确认 fixture seed、backend/frontend 启动、Playwright 全链路均通过。

## 本轮结论

td/032 中提出的 P1 已按确定方案关闭。本轮 review 通过。
