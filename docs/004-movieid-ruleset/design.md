# movieid-ruleset 系统设计

## 一、文档目的

本文档描述 `movieid-ruleset` 系统的正式设计与当前实现边界。

本设计用于解决以下问题：

1. 将脏文件名清洗为可解析的规范影片 ID
2. 将规则从代码中剥离为声明式 YAML
3. 支持目录化规则集与远程规则包
4. 支持在运行时重建 cleaner

## 二、设计目标

本方案的目标是：

1. 使用一套声明式规则统一处理影片 ID 清洗
2. 输出可直接交给 `number.Parse` 使用的规范字符串
3. 提供结构化结果，包括分类、附加标记、候选与命中路径
4. 支持规则目录化拆分
5. 支持本地目录规则集与远程 zip 规则包
6. 支持规则运行时重建
7. 默认主链路不依赖动态脚本执行

## 三、非目标

本方案不做：

1. 不做 AI 猜号
2. 不做网络反查确认
3. 不执行远程 Go/JS/Lua
4. 不把 `number.Parse` 的语义搬入规则集

## 四、总体链路

当前主链路可抽象为：

```text
raw filename
  -> movieidcleaner.Clean / Explain
  -> number.Parse
  -> Searcher.Search
```

其中：

1. `movieidcleaner` 负责清洗、匹配和衍生属性推导
2. `number.Parse` 负责最终结构化语义解析

## 五、核心结果模型

位置：

```text
internal/movieidcleaner/model.go
```

当前 `Result` 已包含：

1. `Normalized`
2. `NumberID`
3. `Suffixes`
4. `Category`
5. `Unrated`
6. `CategoryMatched`
7. `UnratedMatched`
8. `Confidence`
9. `Status`
10. `RuleHits`
11. `Warnings`
12. `Candidates`

这意味着规则集不仅负责“洗字符串”，还负责产生可复用的结构化影片 ID 结果。

## 六、规则集结构

位置：

```text
internal/movieidcleaner/model.go
```

当前 `RuleSet` 包含：

1. `Options`
2. `Normalizers`
3. `RewriteRules`
4. `SuffixRules`
5. `NoiseRules`
6. `Matchers`
7. `PostProcessors`

各项职责如下：

1. `Options`
   1. 定义整个规则集的全局行为，例如大小写策略、空格折叠和无命中时的失败策略。
2. `Normalizers`
   1. 负责在进入正式匹配前做基础字符串清洗。
   2. 适合处理文件路径、扩展名、大小写、全角半角等问题。
   3. 示例：

      ```yaml
      normalizers:
        - name: basename
          type: builtin
          builtin: basename
        - name: strip_ext
          type: builtin
          builtin: strip_ext
      ```

      对输入 `/downloads/ABC-123.mp4`，这两步会先得到 `ABC-123.mp4`，再得到 `ABC-123`。
3. `RewriteRules`
   1. 负责对原始输入做正则改写。
   2. 适合处理稳定且确定的格式变体，例如前缀修正、固定模式归一化。
   3. 示例：

      ```yaml
      rewrite_rules:
        - name: source_a_compact
          pattern: '(?i)SRCA([0-9]+)'
          replace: 'SRCA-$1'
      ```

      对输入 `SRCA123456`，改写后会得到 `SRCA-123456`。
4. `SuffixRules`
   1. 负责识别字幕、多段、高清、全景、特别版等后缀语义。
   2. 这些后缀会被提取、归一化并在后续重新排序。
   3. 示例：

      ```yaml
      suffix_rules:
        - name: subtitle
          type: alias
          aliases: ["SUB", "SUBTITLE"]
          canonical: C
      ```

      对输入 `FILM-123 SUB`，后缀规则会提取出 `SUB`，而不是把这段文本留给 matcher。
5. `NoiseRules`
   1. 负责移除与影片 ID 无关但会干扰匹配的噪声片段。
   2. 适合处理下载站附加标记、说明文字、无关标签。
   3. 示例：

      ```yaml
      noise_rules:
        - name: remove_sample_tag
          type: alias
          aliases: ["SAMPLE", "PREVIEW"]
      ```

      对输入 `SAMPLE ABC-123`，噪声移除后再进入 matcher，会减少误判。
6. `Matchers`
   1. 负责匹配主影片 ID 并生成 `Normalized` 与 `NumberID`。
   2. 同时承担 `category` 和 `flagged` 等衍生属性推导。
   3. 示例：

      ```yaml
      matchers:
        - name: source_a
          category: SOURCE_A
          unrated: true
          pattern: '(?i)SRCA[-_\\s]?([0-9]{3,})'
          normalize_template: 'SRCA-$1'
          score: 100
      ```

      对输入 `srca123456`，matcher 会生成：
      `Normalized = SRCA-123456`
      `NumberID = SRCA-123456`
      `Category = SOURCE_A`
      `Unrated = true`
7. `PostProcessors`
   1. 负责在主匹配完成后做统一后处理。
   2. 适合处理后缀排序、连接符归一化等全局一致性问题。
   3. 示例：

      ```yaml
      post_processors:
        - name: reorder_suffix
          type: builtin
          builtin: reorder_suffix
      ```

      如果前面提取出了多个后缀，例如 `C`、`CD2`、`4K`，后处理阶段会按系统约定顺序输出。

## 七、执行阶段

当前规则执行顺序为：

1. `normalizers`
2. `rewrite_rules`
3. `suffix_rules`
4. `noise_rules`
5. `matchers`
6. `post_processors`
7. 结果契约校验

其中：

1. `normalizers` 负责基础字符串清洗
2. `rewrite_rules` 负责正则改写
3. `suffix_rules` 负责提取与归一化业务后缀
4. `noise_rules` 负责去除噪声
5. `matchers` 负责主影片 ID 匹配、归一化、分类和附加标记属性推导
6. `post_processors` 负责后置统一处理

## 八、规则片段目录

规则片段建议按如下结构组织：

```text
<ruleset-root>/
  001-base.yaml
  002-normalizers.yaml
  003-rewrite_rules.yaml
  004-suffix_rules.yaml
  005-noise_rules.yaml
  006-matchers.yaml
  007-post_processors.yaml
```

这种目录结构体现了两件事：

1. 规则按职责拆分，而不是堆在单文件
2. 文件名前缀只用于组织和排序，不承担覆盖语义

## 九、目录规则合并模型

当前目录加载逻辑位于：

```text
internal/movieidcleaner/loader.go
```

行为如下：

1. 目录内按文件名字典序读取 YAML
2. 每个片段先独立校验
3. 再按字段类型合并到一个 `RuleSet`

合并规则：

1. `version` 必须存在且一致
2. `options` 只能出现一次，或多次出现但内容完全一致
3. 同类型规则名在片段之间不允许重复
4. 目录内部不支持静默覆盖

## 十、Override 合并模型

系统支持在目录或单文件之上继续做规则覆盖。

核心 API：

```go
func MergeRuleSets(base *RuleSet, override *RuleSet) (*RuleSet, error)
```

当前语义：

1. 系统规则集内部不允许同名冲突
2. override 层允许按规则名覆盖系统同名规则
3. 若 override 中某条规则被标记为 `disabled`，则该规则可在最终结果中被移除

这意味着：

1. 规则目录强调“自洽”
2. override 层才具备“覆盖”语义

## 十一、声明式能力边界

当前实现只允许声明式 YAML。

支持的关键能力包括：

### 11.1 Normalizer Builtins

当前支持：

1. `basename`
2. `strip_ext`
3. `fullwidth_to_halfwidth`
4. `trim_space`
5. `collapse_spaces`
6. `to_upper`
7. `replace_pairs`

作用如下：

1. `basename`
   1. 取路径中的基础文件名，去掉目录部分。
   2. 适用于从完整路径收敛到文件名本体。
   3. 示例：

      ```text
      /mnt/media/ABC-123.mp4 -> ABC-123.mp4
      ```
2. `strip_ext`
   1. 去掉文件扩展名。
   2. 适用于把 `ABC-123.mp4` 还原成 `ABC-123`。
   3. 示例：

      ```text
      ABC-123.mp4 -> ABC-123
      ```
3. `fullwidth_to_halfwidth`
   1. 将全角字符转换为半角字符。
   2. 适用于处理中日文环境下常见的全角数字、字母和符号。
   3. 示例：

      ```text
      ＡＢＣ－１２３ -> ABC-123
      ```
4. `trim_space`
   1. 去除首尾空白字符。
   2. 适用于清理输入边缘噪声。
   3. 示例：

      ```text
      "  ABC-123  " -> "ABC-123"
      ```
5. `collapse_spaces`
   1. 将连续空白折叠为单个空格。
   2. 适用于稳定后续 rewrite 与 matcher 的输入形态。
   3. 示例：

      ```text
      "FILM   123" -> "FILM 123"
      ```
6. `to_upper`
   1. 将输入统一转为大写。
   2. 适用于让影片 ID 匹配大小写无关且输出稳定。
   3. 示例：

      ```text
      film-123 -> FILM-123
      ```
7. `replace_pairs`
   1. 按配置表批量替换字符或片段。
   2. 适用于做静态映射式归一化，而不必写正则 rewrite。
   3. 示例：

      ```yaml
      normalizers:
        - name: replace_misc
          type: builtin
          builtin: replace_pairs
          pairs:
            "_": "-"
            "—": "-"
      ```

      对输入 `FILM_123`，会先归一为 `FILM-123`。

### 11.2 Rewrite Rules

`RewriteRule` 的作用是：

1. 用正则把稳定的输入变体改写成更接近标准影片 ID 的形式。
2. 它介于基础清洗和正式匹配之间。
3. 适合处理“可以安全重写”的模式，不适合做含糊猜测。

### 11.3 Suffix Rules

`SuffixRule` 的作用是：

1. 从输入中提取后缀语义，而不是把它们留在主影片 ID 里参与匹配。
2. 将不同写法归一成统一后缀，例如字幕、多段版本、清晰度、全景版、特别版等。
3. 为最终 `Suffixes` 和 `Normalized` 输出提供稳定来源。

### 11.4 Noise Rules

`NoiseRule` 的作用是：

1. 移除无助于主影片 ID 识别的噪声片段。
2. 减少 matcher 被无关信息干扰的概率。
3. 它不负责生成语义，只负责删噪。

### 11.5 Post Processor Builtins

当前支持：

1. `reorder_suffix`
2. `normalize_hyphen`

作用如下：

1. `reorder_suffix`
   1. 按统一规则重排后缀链。
   2. 目的是让输出顺序稳定，便于 `number.Parse`、日志和测试比较。
   3. 示例：

      ```text
      FILM-123-UHD-SUB-DISC2 -> FILM-123-SUB-DISC2-UHD
      ```
2. `normalize_hyphen`
   1. 统一连接符写法。
   2. 适用于把不同破折号、下划线或杂散分隔符收敛到系统约定格式。
   3. 示例：

      ```text
      ABC_123 -> ABC-123
      ```

### 11.6 Matchers

`MatcherRule` 是整个规则集的核心，作用是：

1. 在清洗后的输入中识别主影片 ID。
2. 通过 `normalize_template` 生成规范化输出。
3. 通过 `score` 控制候选优先级。
4. 通过 `category` 生成分类信息。
5. 通过 `unrated` 生成附加布尔标记。
6. 通过 `require_boundary` 与 `prefixes` 控制匹配边界和适用前缀。

示例：

```yaml
matchers:
  - name: generic_code
    pattern: '(?i)([A-Z]{2,6})[-_\\s]?([0-9]{2,6})'
    normalize_template: '$1-$2'
    score: 80
    require_boundary: true
```

对输入：

```text
FILM-123 sample
```

该 matcher 可以稳定命中 `FILM-123`。

但对于：

```text
XFILM123Y
```

当 `require_boundary: true` 时，系统会更保守，不会轻易把中间这段连续子串识别成独立影片 ID；如果改成 `false`，则允许更激进地从长字符串内部抽取候选。

`MatcherRule` 当前已支持：

1. `category`
2. `unrated`
3. `pattern`
4. `normalize_template`
5. `score`
6. `require_boundary`
7. `prefixes`

这使得分类与附加标记判断已内聚到规则集本身，不再依赖额外脚本系统。

## 十二、Explain 与可观察性

当前 cleaner 不只提供 `Clean`，还提供：

```go
Explain(input string) (*ExplainResult, error)
```

`ExplainResult` 会记录：

1. 每个阶段
2. 命中规则
3. 输入输出变化
4. 候选项
5. 最终结果

这使得问题定位与调试能力显著强于简单的字符串改写链。

## 十三、远程规则包

当前远程规则包模型已经落地，位置：

```text
internal/movieidcleaner/bundle.go
internal/bundle/manager.go
```

能力包括：

1. 本地目录规则包
2. 远程 zip 规则包
3. zip 缓存
4. 直接读取 zip，不解压
5. `manifest.yaml + entry` 模型
6. callback + `Start(ctx)` 生命周期

## 十四、规则包结构

本地目录与远程 zip 使用同一结构：

```text
<bundle-root>/
  manifest.yaml
  <entry>/
    *.yaml
```

对规则集来说，默认入口通常是：

```yaml
entry: ruleset
```

## 十五、规则包生命周期

通用 bundle manager 使用 callback 模型。

语义：

1. 首次 `Start(ctx)` 会完成初始化加载
2. 远程 source 会在后台继续同步
3. 新数据只有在 callback 成功后才算激活成功
4. `movieidcleaner` callback 中会重建 cleaner 并替换运行时实例

因此规则集当前已经支持真正的 runtime 重建。

## 十六、当前实现结构

### 16.1 数据模型与执行器

```text
internal/movieidcleaner/model.go
internal/movieidcleaner/cleaner.go
internal/movieidcleaner/loader.go
internal/movieidcleaner/runtime.go
```

### 16.2 规则包与远程加载

```text
internal/movieidcleaner/bundle.go
internal/bundle/manager.go
```

### 16.3 规则片段目录

规则片段以目录化 YAML 组织，并通过 bundle 的 `entry` 指向对应目录。

## 十七、与配置层的边界

当前原则是：

1. `internal/config` 只在 `cmd` 层使用
2. `movieidcleaner` 包不直接依赖 `internal/config`
3. `cmd` 层负责把配置转换为 cleaner/bundle 所需的内部参数

## 十八、结论

当前 `movieid-ruleset` 系统已经具备完整实现，包含：

1. 结构化规则模型
2. 目录化规则片段
3. override 合并
4. explain 能力
5. 本地 bundle
6. 远程 zip bundle
7. 运行时 cleaner 重建

因此该设计已可作为正式文档归档。
