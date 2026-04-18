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
make test                 # go test ./cmd/... ./internal/...（快速测试，不收集覆盖率）
make test-coverage        # go test ./internal/... 并检查覆盖率 ≥ 95%
make install-golangci-lint  # 安装 golangci-lint 到 ./bin/
make lint-go              # golangci-lint run（需先 install）
make backend-check        # build + test-coverage + lint-go（后端完整检查）

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
- 覆盖率要求：`internal/` 目录整体覆盖率 ≥ 95%，低于阈值 CI 将失败
- 覆盖率排除：`internal/browser` 包不计入覆盖率阈值（需要真实浏览器，CI 环境无浏览器）
- 覆盖率排除：`internal/bootstrap` 包不计入覆盖率阈值（组装/编排代码，需要集成环境）
- 覆盖率检查：`make test-coverage`（阈值可通过 `GO_COVERAGE_THRESHOLD` 变量调整）
- 浏览器测试：设置 `YAMDC_BROWSER_TEST=1` 环境变量可启用依赖真实浏览器的测试
- 测试用例要求: 覆盖至少 `正常case`, `异常case`, `边缘case` 3种路径

### 前端测试

- 框架：vitest
- 测试文件在 `web/src/lib/__tests__/`
- 运行：`cd web && npm run test`（即 `vitest run`）
- 测试用例要求: 覆盖至少 `正常case`, `异常case`, `边缘case` 3种路径

---

## 前端范围与风格约定

- **定位：桌面优先**。前端只保证桌面浏览器（宽度 ≥ 1024px）体验，不考虑移动端 / 平板布局。
  - 不投入响应式适配成本；已有的 `@media` 断点代码允许随手清理，但不强制。
  - `data-label="..."` 这类"移动端卡片化表格"兜底 pattern **不作为新组件规范**，存量可以逐步下线。
- **视觉风格：warm serif 复古基调**（`Iowan Old Style` + 暖色底）是刻意选的，重构/重写不改主视觉语言。
- **样式工程**：走 Tailwind v4 utility + `@theme` token。新增组件优先用 utility，尽量不往 `globals.css` 加全局 class。详见 `td/022-frontend-optimization-roadmap.md`。
- **组件库**：手写 `components/ui/`（Button/Modal/Badge 等），不引入 shadcn/ui 或其它第三方组件库。
- **暗色模式**：明确不做，不要引入 `prefers-color-scheme: dark` 或 `data-theme` 切换逻辑。