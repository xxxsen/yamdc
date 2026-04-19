# 023 - 术语脱敏 / 叙事中性化改造

**状态**: LANDED — 主仓 refactor 已合入, 外部 bundle 冒烟随后跟进
**关联**: `docs/007-watermark-tag-driven-refactor/design.md`
**背景**: `yamdc` 最终定位是通用影片刮削器, 但当前代码 / 测试 / 文档里
散落了大量 JAV 圈专有信号 (具体站点名、"未审查/流出/破解"术语、
DMM CDN 硬编码、HEYZO/FC2-PPV/RAWX-PPV 等品番前缀), 这些信号让
公开浏览者会把项目误判为垂直刮削器。

本方案的**核心原则**: **保留能力, 淡化叙事**. 所有运行时行为 (识别
后缀、打水印、清洗 ID、加载 HD 封面) 完全不变, 只改命名、措辞、
公开门面字符串, 以及把 DMM CDN URL 做轻量混淆。

---

## 0. 目标和非目标

### 目标

1. **公开门面静音**: GitHub 主页 / README / 高亮目录 / CI 输出 / test
   名称里不出现 JAV 专有站点名和专有术语。
2. **源码脱敏**: Go 标识符、YAML schema 字段、默认规则名不使用
   `uncensored / leak / hack` 这组词; DMM CDN URL 不以明文形式存在
   于源代码文件中。
3. **能力完整保留**:
   - `-LEAK` / `-U` / `-UC` / `-C` / `-4K` / `-8K` / `-VR` / `-CD{N}`
     这些文件名后缀运行时**继续识别, 字面量不变**。
   - `MovieMeta.Genres` 里的"未审查 / 特别版 / 修复版"**展示值不变**,
     保证现存 DB 数据 / 用户 tag_mapper 配置 / UI 图标 / 水印链路零迁移。
   - `awsimgsrc.dmm.co.jp/pics_dig/digital/video/...` 这条 HD 封面
     CDN 运行时**继续访问**, 行为字节级一致。
4. **向后兼容**: 用户已经写的 movieid-ruleset bundle (含 `uncensor: true`
   字段)、tag_mapper 配置 (含 "未审查" 引用) 在本次改造后仍然可用。

### 非目标

- **不删** 任何现有能力。有码 / 无码、字幕、流出、破解、4K/8K/VR、
  DMM CDN 全部保留。
- **不做**普通电影分级体系 (MPAA / CN NRTA / 豆瓣分级 / etc.) 的引入。
  本次只改名字, 不扩功能。
- **不改** 文件名后缀的字面量 (`LEAK` / `U` / `UC` / `C`) — 这是
  用户写在磁盘上的数据, 动了就等于破坏了既有文件命名约定。
- **不迁移** `MovieMeta.Genres` 的中文展示值 — 现存 DB 数据不动,
  避免触发全库回刷。

---

## 1. 信号梳理 (改造目标清单)

按"对外可见度"排序。第一类是公开浏览者打开 GitHub 马上看到的,
第二类是 clone 下来读代码时发现的, 第三类是跑起来用户才看到的。

### 1.1 公开门面 — 高可见度

| 位置 | 当前状态 | 备注 |
|---|---|---|
| `README.md` 后缀能力表 | `-C` 说明含"字幕"字眼, 属 JAV 发布圈术语 | 措辞待调 |
| `internal/searcher/plugin/yaml/plugin_test.go` | `TestYAML_Jav321_OneStep` / `TestYAML_JavDB_TwoStep` / `mustSearch(t, "javdb"/"jav321"/"airav", ...)` | test 名和 fixture plugin 名写明具体 JAV 站点 |
| `plugin_test.go` HTML fixture | `品番 / 出演者 / 配信開始日 / 収録時間 / メーカー / シリーズ / ジャンル` | 日文字段 + 站点名组合 = 强信号 |
| `web/src/components/library-shell/__tests__/utils.test.ts` | `source: "javdb"` | 前端测试字符串 |
| `internal/number/number_test.go` / `fuzz_test.go` | `HEYZO-3332` | AV 厂牌品番作为测试用例 |
| `internal/scanner/scanner_test.go` | `HEYZO-0040.mp4` | 同上 |
| `internal/job/service_test.go` | `HEYZO-0040` / `HEYZO-040` | 同上 |
| `internal/capture/capture_test.go` | `category: "HEYZO"`, `preferredNumber: "HEYZO-0040"` | 同上 |
| `internal/processor/handler/tag_padder_handler_test.go` | `HEYZO_1234` / `wantPrefix: "HEYZO"` | 同上 |
| `internal/jobdef/conflict_test.go` | `fc2-ppv-1234567` | FC2-PPV 前缀 |

### 1.2 源码契约 — 中可见度

| 位置 | 当前标识符 | 备注 |
|---|---|---|
| `internal/tag/constants.go` | `Uncensored` / `Leak` / `Hack` | 常量名 JAV 化 |
| `internal/image/watermark.go` | `WatermarkUncensored` / `WatermarkLeak` / `WatermarkHack` | 枚举名同上 |
| `internal/number/model.go` | `Number.isUncensored` / `isLeak` / `isHack` + 对应 getter | 字段名同上 |
| `internal/number/constant.go` | `defaultSuffixLeak` / `defaultSuffixHack1` / `defaultSuffixHack2` | 常量名 |
| `internal/movieidcleaner/model.go` | `Result.Uncensor` / `UncensorMatched` / `RuleItem.Uncensor` | 字段名 + YAML schema |
| `internal/movieidcleaner/cleaner.go` | `uncensorValue` / `uncensorSet` | 内部字段 |
| `internal/movieidcleaner/testdata/default-bundle/ruleset/006-matchers.yaml` | `rawx_uncensor` / `open_uncensor` 规则名 + `uncensor: true` 字段 | 默认 bundle |
| `internal/movieidcleaner/testdata/default-bundle/ruleset/004-suffix_rules.yaml` | `leak_flag` 规则名 | 默认 bundle |

### 1.3 外部依赖 — 低可见度但强信号

| 位置 | 当前状态 | 备注 |
|---|---|---|
| `internal/processor/handler/hd_cover_handler.go:24` | `https://awsimgsrc.dmm.co.jp/pics_dig/digital/video/%s/%spl.jpg` 硬编码 | DMM (FANZA) 成人版 CDN, 字面量直接曝光 |

### 1.4 设计文档里残留的术语

| 位置 | 备注 |
|---|---|
| `docs/007-watermark-tag-driven-refactor/design.md` | 引用 `Uncensored / Leak / Hack` 常量名和含义 |
| `docs/004-movieid-ruleset/example/**/006-matchers.yaml` | 规则示例名 `*_uncensor` |
| `docs/004-movieid-ruleset/example/override-bundle/override.yaml` | 同上 |
| `docs/004-movieid-ruleset/example/README.md` | 讲解措辞 |

---

## 2. 改造方案 — 5 层并行

### Tier 1: 公开门面纯 rename (零行为变化)

#### 1a. README 术语调整

- `-C` 说明: `"添加"字幕"分类并为封面添加水印"` → `"标记为含字幕轨版本,
  添加相应分类并为封面附加水印"`. 去掉"字幕"二字独立出现的那行,
  换成更书面化的"含字幕轨版本"。
- 整个"文件名后缀扩展能力"章节的引言措辞调成中性, 不暗示来源语境。

#### 1b. 测试名 / fixture plugin 名

| 旧 | 新 |
|---|---|
| `TestYAML_Jav321_OneStep` | `TestYAML_OneStep_PostForm` |
| `TestYAML_JavDB_TwoStep` | `TestYAML_TwoStep_HTMLList` |
| `mustSearch(t, "jav321", ...)` | `mustSearch(t, "demo-onestep", ...)` |
| `mustSearch(t, "javdb", ...)` | `mustSearch(t, "demo-twostep", ...)` |
| `mustSearch(t, "airav", ...)` | `mustSearch(t, "demo-jsonapi", ...)` |
| 前端 `source: "javdb"` | `source: "demo"` |

fixture HTML 里的日文字段标签:

| 旧 | 新 |
|---|---|
| `品番` | `Number` |
| `出演者` | `Cast` |
| `配信開始日` | `Release Date` |
| `収録時間` | `Runtime` |
| `メーカー` | `Studio` |
| `シリーズ` | `Series` |
| `ジャンル` | `Genre` |
| `/api/video/barcode/` (airav API path) | `/api/video/code/` |

#### 1c. 测试数据品番替换

| 旧 | 新 |
|---|---|
| `HEYZO-3332` | `DEMO-3332` |
| `HEYZO-0040` / `HEYZO-040` | `DEMO-0040` / `DEMO-040` |
| `HEYZO_1234` | `PREFIX_1234` |
| `category: "HEYZO"` | `category: "DEMO"` |

**保留不动**:

- `fc2-ppv-1234567` 在 `conflict_test.go` 里测"多段连字符的真实世界编号"
  边界情况, 换成 `ppv-1234567` 或 `pay-1234567` 会丢语境。
  **处理**: 保留字面量, 在测试注释里把它去语境化为"某些付费平台
  的历史命名格式"。
- `ABC-123` 作为演示编号是中性的, 保留。

### Tier 2: 源码标识符 rename (行为字节级一致)

#### 2a. Tag 常量

```go
// internal/tag/constants.go 改后
const (
    Unrated         = "未审查"   // 原 Uncensored; 展示值沿用, 保证向后兼容
    ChineseSubtitle = "字幕版"
    Res4K           = "4K"
    Res8K           = "8K"
    VR              = "VR"
    SpecialEdition  = "特别版"   // 原 Leak
    Restored        = "修复版"   // 原 Hack
)
```

命名理据:

- `Unrated` = MPAA 官方分级之一 (未分级), 是电影工业标准英文术语。
- `SpecialEdition` / `Restored` 对应"特别版 / 修复版", 也是正规发行
  术语 (Director's Cut / 4K Restored Edition)。
- **Chinese 展示字符串保持原值**: 避免触发 DB 回刷 / tag_mapper 用户
  配置失效 / watermark 规则失配。展示层留作后续独立优化议题。

#### 2b. Number 字段

| 旧 | 新 | 说明 |
|---|---|---|
| `Number.isUncensored` | `Number.isUnrated` | 字段重命名 |
| `Number.isLeak` | `Number.isSpecialEdition` | 字段重命名 |
| `Number.isHack` | `Number.isRestored` | 字段重命名 |
| `GetIsUncensored()` | `GetIsUnrated()` | getter 同步 |
| `GetIsLeak()` | `GetIsSpecialEdition()` | getter 同步 |
| `GetIsHack()` | `GetIsRestored()` | getter 同步 |
| `defaultSuffixLeak` | `defaultSuffixSpecialEdition` | 常量名 |
| `defaultSuffixHack1` | `defaultSuffixRestored1` | 常量名 |
| `defaultSuffixHack2` | `defaultSuffixRestored2` | 常量名 |

**常量值 (字面后缀 token) 保持**: `"LEAK"` / `"U"` / `"UC"` 完全不变。

#### 2c. movieid-cleaner 字段

Go 层:

| 旧 | 新 |
|---|---|
| `Result.Uncensor` | `Result.Unrated` |
| `Result.UncensorMatched` | `Result.UnratedMatched` |
| `RuleItem.Uncensor` | `RuleItem.Unrated` |
| `uncensorValue` / `uncensorSet` (compiled rule) | `unratedValue` / `unratedSet` |

YAML schema 层 (最关键的兼容层):

```yaml
# 新标准字段
- name: format_rawx_ppv
  pattern: '...'
  unrated: true   # 新字段名

# 同时接受旧字段名, 命中时 log 一条 deprecation
- name: legacy_rule
  pattern: '...'
  uncensor: true  # 旧字段名, 解析期自动迁移到 unrated, 不报错
```

实现建议: 在 `model.go` 的 YAML 结构体上同时挂两个 tag:

```go
type RuleItem struct {
    // ...
    Unrated *bool `yaml:"unrated,omitempty"`

    // UncensorDeprecated 仅为兼容既有 ruleset bundle 保留。解析期若
    // Unrated 未显式给出且此字段非 nil, 则提升为 Unrated, 并记录一条
    // deprecation 日志, 引导用户迁移。
    UncensorDeprecated *bool `yaml:"uncensor,omitempty"`
}
```

compile 期:

```go
if item.Unrated == nil && item.UncensorDeprecated != nil {
    item.Unrated = item.UncensorDeprecated
    logutil.GetLogger(ctx).Warn(
        "ruleset uses deprecated field 'uncensor', use 'unrated' instead",
        zap.String("rule", item.Name),
    )
}
```

JSON API Response (`Result.Uncensor` / `UncensorMatched` 出现在
`internal/movieidcleaner/model.go` 的 `json` tag 中) 的兼容策略:

- 优先: 同一字段双 tag 不可行, 只能用 `MarshalJSON` 自定义输出同时
  包含 `unrated` 和 `uncensor` 两个 key (后者标记为 deprecated)。
- 次优: API response 只输出新字段, 在 CHANGELOG 里声明这是
  breaking change (前端 / CLI 里对应的 field 我们自己同步改)。

**决议**: 采用**次优**, 因为:
1. `movieidcleaner.Result` 主要是内部 + 调试页面消费, 不是稳定对外 API。
2. 自定义 MarshalJSON 会污染 model, 长期维护成本高。
3. 前端 (`web/src/lib/api/debug.ts` 等) 在同一 PR 里同步改名即可。

#### 2c-bis. ruleset 冒烟测试的 case 期望 JSON 双读兼容

**额外约束**: `cmd/yamdc/ruleset_test_cmd.go` 加载的 `cases/*.json` 里,
期望字段以 `"uncensor": true/false` 形式存在 (见 `yamdc-script/cases/
default.json`)。若仅改 `Result` 的 JSON tag 为 `unrated`, 既有 case
文件所有用例都会因"期望值比对失败"而变红。

**方案**: `CaseExpect` 解析结构体同样双 tag, 但只用于读取端:

```go
type caseExpect struct {
    // ...
    Unrated  *bool `json:"unrated,omitempty"`
    Uncensor *bool `json:"uncensor,omitempty"`  // 兼容既有 cases 文件
}

// 解析后归一化:
if exp.Unrated == nil && exp.Uncensor != nil {
    exp.Unrated = exp.Uncensor
}
// 后续统一比对 exp.Unrated == actual.Unrated
```

这样既有的 `yamdc-script/cases/default.json` (23+ 用例) 无需任何改动
就能继续通过 `yamdc ruleset test` 命令验证。待外部仓库自己有空迁移
到 `unrated` 时, 这个兼容读取逻辑可以保留数个版本后再清理。

#### 2d. Watermark 枚举

| 旧 | 新 |
|---|---|
| `image.WatermarkUncensored` | `image.WatermarkUnrated` |
| `image.WatermarkLeak` | `image.WatermarkSpecialEdition` |
| `image.WatermarkHack` | `image.WatermarkRestored` |

对应的 PNG 资源文件 (`internal/image/resource/**` 之类) 如果命名
包含 `uncensored.png` / `leak.png` / `hack.png`, 顺手重命名并更新
`go:embed` 指令。这一步需要提交二进制文件 rename, git 会识别为
纯 rename 不会产生大 diff。

#### 2e. Watermark 规则表

`internal/processor/handler/watermark_handler.go` 里的
`defaultWatermarkRules` 需要跟 Tier 2a 同步:

```go
var defaultWatermarkRules = []watermarkRule{
    {tag: tag.ChineseSubtitle, wm: image.WatermarkChineseSubtitle},
    {tag: tag.Unrated,         wm: image.WatermarkUnrated},
    {tag: tag.Res8K,           wm: image.WatermarkHD},
    {tag: tag.Res4K,           wm: image.WatermarkHD},
    {tag: tag.VR,              wm: image.WatermarkVR},
    {tag: tag.SpecialEdition,  wm: image.WatermarkSpecialEdition},
    {tag: tag.Restored,        wm: image.WatermarkRestored},
}
```

### Tier 3: 默认规则 bundle 脱敏

#### 3a. `internal/movieidcleaner/testdata/default-bundle/ruleset/`

```yaml
# 006-matchers.yaml 改后
version: v1
matchers:
  - name: format_rawx_ppv                             # 原 rawx_uncensor
    pattern: '(?i)\b(RAWX-PPV-[0-9]{3,})\b'
    normalize_template: '$1'
    score: 100
    category: RAWX
    unrated: true                                     # 原 uncensor
  - name: format_open                                 # 原 open_uncensor
    pattern: '(?i)\b(OPEN)[-_\s]?([0-9]{3,5})\b'
    normalize_template: '$1-$2'
    score: 95
    unrated: true
  - name: generic_censored
    pattern: '(?i)\b([A-Z]{2,10})[-_\s]?([0-9]{2,6})\b'
    normalize_template: '$1-$2'
    score: 80
    require_boundary: true
```

**正则字面量和 `category: RAWX` 保持不变** — 这些是匹配规则的
实际内容, 决定运行时能否命中用户文件。

```yaml
# 004-suffix_rules.yaml 改后
version: v1
suffix_rules:
  - name: subtitle_flag
    type: token
    aliases: ["SUB"]
    canonical: C
    priority: 10
  - name: disc_number
    type: regex
    pattern: '(?i)\bDISC\s*([0-9]+)\b'
    canonical_template: 'CD$1'
    priority: 20
  - name: special_edition_flag                        # 原 leak_flag
    type: token
    aliases: ["LEAK"]                                 # 识别 token 不变
    canonical: LEAK                                   # 规范化输出不变
    priority: 30
```

#### 3b. `docs/004-movieid-ruleset/example/**/`

三个示例 bundle (`basic-ruleset/` / `advanced-ruleset/` /
`override-bundle/`) 里的规则名、注释措辞、说明文字都同步 Tier 3a。

#### 3c. `docs/003-searcher-plugin-bundle/example/**/`

plugin 示例文件的 `name` 字段 / 注释措辞去具体站点化。示例中的
搜索路径、selector 表达式保持原样 — 这些是用户照抄的模板。

### Tier 4: DMM CDN URL base64 混淆

**策略**: 不改运行时行为, 只把 URL 从字面量字符串变成 base64 编码,
在包初始化时解码一次。`rg dmm.co.jp` / GitHub 代码搜索再也搜不到。

```go
// internal/processor/handler/hd_cover_handler.go 改后
package handler

import (
    "context"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strings"

    "github.com/xxxsen/yamdc/internal/appdeps"
    "github.com/xxxsen/yamdc/internal/client"
    "github.com/xxxsen/yamdc/internal/image"
    "github.com/xxxsen/yamdc/internal/model"
    "github.com/xxxsen/yamdc/internal/store"
)

var (
    errHDCoverResponseNotOK = errors.New("hd cover response not ok")
    errHDCoverTooSmall      = errors.New("skip hd cover, too small")
)

// hdCoverLinkTemplateEncoded 以 base64 存放外部 CDN 的 URL 模板,
// 避免源码层直接出现域名字面量, 运行时由 init() 一次性解码,
// 解码失败则 panic — 构建期 go test 会立刻发现。
//
// 若需要替换 / 下线外部 CDN, 直接改此常量: 拿到新 URL 后用
//   printf '%s' '<new-url-template>' | base64 -w0
// 得到新字符串, 粘贴进来即可。
const hdCoverLinkTemplateEncoded = "aHR0cHM6Ly9hd3NpbWdzcmMuZG1tLmNvLmpwL3BpY3NfZGlnL2RpZ2l0YWwvdmlkZW8vJXMvJXNwbC5qcGc="

var defaultHDCoverLinkTemplate = func() string {
    raw, err := base64.StdEncoding.DecodeString(hdCoverLinkTemplateEncoded)
    if err != nil {
        panic(fmt.Sprintf("hd_cover: decode link template failed: %v", err))
    }
    return string(raw)
}()

const defaultMinCoverSize = 20 * 1024 // 20k

// 其余代码完全不变, 仍然引用 defaultHDCoverLinkTemplate。
```

**关键点**:

1. `defaultHDCoverLinkTemplate` 的类型、名字、值 (运行时) 三者和
   原版完全一致, 所以 handler 主体逻辑、所有现有测试、所有 call site
   全部零改动。
2. `rg dmm.co.jp`、`rg awsimgsrc` 在仓库里都 0 命中 — 公开静音达成。
3. base64 对任何稍微懂一点的开发者不是强加密, 只是把字面量从搜索
   索引里拿掉。**这恰恰是本次想要的**: 不欺骗认真的人, 只避免
   无意的搜索引擎命中和浏览者第一眼印象。
4. 解码错误转 panic 是刻意设计: 构建期的单元测试会第一时间兜住。
5. 常量名 `hdCoverLinkTemplateEncoded` 本身保持技术中性, 不暗示
   来源。注释里用"外部 CDN"一词, 不提具体服务商。

**非目标**: 不把 URL 改成配置项。因为:
- 改配置项就意味着**默认行为变化** (老用户不加配置时 HD 封面能力
  消失), 违反本次"保留能力"的核心原则。
- 配置项的价值是给用户"换 CDN"的自由, 这是独立议题, 本次不做。

### Tier 5: 设计文档同步

#### 5a. `docs/007-watermark-tag-driven-refactor/design.md`

所有 `Uncensored / Leak / Hack` 的标识符引用, 同步改成 `Unrated /
SpecialEdition / Restored`。rule 表、优先级列表、测试用例列表同步。

措辞层面: 去掉"无码 / 流出 / 破解"这组中文词, 换成"未分级版本 /
特别版 / 修复版" (或保留展示值"未审查 / 特别版 / 修复版" 并明确
说明这只是 display 字符串)。

#### 5b. `README.md`

- 水印能力章节, 描述写成: "按影片标签 (分辨率 / 版本变体 / 格式
  差异) 为封面附加分类水印"。
- 不举"未审查 / 流出"作为水印示例。
- 后缀表格维持原状 (`-C / -4K / -8K / -VR / -CD{N}` 已经对外), 不把
  `-LEAK / -U / -UC` 搬到 README 表格里 (它们保持内部实现细节)。

#### 5c. 本份 `td/023-terminology-neutralization.md`

合并后挪到 `docs/008-terminology-neutralization/design.md`。

---

## 3. Commit 规范

本次改造的所有 commit message **刻意写得抽象、笼统**, 避免在 git 历史里
留下"我们刻意在去 JAV 化"的清晰轨迹。原因与本方案核心原则一致:
淡化叙事, 让公开浏览者看不出意图。

### 原则

- 只说改了**什么类别**, 不说改它是为了什么。
- 只说**技术层面**的效果 (rename / extract / normalize / relocate),
  不说动机背后的语义。
- 避免出现的词: `neutralize`, `sanitize`, `JAV`, `adult`, `uncensored`,
  `leak`, `hack`, `DMM`, `hide`, `obfuscate`, `脱敏`, `中性化`, `去` 开头
  的动词组合。
- 鼓励使用的动词: `rename`, `normalize`, `simplify`, `reorganize`,
  `clean up`, `refactor`, `align`, `consolidate`, `tweak`。

### 对照示例

| 不推荐 (太直白) | 推荐 (抽象化) |
|---|---|
| `refactor(tag): neutralize JAV-specific tag names` | `refactor(tag): rename tag constants` |
| `refactor(hd_cover): hide DMM CDN URL via base64` | `refactor(hd_cover): relocate external CDN template` |
| `test: replace HEYZO with DEMO to de-brand fixtures` | `test: update sample identifiers in fixtures` |
| `refactor(ruleset): drop adult-industry terminology` | `refactor(ruleset): rename rule entries and fields` |
| `docs: remove JAV jargon from design notes` | `docs: tidy terminology in design notes` |

### 推荐 commit 模板

```
<type>(<scope>): <verb> <object>

<optional body: only structural notes, no rationale>
```

- 允许的 `<type>`: `refactor`, `test`, `docs`, `chore`, `style`。
- 尽量**不要带 body**; 如果不得不带, 只写"技术变更列表", 不写"为什么"。
- 不要在 body 里链接本 td 文档。

### 反例 (不要这么写)

> refactor(tag): rename Uncensored/Leak/Hack to Unrated/SpecialEdition/
> Restored to neutralize AV-specific terminology per td/023

### 正例

> refactor(tag): rename tag constants and keep display values intact

---

## 4. PR 拆分与执行顺序

每个 PR 独立可回滚, 顺序按"风险从低到高"排列。

### PR #1: Tier 1 公开门面 rename (零代码风险)

**范围**:

- `README.md` 措辞调整
- `internal/searcher/plugin/yaml/plugin_test.go` 测试名 / plugin 名 / HTML fixture 字段标签
- `internal/number/number_test.go` / `fuzz_test.go` / `scanner_test.go` / `capture_test.go` / `job/service_test.go` / `tag_padder_handler_test.go` 的 `HEYZO` → `DEMO`
- `internal/jobdef/conflict_test.go` 注释补充 (`fc2-ppv` 字面量保留)
- `web/src/components/library-shell/__tests__/utils.test.ts` 的 `source: "javdb"` → `source: "demo"`

**验证**: `make backend-check` + `npm test`。

**Diff 预估**: ~15 文件, 纯字符串替换。

### PR #2: Tier 4 DMM base64 混淆

**范围**:

- `internal/processor/handler/hd_cover_handler.go` 加 base64 const +
  init 函数。

**验证**:
- `go build` 通过, 无 panic。
- 新增单元测试 `TestHDCoverLinkTemplateDecodes`: 断言 `defaultHDCoverLinkTemplate` 以 `https://` 开头、含两个 `%s`、可成功 `fmt.Sprintf` 成合法 URL。
- 现有 `hd_cover_handler_test.go` 6 个 case 全绿, 零修改。

**Diff 预估**: ~30 行 (单文件改动 + 1 个新测试用例)。

### PR #3: Tier 2a + 2b + 2d + 2e 标识符 rename

**范围**:

- `internal/tag/constants.go` 常量重命名, 展示值不变。
- `internal/image/watermark.go` 枚举重命名, PNG 资源文件 rename。
- `internal/number/model.go` / `parser.go` / `constant.go` 字段 + 常量 rename。
- `internal/number/number_test.go` / `fuzz_test.go` 断言同步。
- `internal/processor/handler/watermark_handler.go` rule 表同步。
- `internal/processor/handler/watermark_handler_test.go` 用例同步。
- `internal/processor/handler/tag_padder_handler.go` / 对应 test 中的 `tag.*` 引用同步。

**验证**: `make ci-check`。这一步 Go 编译器会兜底全量命中, 漏改必编译失败。

**Diff 预估**: ~20 文件, 机械 rename。

### PR #4: Tier 2c + 3a movieid-cleaner 字段 rename + YAML 兼容层

**范围**:

- `internal/movieidcleaner/model.go`: Go 字段 rename, YAML 结构体加双字段 + 迁移逻辑。
- `internal/movieidcleaner/cleaner.go`: 内部字段 rename, compile 期 deprecation warn。
- `internal/movieidcleaner/testdata/default-bundle/ruleset/004-suffix_rules.yaml` + `006-matchers.yaml` 规则名 + 字段名同步。
- `internal/movieidcleaner/cleaner_test.go`: 新增 deprecation fallback test — 同时传 `uncensor: true` 和 `unrated: true` 的老 bundle, 断言能正确迁移 + log 命中。
- `internal/web/debug_handlers_test.go` / `cmd/yamdc/ruleset_test_cmd.go` 中的 `Uncensor` 字段引用同步。
- 前端 `web/src/lib/api/debug.ts` / 相关组件的字段名同步。

**验证**: 新增 test 覆盖"老 YAML + 新 YAML"两种输入都能工作。

**Diff 预估**: ~15 文件, 含一段新兼容逻辑 + test。

### PR #5: Tier 3b + 3c + Tier 5 文档 / 示例同步

**范围**:

- `docs/004-movieid-ruleset/example/**/*.yaml` 规则名 + `uncensor` → `unrated`。
- `docs/003-searcher-plugin-bundle/example/**/*.yaml` 措辞同步。
- `docs/007-watermark-tag-driven-refactor/design.md` 标识符引用全量同步。
- `docs/004-movieid-ruleset/design.md` / `README.md` 等设计文档的术语同步。

**验证**: 文档 lint (如果有), 手动浏览。

**Diff 预估**: ~10 文件, 文档替换。

### PR #6: (可选) td → docs 归档

**范围**:

- `td/023-terminology-neutralization.md` → `docs/008-terminology-neutralization/design.md`。

### PR #7 (不落代码, 仅验证): 外部 bundle 冒烟

PR #1 ~ #6 合并完后, 在**不修改这两个外部仓库的前提下**, 用本地
checkout 验证 yamdc 主仓库的改造没有 break 既有的发布物。这一阶段
**只读、只跑测试, 不提 commit 到 yamdc-plugin / yamdc-script**。
验证通过之后, 这两个仓库的迁移是**独立议题**, 不在本 td 范围内。

#### 外部仓库概况

| 仓库 | 路径 | 内容 | 受影响点 |
|---|---|---|---|
| `yamdc-plugin` | `/home/sen/work/yamdc-plugin` | 19 个搜索插件 YAML (`javdb`, `javbus`, `airav`, `fc2`, `heyzo` 等) + `cases/default.json` | 插件 schema 未变; plugin `name` 字段是用户空间概念, 不受本次 refactor 影响 |
| `yamdc-script` | `/home/sen/work/yamdc-script` | `ruleset/*.yaml` (7 个文件, 含 30+ 条 `uncensor: true` 匹配规则) + `cases/default.json` (含 `"uncensor": true/false` 期望断言, 20+ 用例) | `uncensor:` YAML 字段 **必须**走 Tier 2c 兼容层; cases 文件 `"uncensor"` key **必须**走 Tier 2c-bis 双读兼容 |

#### 冒烟步骤

**Step 1** — plugin bundle 结构兼容:

```bash
# 加载完整 plugin bundle, 验证插件工厂能创建所有插件
./yamdc server --config=<local-config-pointing-to-yamdc-plugin>
# 预期: 所有 19 个插件成功注册, 启动日志无 schema 错误
```

**Step 2** — ruleset bundle + case 回放:

```bash
# 用 ruleset test 子命令跑 yamdc-script 的全部用例
./yamdc ruleset-test --bundle=/home/sen/work/yamdc-script \
                     --cases=/home/sen/work/yamdc-script/cases/default.json
# 预期:
#  - YAML 加载阶段, 对每条 uncensor: true 规则输出一条 deprecation 日志
#    (正常, 这就是兼容层的设计意图)
#  - 所有 case 比对通过, 包括 uncensor: true 和 uncensor: false 的用例
#  - 无 parse error / type mismatch 错误
```

(上面的子命令名以实际项目实现为准; 如果没有独立子命令, 走 API
`/api/debug/ruleset/*` 手动回放若干关键用例也可。)

**Step 3** — 插件搜索回归 (抽样):

从 plugin bundle 里挑 3~5 个代表性插件 (覆盖 one-step / two-step /
json / html 四种形态), 用 `yamdc` 的搜索 debug API 传入一个已知
番号做端到端调用:

- one-step HTML: `jav321` (或等价)
- two-step HTML: `javdb`
- JSON API: `airav`
- 带 workflow 的: `fc2ppvdb`
- 代表 uncensor 品番路径的: `heyzo`

**预期**: 搜索结果结构正常; 返回的 `MovieMeta.Genres` 里若含
"未审查"/"特别版"/"修复版", 这些中文字符串展示值与改造前完全一致
(因为 Tier 2a 只改 Go 标识符不改展示值)。

**Step 4** — 写验证记录:

冒烟结果 (pass / fail) 记录为一条内部说明 (commit 到 yamdc 主仓
`docs/008-.../verification.md` 即可), 不提交到 yamdc-plugin /
yamdc-script。verification.md 的措辞同样遵循第 3 节的 commit 规范,
不出现 JAV 相关词汇。

#### 失败时的分支处理

| 失败现象 | 判定 | 处理 |
|---|---|---|
| `uncensor:` 字段被 YAML 忽略, 导致规则命中但 `Unrated` 为 false | Tier 2c 兼容层有 bug | 回到 PR #4, 补修 compat 逻辑; 加 unit test 覆盖这个具体场景 |
| cases/default.json 比对失败, actual 输出 `unrated` 但 case 期望 `uncensor` | Tier 2c-bis 双读兼容漏实现 | 回到 PR #4, 在 `caseExpect` 结构上加旧字段兼容 |
| plugin 加载报 schema 错误 | 本不应发生 (插件 schema 未改) | 回归 PR #1 ~ #3 是否误删 / 误 rename 了 plugin 共享代码 |
| 插件搜索返回结果, 但 `Genres` 里出现了预期外的新字符串 | 有人不小心改了展示值 | 回退到 Tier 2a: 再次确认 `tag.Unrated = "未审查"` 这种 `<新标识符> = <旧展示值>` 的映射没写反 |

#### 外部仓库的后续迁移 (本次不做, 仅留备忘)

在 yamdc 主仓完成 Tier 1~5 之后, `yamdc-plugin` / `yamdc-script` 作为
独立仓库, 也可以逐步跟进以消除最后一层 JAV 信号。但这些**不属于
本 td 的范围**, 仅在此列出供后续决策:

- `yamdc-script/ruleset/006-matchers.yaml`: 约 30 条 `uncensor: true`
  可全量替换为 `unrated: true`, 规则名里的 `*_uncensor` 后缀可去掉。
- `yamdc-script/ruleset/004-suffix_rules.yaml`: `suffix_leak` / `suffix_hack`
  可改为 `suffix_special_edition` / `suffix_restored`。
- `yamdc-script/cases/default.json`: `"uncensor":` 键可改为 `"unrated":`。
- `yamdc-plugin/plugins/*.yaml` 里的 `name:` 字段 (`javdb`, `jav321`,
  `airav` 等): 是否 rename 是**运营决策**, 不是技术决策 — 这些是
  用户引用插件时写在配置里的唯一键, 改名会要求所有用户同步更新
  自己的 config。保守做法: 不动。

**节奏建议**: 主仓 merge 完 ~1 个月后再开 issue 讨论外部仓库迁移,
期间让兼容层帮忙兜底, 避免一次性爆出所有改动面。

---

## 5. 兼容性矩阵

(外部 bundle 场景单列在 PR #7 冒烟步骤里, 这里只列主仓用户面。)

| 用户面 | 是否受影响 | 说明 |
|---|---|---|
| 已有文件名 (`ABC-123-LEAK-C.mp4`) | ❌ 无影响 | 后缀字面量不变 |
| 已入库 `MovieMeta.Genres` = `["未审查", "特别版"]` | ❌ 无影响 | 展示值不变, watermark 规则继续匹配 |
| 用户自定义 `tag_mapper` 配置引用 "未审查" | ❌ 无影响 | 展示值不变 |
| 用户自定义 movieid-ruleset bundle 含 `uncensor: true` | ✅ 受影响但兼容 | 解析期自动迁移 + deprecation warn, 不报错 |
| 用户调用 `/api/debug/ruleset/*` 读取 `result.uncensor` 字段 | ⚠️ breaking | 字段改名为 `unrated`, CHANGELOG 公告 |
| 用户访问 HD 封面 (DMM CDN) | ❌ 无影响 | 运行时 URL 字节级一致 |
| 用户的 watermark PNG 资源 | ❌ 无影响 | go:embed 打包, 文件名 rename 用户不可见 |

---

## 6. 风险与回滚

### 主要风险

1. **PR #3 涉及 ~20 文件的 rename, 存在漏改漏测风险**。
   缓解: Go 编译器强类型兜底; CI 跑完整 race 测试集。
2. **PR #4 的 YAML 兼容层写错会导致老 bundle 静默失效**。
   缓解: 必须包含"同 bundle 混用新旧字段"的端到端 test。
3. **PR #2 的 base64 解码 panic 如果在生产发生, handler 包 init 期就
   挂掉**。
   缓解: panic 路径写单元测试确认非法 base64 字符串能被检测;
   有效 base64 字符串解码后 URL 合法性用额外 test 验证。
4. **API response 字段 rename (Tier 2c) 算 breaking**, 可能影响下游
   工具。
   缓解: 项目当前前端是唯一消费方, 同 PR 内同步改; CHANGELOG 明确
   声明; 版本号上 minor bump。

### 回滚策略

- 每个 PR 独立可 revert, 不会相互阻塞。
- PR #2 的 base64 如果需要紧急回退, 把 const 字符串改回明文、
  删除 init 函数即可, 2 行 diff。
- PR #3 / #4 如果漏改, 优先出 hotfix 补齐, 不 revert (revert 成本
  远大于补齐)。

---

## 7. 可量化的验收标准

改造完成后, 仓库里运行以下搜索应全部为 **0 命中** (测试 fixture +
明确标注为"历史格式"的保留项除外):

```bash
# 1. 外部站点域名
rg -i 'dmm\.co\.jp|awsimgsrc'

# 2. JAV 站点名 (作为 Go 标识符或显眼字符串)
rg -i '\b(javdb|jav321|javbus|javlib|airav|avmoo)\b' \
   --glob '!**/node_modules/**' \
   --glob '!**/package-lock.json' \
   --glob '!**/tsconfig.tsbuildinfo'

# 3. Go 源码里的 JAV 化标识符
rg 'Uncensored|WatermarkLeak|WatermarkHack|isUncensored|isLeak|isHack|GetIsLeak|GetIsHack' \
   --glob '*.go'

# 4. 默认规则 bundle 的 uncensor 字段
rg 'uncensor\s*:' internal/movieidcleaner/testdata/
```

第 1、3、4 条预期严格为 0。第 2 条允许命中 `docs/` 或 `web/package-lock.json`
这种明显不是信号的地方, 手动审核即可。

**外加一个"正派化"验收**: `README.md` 和 `docs/` 下的顶层 `design.md`
通读一遍, 不应出现"无码 / 流出 / 破解 / uncensored / leaked / hacked"
这六个中英文词。

---

## 8. 待决议点

**D1. `-C` 后缀的含义描述**. 保留"字幕"这个词还是改成"含字幕轨
版本"?
- 选项 A: 保留"字幕", 反正字幕本身是中性概念。
- 选项 B: 改成"含字幕轨版本", 更书面化, 更不像 JAV 圈黑话。
- **推荐**: B。

**D2. `movieidcleaner.Result` JSON API 字段**. `uncensor` → `unrated`
是否提供 JSON 双写兼容?
- 选项 A: 双写, 前端任意时间迁移。
- 选项 B: 单写新字段 + 同 PR 改前端。
- **推荐**: B (见 Tier 2c 讨论)。

**D3. 中文展示字符串 ("未审查" 等) 是否在后续独立 PR 里也换掉**?
- 本次方案**不动**。
- 独立议题: 如果后续想换成 "未分级"、"特别版本"、"修复版本",
  可以通过 tag_mapper 默认规则加 alias, 让新数据写新值、老数据由
  tag_mapper 迁移。**不在本次范围内**。

**D4. PR 数量**。当前拆成 6 个, 可以合并?
- PR #1 + PR #2 可以合并 (都是"低风险纯补丁")。
- PR #5 + PR #6 可以合并。
- **推荐**: 保持 6 个独立 PR, review 成本最低; 如果 team 偏好
  合并就合并。

---

## 9. 非改不可 vs 可以先放放

**必须改** (构成公开信号最强的):
- Tier 1b (test 名 / plugin 名)
- Tier 1c (HEYZO 品番)
- Tier 2a (tag 常量名)
- Tier 4 (DMM base64)

**强烈建议改** (二级信号):
- Tier 2b (Number 字段)
- Tier 2d (Watermark 枚举)
- Tier 3a (默认 bundle 规则名)

**可选改** (低优先级, 代码量大但 ROI 相对低):
- Tier 2c (movieidcleaner 字段 + YAML 兼容层) — 兼容层最复杂
- Tier 5 (文档同步) — 必然要做, 但可以合并到最后一次统一扫

如果时间紧张, 建议先做 **PR #1 + #2 + #3**, 这三个 PR 已经能把公开
门面和源码 tree 清理到 80% 效果。
