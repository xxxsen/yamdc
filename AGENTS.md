# AGENTS.md — AI Agent 工程规范

本文件定义 AI Agent 在 yamdc 项目中进行代码修改、测试和审查时必须遵循的规范。

---

## 0. 总则（最高优先级）

- **不允许主动降低代码质量要求**。例如：
  - 修改 `.golangci.yml` 放宽 lint 检查；
  - 降低 Go / 前端测试覆盖率阈值；
  - 通过 `//nolint`、`// eslint-disable` 或类似手段绕过问题；
  - 在 Makefile、CI 或测试脚本中跳过失败用例；
  - 用删除测试、缩小覆盖范围替代真实修复。
- 上述行为均**不被允许**，除非得到用户明确许可。必须优先通过重构、拆分函数、补充测试、修正实现来解决。
- 当本文件与其它规则、用户偏好或外部规范冲突时，以本文件为准；如有疑问，先向用户确认。

---

## 1. 项目概述

yamdc — 影片元数据抓取、补全与媒体库管理工具。Go 后端 + Next.js 前端，前后端分离。

- `yamdc run` — 单次扫描抓取
- `yamdc server` — HTTP API 服务 + WebUI

---

## 2. 文档与临时方案目录规范

- `td/` 是**临时方案设计目录**，仅用于功能设计、review、实施过程中的中间文档沉淀。
- `td/` 中的内容不是稳定文档，不作为长期工程文档维护。
- 功能完整实施、review 通过并确认稳定后，必须将对应内容整理进 `docs/`。
- **禁止在正式文档、测试、代码、注释、用户可见文案中引用 `td/` 或 `tdxxx` 临时编号**，包括但不限于：
  - `docs/` 下的文档；
  - Go / TypeScript / React 源码；
  - 单元测试、集成测试、E2E 测试；
  - 代码注释、测试注释、测试用例名称；
  - API 返回文案、前端展示文案、错误提示。
- 测试文件名必须按被测功能或关联源文件命名，例如测试 `abc.go` 的行为应命名为 `abc_test.go` 或清晰的功能名，不允许使用 `tdxxx_test.go`。
- 发现已有代码、测试或正式文档引用 `td/` 时，应在相关修改中一并清理为功能语义描述或正式 `docs/` 引用。

---

## 3. 构建与测试

### Make 命令一览

```bash
# ── 后端 ──
make build                  # go build -o yamdc ./cmd/yamdc
make test                   # go test -race ./cmd/... ./internal/...
make test-coverage          # go test -race ./internal/... 并检查覆盖率 >= 95%
make install-golangci-lint  # 安装 golangci-lint 到 ./bin/
make lint-go                # golangci-lint run
make backend-check          # build + test-coverage + lint-go

# ── 前端 ──
make web-install            # cd web && npm ci
make web-lint               # cd web && npm run lint
make web-knip               # cd web && npm run knip
make web-test               # cd web && npm run test:coverage
make web-build              # cd web && npm run build
make web-check              # install + lint + knip + test + build

# ── 全量 ──
make ci-check               # backend-check + web-check
```

**提交前必须确保 `make ci-check` 通过。**

### Go 测试

- 测试范围：`./cmd/... ./internal/...`
- 框架：标准 `testing` + `github.com/stretchr/testify`
- 测试文件与源码同目录，命名 `*_test.go`
- 覆盖率要求：`internal/` 目录整体覆盖率 >= 95%，低于阈值 CI 失败
- 覆盖率排除：
  - `internal/browser`：依赖真实浏览器；
  - `internal/bootstrap`：组装/编排代码；
  - `internal/testsupport`：测试助手包。
- 覆盖率检查：`make test-coverage`
- 浏览器测试：设置 `YAMDC_BROWSER_TEST=1` 可启用真实浏览器测试
- 每个被测函数/组件至少覆盖：
  - 正常路径；
  - 异常路径；
  - 边缘路径。

### 前端测试

- 框架：vitest
- 测试文件优先与被测模块就近放在 `__tests__/`
- 运行：`cd web && npm run test` 或 `make web-test`
- 覆盖率要求：statements/functions/lines >= 95%，branches 当前不得低于配置阈值
- 新增被测源文件时，必须同步更新 `web/vitest.config.ts` 的 coverage include，不能用缩小 include 的方式维持覆盖率。

---

## 4. Lint 规范

### Go 后端

- 使用 `golangci-lint` v2，配置文件为 `.golangci.yml`
- **禁止修改 `.golangci.yml`**，除非获得明确许可
- 所有代码必须通过 lint 检查，零 issue

### 前端

- 使用 ESLint + Next.js / TypeScript 规则
- 所有代码必须通过 `npm run lint`
- 禁止通过 `eslint-disable` 压制问题；确需例外时必须有明确、局部、可审计的原因，并优先拆分或重构。

---

## 5. 前端范围与风格约定

- **定位：桌面优先**。前端只保证桌面浏览器（宽度 >= 1024px）体验，不投入移动端 / 平板布局成本。
- **视觉风格：warm serif 复古基调**（`Iowan Old Style` + 暖色底）是刻意选择，重构或重写不得改变主视觉语言。
- **样式工程**：使用 Tailwind v4 utility + `@theme` token。新增组件优先用 utility，避免新增全局 class。
- **组件库**：使用手写 `components/ui/`，不引入 shadcn/ui 或其它第三方组件库。
- **暗色模式**：明确不做，不引入 `prefers-color-scheme: dark` 或 `data-theme` 切换逻辑。
- 新增可交互控件必须有清晰的 hover、focus-visible、disabled、loading/error/empty 状态。
- 图标按钮优先使用 `lucide-react`，并提供 `aria-label` / `title`。

---

## 6. 代码修改后的 Review 流程

当本次任务涉及代码修改（bug 修复、重构、优化等）时，完成编码和检查后，必须启动独立 review 流程：

```text
review -> 修复 -> 再 review -> 再修复 -> ... -> 无任何 P0/P1/P2 bug
```

退出条件：连续一轮 review 中未发现任何 P0、P1 或 P2 级别 bug。

| 级别 | 定义 |
|------|------|
| P0 | 崩溃、数据损坏、安全漏洞 |
| P1 | 功能性错误、计算结果不正确 |
| P2 | 逻辑缺陷、未处理边界、性能问题 |
| P3 | 代码风格、命名不规范（不阻塞） |

---

## 7. 功能实现后的 Review 流程

当本次任务涉及基于需求文档实现新功能时，完成编码和检查后，必须启动独立功能 review：

```text
review -> 补齐功能 -> 再 review -> 再补齐功能 -> ... -> 无任何功能缺失
```

功能 review 完成后，还必须执行“代码修改后的 Review 流程”，确保没有 P0/P1/P2 bug。

---

## 8. 跨仓库集成测试要求

本工程涉及两个配套仓库：

- 搜索插件仓库
- 清理脚本仓库

如果修改涉及搜索插件加载、插件运行、插件 case 格式、搜索结果解析、MovieID 清理规则、清理脚本 case 格式或 ruleset 兼容性，必须补充并运行对应的跨仓库集成测试，验证修改后的逻辑符合预期。

必须优先使用真实仓库中的 case / ruleset：

- 搜索插件：使用真实插件仓库中的 case，通过 `yamdc plugin-test` 或等价集成命令验证。
- 清理脚本：使用真实清理脚本仓库中的 cases 与 rules，通过 `yamdc ruleset-test` 或等价集成命令验证。

不允许只用单元测试替代跨仓库集成测试；单元测试用于覆盖局部边界，集成测试用于确认真实插件 / 规则仓库仍兼容。
