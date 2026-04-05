# 搜索插件 YAML 化完整设计文档

## 1. 文档目的

本文档用于收敛本次“搜索插件 YAML 化”需求涉及的设计、实现边界和最终落地结果。

它作为后续维护、扩展、前端调试页接入和代码 review 的统一入口。

本文档以**当前代码实际实现**为准，而不是早期草案。

## 2. 背景与目标

项目原有搜索插件大多以单个 Go 文件实现，主要负责：

1. 组装请求
2. 执行一步或两步搜索
3. 从 HTML 或 JSON 响应中提取字段
4. 对少量字段做字符串、时间、时长转换

本次改造目标：

1. 将尽可能多的搜索插件剥离为 YAML 配置
2. 不复制搜索主流程，继续复用 `DefaultSearcher`
3. 让 YAML 成为前端可构建、可调试、可校验的插件描述格式
4. 保持能力通用，避免把 YAML 演化成插件私有 DSL

## 3. 总体架构

运行时边界保持不变：

1. 插件仍实现 `api.IPlugin`
2. `DefaultSearcher` 继续负责统一 HTTP 请求、缓存、图片落盘、元数据修正与校验

整体链路：

```text
yaml bytes
  -> raw spec
  -> compiler
  -> compiled spec
  -> YAMLSearchPlugin (api.IPlugin)
  -> DefaultSearcher
```

相关目录：

```text
internal/searcher/plugin/yaml/
internal/searcher/plugin/yamlplugin/
internal/searcher/plugin/yamlitest/
```

其中：

1. `yaml/` 存放内置 YAML 配置
2. `yamlplugin/` 存放 loader/compiler/runtime
3. `yamlitest/` 存放 legacy vs yaml parity 集成测试

## 4. 设计原则

### 4.1 复用现有搜索主流程

不重新实现 `DefaultSearcher`，避免重复维护请求、缓存、媒体抓取、结果校验等公共逻辑。

### 4.2 YAML 不是脚本语言

YAML 只表达：

1. 请求定义
2. 工作流定义
3. 字段提取
4. 少量模板函数
5. 少量条件函数
6. 少量后处理

不支持任意脚本执行，不支持完整布尔表达式语言。

### 4.3 允许少量领域辅助能力

允许极少量“影片搜索领域”的辅助函数或 parser，但不允许插件私有能力混入白名单。

### 4.4 面向前端构建

虽然 YAML 最终由前端调试页构建，但不强制所有内容都变成 AST 级配置树。当前方案采用：

1. 普通值使用轻量模板字符串
2. 条件使用 `mode + conditions`
3. 条件项使用受限函数调用字符串

## 5. 能力分层

### 5.1 通用抓取能力

这是 YAML schema 的主体：

1. `request`
2. `multi_request`
3. `workflow.search_select`
4. `scrape.format: html`
5. `scrape.format: json`
6. `selector.kind: xpath`
7. `selector.kind: jsonpath`
8. `decode_charset`
9. 模板函数
10. 条件函数
11. 通用 parser / transform

### 5.2 领域辅助能力

只保留少量与当前番号/影片搜索生态相关但不绑定单一站点的能力，例如：

1. `clean_number`

### 5.3 系统映射能力

用于映射到当前 `MovieMeta`：

1. `postprocess.defaults`
2. `postprocess.switch_config`

这部分不是纯抓取 DSL 的核心能力，应控制规模。

## 6. 最终 Schema

当前顶层结构：

```yaml
version: 1
name: plugin_name
type: one-step | two-step
hosts:
  - https://example.com

precheck: {}
request: {}
multi_request: {}
workflow: {}
scrape: {}
postprocess: {}
```

约束：

1. `request` 与 `multi_request` 互斥
2. `type: two-step` 时，必须存在 `workflow.search_select`
3. `request` 或 `multi_request` 至少存在一个

### 6.1 `precheck`

用于番号前置筛选和变量准备。

示例：

```yaml
precheck:
  number_patterns:
    - ^PREFIX-.*
```

### 6.2 `request`

定义单次请求。

支持：

1. `method`
2. `path` 或 `url`
3. `query`
4. `headers`
5. `cookies`
6. `body`
7. `accept_status_codes`
8. `not_found_status_codes`
9. `response.decode_charset`

### 6.3 `multi_request`

用于多候选请求回退。

这是最终实现里替代早期 `workflow.multi_try` 的结构。

示例：

```yaml
multi_request:
  unique: true
  candidates:
    - ${number}
    - ${replace(${number}, "-", "_")}
  request:
    method: GET
    path: /search/${candidate}
    accept_status_codes: [200]
  success_when:
    mode: and
    conditions:
      - selector_exists(xpath("//*[@id='waterfall']/div/a"))
```

### 6.4 `workflow.search_select`

用于两步搜索中的搜索页结果选择。

示例：

```yaml
workflow:
  search_select:
    selectors:
      - name: read_link
        kind: xpath
        expr: //*[@id="waterfall"]/div/a/@href
    match:
      mode: and
      conditions:
        - contains("${item.read_link}", "movie")
      expect_count: 1
    return: ${item.read_link}
    next_request:
      method: GET
      url: ${build_url("https:", ${value})}
      accept_status_codes: [200]
```

说明：

1. `expect_count` 位于 `match` 下
2. `0` 或不填表示关闭数量约束

### 6.5 `scrape`

当前支持两种格式：

1. `html`
2. `json`

HTML 用 `xpath`，JSON 用 `jsonpath`。

示例：

```yaml
scrape:
  format: json
  fields:
    title:
      selector:
        kind: jsonpath
        expr: $.data.title
      parser: string
```

### 6.6 `postprocess`

支持：

1. `assign`
2. `defaults`
3. `switch_config`

示例：

```yaml
postprocess:
  assign:
    number: ${number}
  defaults:
    title_lang: en
    plot_lang: en
  switch_config:
    disable_release_date_check: true
```

## 7. 模板系统

模板使用轻量字符串插值：

```text
${number}
${replace(${number}, "-", "_")}
${build_url(${host}, ${value})}
```

当前常用变量：

1. `number`
2. `host`
3. `body`
4. `vars.xxx`
5. `item.xxx`
6. `item_variables.xxx`
7. `meta.xxx`
8. `value`
9. `candidate`

当前常用模板函数：

1. `build_url`
2. `to_upper`
3. `to_lower`
4. `trim`
5. `trim_prefix`
6. `trim_suffix`
7. `replace`
8. `clean_number`
9. `first_non_empty`
10. `concat`
11. `last_segment`

### 7.1 模板约束

模板系统只做值计算，不做布尔判断和流程控制。

约束：

1. 模板函数必须来自白名单
2. 模板变量必须来自白名单
3. 模板返回值统一按字符串处理
4. 允许有限嵌套，但不建议超过 2 层

### 7.2 模板函数说明

当前已实现模板函数：

1. `build_url(base, ref)`
2. `to_upper(value)`
3. `to_lower(value)`
4. `trim(value)`
5. `trim_prefix(value, prefix)`
6. `trim_suffix(value, suffix)`
7. `replace(value, old, new)`
8. `clean_number(value)`
9. `first_non_empty(v1, v2, ...)`
10. `concat(v1, v2, ...)`
11. `last_segment(value, sep)`

说明：

1. `clean_number` 属于领域辅助函数，不属于纯通用字符串函数
2. `last_segment` 是通用拆段函数，适用于从标准化编号中提取末段标识

## 8. 条件系统

条件统一采用：

```yaml
match:
  mode: and
  conditions:
    - equals("${vars.clean_number}", "${item_variables.clean_item_number}")
```

当前白名单函数：

1. `contains`
2. `equals`
3. `starts_with`
4. `ends_with`
5. `regex_match`
6. `selector_exists`

不支持完整表达式字符串，例如：

```yaml
match: contains("${body}", "片名") and contains("${body}", "番号")
```

### 8.1 条件函数参数约束

当前条件函数签名可理解为：

1. `contains(string, string) -> bool`
2. `equals(string, string) -> bool`
3. `starts_with(string, string) -> bool`
4. `ends_with(string, string) -> bool`
5. `regex_match(string, pattern) -> bool`
6. `selector_exists(xpath("...")) -> bool`

约束：

1. 所有字符串参数必须加双引号
2. `selector_exists` 当前只支持 `xpath("...")`
3. 条件只允许出现在 `mode + conditions` 结构中

## 9. Selector 能力

### 9.1 HTML

HTML 抓取使用：

1. `scrape.format: html`
2. `selector.kind: xpath`

### 9.2 JSON

JSON 抓取使用：

1. `scrape.format: json`
2. `selector.kind: jsonpath`

JSONPath 执行层当前基于：

1. `github.com/PaesslerAG/jsonpath`

当前已经验证的典型表达式：

1. `$.result.name`
2. `$.result.factories[0].name`
3. `$.result.actors[*].name`
4. `$.data.tagList[*].label`
5. `$.data.model.displayName`

运行时约束：

1. 缺失 JSON key / index 视为“空结果”，不作为硬错误
2. 单值字段取首个命中
3. `multi: true` 字段取全量命中列表

## 10. Parser 与 Transform

### 10.1 当前常用 parser

1. `string`
2. `string_list`
3. `date_only`
4. `duration_default`
5. `duration_hhmmss`
6. `duration_mm`
7. `duration_mmss`
8. `duration_human`
9. `time_format`
10. `date_layout_soft`

说明：

1. `duration_mmss` 为本次补充的通用 parser，适用于 `mm:ss` 形式时长
2. `date_layout_soft` 解析失败时返回 `0`，不直接报错

### 10.2 当前常用 string transform

1. `trim`
2. `trim_prefix`
3. `trim_suffix`
4. `trim_charset`
5. `replace`
6. `split_index`
7. `to_upper`
8. `to_lower`
9. `regex_extract`

说明：

1. `regex_extract` 为本次补充的通用 transform，可用于从 style 或文本中截取目标字段
2. 当正则不匹配时，当前实现返回空字符串

### 10.3 当前常用 list transform

1. `remove_empty`
2. `dedupe`
3. `map_trim`
4. `replace`
5. `split`
6. `to_upper`
7. `to_lower`

## 11. Go 侧实现设计

### 11.1 主要文件

当前核心实现位于：

1. `internal/searcher/plugin/yamlplugin/spec.go`
2. `internal/searcher/plugin/yamlplugin/plugin.go`
3. `internal/searcher/plugin/yamlplugin/template.go`
4. `internal/searcher/plugin/yamlplugin/condition.go`
5. `internal/searcher/plugin/yamlplugin/jsonpath.go`
6. `internal/searcher/plugin/yamlplugin/builtins.go`

### 11.2 运行时职责

`spec.go`

1. 承载 YAML 原始结构
2. 定义 schema 级字段

`template.go`

1. 模板语法校验
2. 模板变量渲染
3. 模板函数执行

`condition.go`

1. 条件 grammar 解析
2. 条件函数执行

`jsonpath.go`

1. JSONPath 执行适配
2. 基于 `github.com/PaesslerAG/jsonpath`
3. 将结果拍平成字符串列表

`plugin.go`

1. 编译 raw spec
2. 构建请求
3. 驱动 `multi_request`
4. 驱动 `workflow.search_select`
5. 执行 HTML / JSON scrape
6. 执行 postprocess

`builtins.go`

1. 注册 YAML 内置插件
2. 覆盖 legacy 插件前保留 `legacy:<name>`

### 11.3 当前关键编译约束

1. `request` 与 `multi_request` 互斥
2. `type: two-step` 必须有 `workflow.search_select`
3. `scrape.format: html` 时 selector 只能是 `xpath`
4. `scrape.format: json` 时 selector 只能是 `jsonpath`
5. `request.path` 与 `request.url` 互斥
6. 条件必须是白名单函数
7. 模板函数必须是白名单函数

## 12. 请求与响应职责边界

### 11.1 请求阶段

1. `OnMakeHTTPRequest` 构造首个请求
2. `multi_request` 负责候选请求回退
3. `workflow.search_select` 负责搜索页选中详情链接

### 11.2 响应阶段

1. `workflow` 自己处理中间请求状态码
2. `OnPrecheckResponse` 只处理最终响应

### 11.3 媒体请求装饰

媒体请求默认继承最终页面请求的：

1. `headers`
2. `cookies`
3. `referer`

这一点对依赖来源页请求头或来源页引用关系的站点是必要的。

## 13. 前端构建边界

当前 schema 是按“前端可构建，但不强迫一切结构化 AST”设计的。

适合前端直接做表单化编辑的部分：

1. `request`
2. `multi_request`
3. `scrape.fields`
4. `postprocess`

适合做受限编辑器的部分：

1. 模板字符串
2. `match.mode + conditions`
3. `success_when.mode + conditions`

不建议前端把 YAML 当成通用脚本语言解释器。

## 14. parity 集成测试设计

每个已迁移插件都应有一个 parity 集成测试文件，位于：

```text
internal/searcher/plugin/yamlitest/
```

规则：

1. 每个测试维护自己的 `numbers` 列表
2. 为空时 `skip`
3. 逐个调用 legacy 插件与 YAML 插件
4. 比较 `error presence / found / meta`

关键实现点：

1. YAML 插件覆盖注册前，legacy 会保存为 `legacy:<name>`
2. 比较前会清理 `ExtInfo` 和媒体 `Key`

## 15. 本次迁移过程中的关键设计收敛

### 13.1 `multi_try` -> `multi_request`

早期草案中的 `workflow.multi_try` 已被替换为顶层 `multi_request`。

原因：

1. `multi_try` 本质是请求调度策略，不是页内 workflow
2. 顶层建模后，`multi_request` 的成功响应可以继续进入 `workflow.search_select`
3. 多候选搜索词重试类插件因此可以自然表达

### 13.2 `expect_match_count` -> `match.expect_count`

匹配数量约束被收拢到 `match` 下：

```yaml
match:
  mode: and
  conditions:
    - contains("${item.read_link}", "movie")
  expect_count: 1
```

### 15.3 JSON 正式支持

为支持基于 JSON API 的站点，runtime 新增：

1. `scrape.format: json`
2. `selector.kind: jsonpath`

### 15.4 API 化站点支持

当站点页面结构不稳定或已 API 化时，可直接切换到：

1. API 请求
2. JSON scrape
3. 必要的请求头补充

legacy 与 YAML 可以同时迁移到 API/JSON 方案。

### 15.5 分阶段迁移复杂站点

对于存在多种编号格式、且不同格式走不同流程的站点，可先支持其中一条更稳定、更简单的链路。

原因：

1. 简单链路可以先落地并建立 parity
2. 复杂链路往往需要额外的分支、query 参数匹配或更复杂的工作流能力

## 16. 当前插件迁移结果

截至当前代码，现有插件体系已经具备完整的 YAML 迁移能力覆盖。

其中值得额外说明的：

1. HTML 与 JSON 两条抓取链路都已完成验证
2. 复杂站点允许按子场景拆分、分阶段支持

## 17. 明确不进入白名单的能力

以下能力不应进入 schema、template、condition 白名单：

1. 站点名出现在函数名里
2. 只能服务于单一插件的字符串函数
3. 只能靠理解某一站点 DOM / URL 结构才能解释的函数

示例：

1. `strip_site_prefix(...)`
2. `parse_site_specific_id(...)`
3. `normalize_site_number(...)`
4. `extract_site_specific_model(...)`

本次如果某个站点需要能力扩展，原则是：

1. 先抽象成通用能力
2. 再进入白名单

## 18. 后续建议

后续如果继续演进，建议优先控制下面几类变更：

1. 新模板函数必须先过“是否通用”评审
2. 新条件函数必须保持布尔语义和固定签名
3. 新 transform/parser 优先做成通用能力，不引入站点私有命名
4. 对复杂多分支链路，优先考虑是否引入通用 `branch` 与 `url_query`
5. parity 测试应持续补号段，避免迁移后无回归保障

## 19. 结论

本次 YAML 化改造已经形成一套可运行、可测试、可逐步扩展的搜索插件体系：

1. 运行时边界稳定
2. Schema 足够表达当前大部分插件
3. HTML 与 JSON 两条抓取路径都已落地
4. legacy 与 YAML 之间已有 parity 测试保障

后续扩展应继续遵循两个原则：

1. 先抽象成通用能力，再进入白名单
2. 能拆分插件场景时，优先拆分，不急于引入复杂控制流
