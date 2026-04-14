# 浏览器抓取（Browser Fetch）设计文档

## 1. 背景

项目原有抓取链路基于标准 HTTP 请求，无法处理 JavaScript 动态渲染的站点。
本方案基于 go-rod 引入无头浏览器抓取模式，使插件可声明使用浏览器渲染页面，
运行时自动管理浏览器生命周期，渲染后的 HTML 透明返回给下游 scrape 逻辑。

## 2. 核心概念

### 2.1 fetch_type

插件顶层字段，决定该插件的全部请求统一使用哪种抓取方式：

| 值 | 说明 |
|---|---|
| `go-http`（默认） | 标准 HTTP 请求，不启动浏览器 |
| `browser` | 所有页面请求通过无头浏览器渲染 |

设计原则：**同一插件内不允许交错使用两种抓取方式**，避免 cookie 在浏览器和 HTTP 客户端之间同步的复杂性。

### 2.2 browser 块

当 `fetch_type: browser` 时，每个 request 可选声明 `browser` 块来控制页面等待策略：

```yaml
request:
  browser:
    wait_selector: "//div[@class='result']"   # XPath，等待该元素出现
    wait_timeout: 30                           # 超时秒数
```

- **有 wait_selector**：导航后等待指定 XPath 元素出现，超时则报错
- **无 wait_selector**：导航后等待 `NetworkAlmostIdle`（网络请求基本停止）后返回

不同 request 可以指定不同的 wait 参数（如搜索页无需等待特定元素，详情页需要）。

## 3. YAML 插件示例

### 3.1 go-http 模式（默认）

```yaml
version: 1
name: example
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true
```

### 3.2 browser 模式

```yaml
version: 1
name: example-browser
type: two-step
fetch_type: browser
hosts:
  - https://js-rendered-site.com
request:
  method: GET
  path: /search?q=${number}
  headers:
    Accept-Language: en-US,en;q=0.5
  cookies:
    session: verified
workflow:
  search_select:
    selectors:
      - name: link
        kind: xpath
        expr: //a[@class="item"]/@href
    match:
      mode: and
      conditions:
        - contains("${item.link}", "${number}")
    return: "${item.link}"
    next_request:
      method: GET
      url: "${value}"
      browser:
        wait_selector: //div[@class="detail"]
        wait_timeout: 15
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //h1/text()
      parser: string
      required: true
    cover:
      selector:
        kind: xpath
        expr: //img[@class="cover"]/@src
      parser: string
      required: true
```

## 4. 架构

### 4.1 包结构

```
internal/browser/
├── context.go     # Params 定义及 context 传递
├── config.go      # Config（RemoteURL/DataDir/Proxy）
├── browser.go     # INavigator 接口、httpClientWrap、cookie 管理
└── rod.go         # go-rod 实现（本地 + 远程）
```

### 4.2 请求链路

```
YAML Plugin buildRequest()
  │
  │  fetch_type == "browser" ?
  │  ├─ yes → 注入 browser.Params（含用户 headers）到 context
  │  └─ no  → 不注入
  │
  ▼
DefaultSearcher.invokeHTTPRequest()
  │
  ▼
httpClientWrap.Do(req)
  │
  │  context 中有 Params ?
  │  ├─ yes → 合并 cookies → navigator.Navigate() → 返回渲染 HTML
  │  │        └─ 导航后提取 cookies 存入 jar
  │  └─ no  → 注入 jar 中的 cookies → impl.Do(req)（标准 HTTP）
  │
  ▼
响应返回给 scrape 层（零改动）
```

### 4.3 cookie 流转

```
browser Navigate  ──提取cookies──▶  httpClientWrap.jar
                                          │
后续 HTTP 请求（图片下载等） ◀──注入cookies──┘
```

- `fetch_type=browser`：浏览器导航后通过 `extractCookies` 提取页面 cookies 存入 `httpClientWrap` 自有的 `cookiejar.Jar`
- 后续非浏览器请求（如图片下载）通过 `injectCookies` 将 jar 中的 cookies 注入到 `http.Request`
- 不直接修改底层 `http.Client` 的 jar，避免耦合和竞争

### 4.4 headers 传递

用户在 YAML `request.headers` 中配置的 header **仅限用户声明的 header** 会通过 `Params.Headers` 传递给浏览器：

1. `buildRequest` 中根据 `spec.headers` 的 key 从 `req.Header` 提取用户配置的值填入 `Params.Headers`
2. `rod.go` 通过 `page.SetExtraHeaders()` 注入到浏览器页面
3. 不传递框架自动添加的 header（如 Referer）和 HTTP 专属 header（如 Content-Type）

## 5. 核心接口

### 5.1 Params

```go
// internal/browser/context.go

type Params struct {
    WaitSelector string           // XPath，为空则等待 NetworkAlmostIdle
    WaitTimeout  time.Duration    // 等待超时，0 使用默认 60s
    Cookies      []*http.Cookie   // 注入到浏览器的 cookies
    Headers      http.Header      // 注入到浏览器的 headers（仅用户配置）
}
```

通过 `context.Context` 在请求链路中传递：

```go
func WithParams(ctx context.Context, params *Params) context.Context
func GetParams(ctx context.Context) *Params   // 返回 nil 表示非浏览器请求
```

### 5.2 INavigator

```go
// internal/browser/browser.go

type NavigateResult struct {
    HTML    []byte
    Cookies []*http.Cookie
}

type INavigator interface {
    Navigate(ctx context.Context, url string, params *Params) (*NavigateResult, error)
    Close() error
}
```

### 5.3 Config

```go
// internal/browser/config.go

type Config struct {
    RemoteURL string   // CDP 远程调试地址，为空则启动本地浏览器
    DataDir   string   // 浏览器下载/缓存目录（${data_dir}/browser）
    Proxy     string   // 代理，复用 network_config.proxy
}
```

全局配置中对应：

```jsonc
{
  "browser_config": {
    "remote_url": ""   // 可选，配置后连接远程浏览器
  }
}
```

## 6. go-rod 实现

### 6.1 浏览器启动

- 使用 go-rod 自动下载的 Chromium，不搜索系统浏览器路径
- 下载目录：`${data_dir}/browser`
- 启动参数：`--headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage --window-size=1920,1080`
- 代理：从全局 `network_config.proxy` 获取

### 6.2 反检测

使用 `go-rod/stealth` 创建页面（`stealth.Page(b)`），自动注入反检测 JavaScript 脚本，
规避常见的 `navigator.webdriver` 等指纹检测。

### 6.3 页面等待策略

| 条件 | 行为 |
|------|------|
| `WaitSelector` 非空 | `page.Navigate(url)` → `page.ElementX(selector)` |
| `WaitSelector` 为空 | `page.WaitNavigation(NetworkAlmostIdle)` → `page.Navigate(url)` |

### 6.4 生命周期管理

- **懒初始化**：首次浏览器请求时才启动 Chromium
- **空闲回收**：30s 无请求后自动关闭浏览器进程释放资源
- **导航期间暂停计时**：`pauseIdleTimer` / `resumeIdleTimer` 避免导航中被回收
- **应用退出清理**：通过 `AddCleanup` 注册 `nav.Close()`

如果没有任何插件使用 `fetch_type: browser`，浏览器进程从不启动，零开销。

### 6.5 远程调试

配置 `browser_config.remote_url` 后，连接到外部运行的浏览器实例，用于开发调试：

```bash
# 启动调试浏览器
chrome --remote-debugging-port=9222

# 配置连接
{
  "browser_config": {
    "remote_url": "localhost:9222"
  }
}
```

远程模式下不管理浏览器进程（不启动、不关闭、不回收）。

## 7. 前端编辑器

### 7.1 Fetch Type 配置

位于 Basic 区域，与 Plugin Name、Type 同行展示：

```
┌──────────────────────────────────────────────────┐
│  Plugin Name [example  ]  Type [one-step▾]  Fetch Type [go-http▾]  │
└──────────────────────────────────────────────────┘
```

### 7.2 Browser Rendering 配置

位于 Request 区域的高级选项中，仅在 `Fetch Type = browser` 时显示：

```
┌──────────────────────────────────┐
│ ▼ Browser Rendering              │
│                                  │
│  Wait XPath   [//div[@class=...]]│
│  Wait Timeout [30              ] │
└──────────────────────────────────┘
```

### 7.3 数据模型

```typescript
// EditorState 顶层
fetchType: string;             // "go-http" | "browser"

// RequestFormState（每个 request 独立）
browserWaitSelector: string;   // XPath
browserWaitTimeout: string;    // 秒数

// PluginEditorDraft
fetch_type?: string;           // 仅 "browser" 时输出，空/go-http 时 undefined

// PluginEditorBrowserSpec
interface PluginEditorBrowserSpec {
  wait_selector?: string;
  wait_timeout?: number;
}
```

## 8. 兼容性

- 旧插件不含 `fetch_type` 字段 → 默认 `go-http`，行为不变
- `BrowserSpec` 中不再有 `enable` 字段，由 `fetch_type` 统一控制
- 图片等静态资源始终走标准 HTTP + cookie 注入，不经过浏览器

## 9. 集成测试

通过环境变量驱动的集成测试验证两种模式（`internal/searcher/plugin/yaml/fetch_type_integration_test.go`）：

```bash
FETCH_TYPE_TEST_URL="https://raw.githubusercontent.com/.../plugins/plugin.yaml" \
FETCH_TYPE_TEST_NUMBER="ABC-1234" \
FETCH_TYPE_TEST_WAIT='//div[@class="row movie"]' \
go test ./internal/searcher/plugin/yaml/ -run 'TestFetchType_' -v
```

| 环境变量 | 说明 |
|---------|------|
| `FETCH_TYPE_TEST_URL` | 远程 YAML 插件 URL（必填） |
| `FETCH_TYPE_TEST_NUMBER` | 测试番号（必填） |
| `FETCH_TYPE_TEST_WAIT` | browser 模式的 wait_selector（选填） |

测试流程：
1. 下载远程 YAML → go-http 模式抓取 → 验证元数据 + 图片下载
2. 自动转换为 browser YAML → browser 模式抓取 → 验证元数据 + 图片下载

浏览器 headers/cookies 注入的单元测试位于 `internal/browser/rod_test.go` 的 `TestBrowserInjectHeadersAndCookies`，
通过本地 HTTP server 验证自定义 headers 和 cookies 确实被浏览器发出。

## 10. 依赖

```
github.com/go-rod/rod       # 浏览器自动化
github.com/go-rod/stealth   # 反指纹检测
```

go-rod 自动管理 Chromium 下载，二进制缓存在 `${data_dir}/browser` 目录。
