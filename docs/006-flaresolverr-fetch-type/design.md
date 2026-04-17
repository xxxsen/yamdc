# FlareSolverr Fetch Type 设计文档

## 1. 背景

部分目标站点使用 Cloudflare 防护，标准 HTTP 请求会被拦截返回 challenge 页面。
项目原有方案是基于域名白名单的 `solverClient`：在配置文件中列出需要绕过的域名，由 HTTP 客户端按域名判断是否走 FlareSolverr。

该方案存在以下问题：

- **配置耦合**：用户需要同时维护域名列表和插件配置，当插件增减时容易遗漏
- **粒度不匹配**：域名级别的控制无法适应同一域名下部分接口需要绕过、部分不需要的场景
- **与 browser 模式不一致**：browser 模式已通过 YAML `fetch_type` 声明式集成，而 FlareSolverr 却需要额外的全局配置

本方案将 FlareSolverr 集成为 YAML 插件系统的第三种 `fetch_type`，与 `go-http`、`browser` 并列，
插件通过声明 `fetch_type: flaresolverr` 即可自动走 FlareSolverr 通道，无需额外配置域名。

## 2. 核心概念

### 2.1 fetch_type 扩展

| 值 | 说明 |
|---|---|
| `go-http`（默认） | 标准 HTTP 请求 |
| `browser` | 无头浏览器渲染（go-rod） |
| **`flaresolverr`** | 通过 FlareSolverr 服务绕过 Cloudflare 防护 |

同一插件内所有请求统一使用声明的 `fetch_type`，不允许交错。

### 2.2 配置精简

旧配置：

```jsonc
{
  "flare_solverr_config": {
    "enable": true,
    "host": "http://127.0.0.1:8191",
    "domains": {                      // 已移除
      "www.example.com": true
    }
  }
}
```

新配置仅保留两个字段：

```jsonc
{
  "flare_solverr_config": {
    "enable": true,
    "host": "http://127.0.0.1:8191"
  }
}
```

`domains` 字段被移除——是否走 FlareSolverr 完全由各插件的 `fetch_type` 声明决定。

### 2.3 与 FlareSolverr 的交互

FlareSolverr 是一个独立运行的代理服务，提供 REST API 来解决 Cloudflare 挑战。

请求格式（POST `/v1`）：

```json
{
  "cmd": "request.get",
  "url": "https://protected-site.example.com/page?id=12345",
  "maxTimeout": 40000
}
```

响应格式：

```json
{
  "status": "ok",
  "solution": {
    "url": "https://protected-site.example.com/page?id=12345",
    "status": 200,
    "response": "<html>...</html>",
    "cookies": [
      {
        "name": "cf_clearance",
        "value": "...",
        "domain": ".example.com",
        "path": "/",
        "secure": true,
        "httpOnly": true
      }
    ],
    "userAgent": "Mozilla/5.0 ..."
  }
}
```

关键点：
- 仅支持 GET 请求
- 响应中包含渲染后的完整 HTML 和解决 challenge 后获得的 cookies
- 这些 cookies 对后续的同域请求（如图片下载）至关重要

## 3. YAML 插件示例

```yaml
version: 1
name: example-cf-site
type: two-step
fetch_type: flaresolverr      # 声明使用 FlareSolverr
hosts:
  - https://protected-site.example.com
request:
  method: GET
  path: /search?keyword=${number}
  headers:
    Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1[@class="title"]/text()
      parser: string
      required: true
    # ...
```

与 `go-http` 插件的唯一区别是顶层增加 `fetch_type: flaresolverr`，其余 scrape/workflow 逻辑完全不变。

## 4. 架构

### 4.1 包结构

```
internal/flarerr/
├── context.go     # Params 定义及 context 传递（WithParams / GetParams）
├── client.go      # solveRequest 核心逻辑（与 FlareSolverr 交互）
├── model.go       # FlareSolverr 请求/响应数据模型
└── wrap.go        # httpClientWrap（context 驱动的 HTTP 客户端包装）
```

### 4.2 设计模式：context 驱动的 HTTP 客户端包装

本方案复用了 `browser` 模式确立的架构模式——通过 `context.Context` 传递信号标记，
HTTP 客户端包装层根据 context 中的标记决定请求路由：

```
YAML Plugin applyFetchTypeContext()
  │
  │  fetch_type == "flaresolverr" ?
  │  ├─ yes → 注入 flarerr.Params 到 context
  │  └─ no  → 不注入（或注入 browser.Params）
  │
  ▼
DefaultSearcher.invokeHTTPRequest()
  │
  ▼
browser.httpClientWrap.Do(req)     ← 最外层
  │
  │  context 中有 browser.Params ?
  │  ├─ yes → 浏览器渲染
  │  └─ no  → 传递给内层
  │
  ▼
flarerr.httpClientWrap.Do(req)     ← 中间层
  │
  │  context 中有 flarerr.Params ?
  │  ├─ yes → solveRequest() → FlareSolverr /v1
  │  │        └─ 提取 cookies 存入 jar
  │  └─ no  → 注入 jar 中的 cookies → impl.Do(req)
  │
  ▼
base HTTP client                   ← 最内层
```

### 4.3 客户端包装链

HTTP 客户端通过装饰器模式逐层包装，包装顺序（从内到外）：

```
base HTTP client
  └─ flarerr.httpClientWrap    （仅 flare_solverr_config.enable = true 时包装）
      └─ browser.httpClientWrap （始终包装）
```

`browser` 包装在最外层，`flarerr` 在中间层。这确保：

1. `fetch_type: browser` 的请求在最外层被拦截，走浏览器渲染
2. `fetch_type: flaresolverr` 的请求穿过 browser 层（无 browser.Params），在 flarerr 层被拦截
3. `fetch_type: go-http` 的请求穿过两层包装，直达 base HTTP client
4. 图片下载等非标记请求在 flarerr 层获得 cookie 注入后走标准 HTTP

### 4.4 cookie 流转

```
FlareSolverr /v1 响应
  │
  │  解析 solution.cookies
  ▼
flarerr.httpClientWrap.jar  ──注入cookies──▶  后续 HTTP 请求（图片下载等）
```

- FlareSolverr 返回的 cookies（如 `cf_clearance`）被存入 `httpClientWrap` 内部的 `cookiejar.Jar`
- 后续非 FlareSolverr 请求（如图片下载）自动从 jar 中注入这些 cookies
- 不重复注入已存在的同名 cookie

## 5. 核心接口

### 5.1 Params

```go
// internal/flarerr/context.go

type Params struct{}
```

`Params` 是一个标记类型，其存在于 context 中即表示该请求应走 FlareSolverr。
与 `browser.Params`（含 WaitSelector 等字段）不同，`flarerr.Params` 当前无需额外参数——
所有行为由 FlareSolverr 服务端自行决定。

```go
func WithParams(ctx context.Context, params *Params) context.Context
func GetParams(ctx context.Context) *Params   // 返回 nil 表示非 FlareSolverr 请求
```

### 5.2 httpClientWrap

```go
// internal/flarerr/wrap.go

type httpClientWrap struct {
    impl     client.IHTTPClient   // 内层 HTTP 客户端
    endpoint string               // FlareSolverr 服务地址
    timeout  time.Duration        // 请求超时（默认 40s）
    jar      *cookiejar.Jar       // cookie 持久化
}

func NewHTTPClient(impl client.IHTTPClient, endpoint string) client.IHTTPClient
```

### 5.3 solveRequest

```go
// internal/flarerr/client.go

type solveResult struct {
    StatusCode int
    HTML       []byte
    Cookies    []*http.Cookie
}

func solveRequest(endpoint string, timeout time.Duration, req *http.Request) (*solveResult, error)
```

内部向 FlareSolverr POST `/v1` 发起请求，返回包含渲染 HTML 和解析后 cookies 的结果。
仅支持 GET 方法，非 GET 请求返回错误。

## 6. 配置

### 6.1 全局配置

```go
// internal/config/config.go

type FlareSolverrConfig struct {
    Enable bool   `json:"enable"`
    Host   string `json:"host"`
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable` | bool | `false` | 是否启用 FlareSolverr 支持 |
| `host` | string | `http://127.0.0.1:8191` | FlareSolverr 服务地址 |

- `enable = false` 时不会创建 `flarerr.httpClientWrap`，零开销
- `enable = true` 但无插件使用 `fetch_type: flaresolverr` 时，wrapper 存在但不会被触发

### 6.2 YAML 插件字段

```yaml
fetch_type: flaresolverr   # 顶层字段，可选值: go-http | browser | flaresolverr
```

不需要额外的 `flaresolverr` 块或域名配置。

## 7. YAML 插件 fetch_type 分发

```go
// internal/searcher/plugin/yaml/plugin.go

func (p *SearchPlugin) applyFetchTypeContext(req *http.Request, spec *compiledRequest) *http.Request {
    switch p.spec.fetchType {
    case fetchTypeBrowser:
        return applyBrowserParams(req, spec)
    case fetchTypeFlaresolverr:
        return req.WithContext(flarerr.WithParams(req.Context(), &flarerr.Params{}))
    default:
        return req
    }
}
```

三种模式的上下文注入在同一个分发点完成，保持代码集中和一致。

## 8. 前端编辑器

Plugin Editor 的 Fetch Type 下拉增加 `flaresolverr` 选项：

```
┌────────────────────────────────────────────────────────────┐
│  Plugin Name [example]  Type [two-step▾]  Fetch Type [flaresolverr▾]  │
└────────────────────────────────────────────────────────────┘
```

三个可选值：`go-http` | `browser` | `flaresolverr`。

选择 `flaresolverr` 时不展示额外配置面板（与 `go-http` 一致），
因为 FlareSolverr 的行为完全由服务端控制，不需要客户端指定等待策略。

## 9. 与旧方案的对比

| 维度 | 旧方案（domain-based） | 新方案（fetch_type） |
|------|----------------------|---------------------|
| 配置位置 | 全局 `domains` 白名单 | 插件 YAML `fetch_type` |
| 控制粒度 | 域名级 | 插件级 |
| 耦合度 | 插件增减需同步修改全局配置 | 插件自包含，零外部依赖 |
| Cookie 管理 | 无（FlareSolverr 返回的 cookies 被丢弃） | 自动持久化并注入后续请求 |
| 代码复杂度 | 独立的 `solverClient` + 域名匹配逻辑 | 复用 browser 的 context 驱动模式 |
| 配置字段 | `enable` + `host` + `domains` | `enable` + `host` |

## 10. 兼容性

- 旧插件不含 `fetch_type` 字段 → 默认 `go-http`，行为不变
- `flare_solverr_config` 中的 `domains` 字段被移除，旧配置文件中如包含该字段会被 JSON 反序列化忽略
- 旧的 domain-based `solverClient`（`ICloudflareSolverClient`、`New`、`MustAddToSolverList`）已整体移除
- 图片等静态资源始终走标准 HTTP + cookie 注入，不经过 FlareSolverr

## 11. 依赖

无新增外部依赖。FlareSolverr 交互使用标准库 `net/http` + `encoding/json`。

运行时依赖：

- FlareSolverr 服务实例（Docker 部署推荐：`ghcr.io/flaresolverr/flaresolverr:latest`）
