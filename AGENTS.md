# AGENTS.md

## 项目概述

yamdc — 影片元数据抓取、补全与媒体库管理工具。Go 后端 + Next.js 前端，前后端分离。

- `yamdc run` — 单次扫描抓取
- `yamdc server` — HTTP API 服务 + WebUI

---

## 仓库结构

```
cmd/yamdc/              CLI 入口（cobra）
internal/               Go 后端全部业务逻辑
  config/               配置结构体与解析（hujson JSON）
  capture/              核心抓取流程编排
  searcher/             搜索器 + YAML 声明式插件引擎
  processor/handler/    后处理 handler 链（注册式工厂）
  model/                核心数据模型（MovieMeta）
  web/                  HTTP API（gin）
  repository/           SQLite 持久层
  ...                   其余子包见目录名
web/                    Next.js 前端
  src/app/              页面路由
  src/components/       React 组件（*-shell.tsx）
  src/lib/api.ts        API 客户端（所有后端交互集中于此）
```

---

## 构建与测试

### Make 命令一览

```bash
# ── 后端 ──
make build                # go build -o yamdc ./cmd/yamdc
make test                 # go test ./cmd/... ./internal/...
make install-golangci-lint  # 安装 golangci-lint 到 ./bin/
make lint-go              # golangci-lint run（需先 install）
make backend-check        # build + test + lint-go（后端完整检查）

# ── 前端 ──
make web-install          # cd web && npm ci
make web-lint             # cd web && npm run lint（eslint）
make web-build            # cd web && npm run build（Next.js 生产构建）
make web-check            # install + lint + build（前端完整检查）

# ── 全量 ──
make ci-check             # backend-check + web-check（与 CI 一致）
```

**提交前务必确保 `make ci-check` 通过。**

### Go 测试

- 测试范围：`./cmd/... ./internal/...`
- 框架：标准 `testing` + `github.com/stretchr/testify`
- 测试文件与源码同目录，命名 `*_test.go`
- 运行单个包测试示例：`go test ./internal/number/...`
- 测试用例要求: 覆盖至少 `正常case`, `异常case`, `边缘case` 3种路径

### 前端测试

- 框架：vitest
- 测试文件在 `web/src/lib/__tests__/`
- 运行：`cd web && npm run test`（即 `vitest run`）
- 测试用例要求: 覆盖至少 `正常case`, `异常case`, `边缘case` 3种路径