# 封面水印 Handler 改造：从 Number 字段驱动到 Tag 驱动

## 0. 文档定位

- **对象**: `internal/processor/handler/watermark_handler.go` 及其在 pipeline 中的位置。
- **目的**: 把水印决策从"读 `Number` 内部字段"迁到"读 `MovieMeta.Genres` 标签集"，消除来源耦合与双真相，让新增水印类型的成本落到"改一张 table"。
- **性质**: **重构 + 轻量语义扩展**，非 breaking change。
- **规模预估**: 3 个 PR 左右，纯后端改动，不涉及前端 / DB / 接口。
- **前提决策**（已与 owner 确认）:
  - 引入一个新包 `internal/tag` 专放标签常量，`number` 和 `processor/handler` 都依赖它；**不**采用"直接从 `number` 导出大写常量"的方案。
  - 水印 → tag 映射表本次**不做成可配置**；写死在 handler 内，作为 priority table。可配置属于后续扩能，不在本次范围。
  - 水印匹配**忽略大小写**，兼容非 `tag_padder` 来源（YAML 插件 / 用户手填）可能出现 `"4k"` 等变体。
  - 水印匹配**按最终 watermark 去重**，允许多个 tag 映射到同一 watermark（如 `4K` / `8K` 都打"高清"水印），`Genres` 同时含多个映射 tag 时也只画一次。
  - 新增一个 `tag_dedup` handler，作为 **pipeline 末端的兜底清洁工**运行在 `tag_mapper` 之后：按 case-insensitive 去重，冲突时**优先保留大写形式**（`"4K"` 胜 `"4k"`，`"VR"` 胜 `"vr"`）。`tag_mapper` 是可选 handler（用户没配置 JSON 就是 no-op），所以必须有一个不依赖配置的 handler 来保证最终 `Genres` 数据质量。

---

## 1. 现状

### 1.1 Watermark handler 直接耦合 `Number` getter

```19:48:internal/processor/handler/watermark_handler.go
func (h *watermark) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta.Poster == nil || len(fc.Meta.Poster.Key) == 0 {
		return nil
	}
	tags := make([]image.Watermark, 0, 5)
	if fc.Number.GetIs4K() {
		tags = append(tags, image.WM4K)
	}
	if fc.Number.GetIs8K() {
		tags = append(tags, image.WM8K)
	}
	if fc.Number.GetIsVR() {
		tags = append(tags, image.WMVR)
	}
	if fc.Number.GetExternalFieldUnrated() {
		tags = append(tags, image.WMUnrated)
	}
	if fc.Number.GetIsChineseSubtitle() {
		tags = append(tags, image.WMChineseSubtitle)
	}
	if fc.Number.GetIsSpecialEdition() {
		tags = append(tags, image.WMSpecialEdition)
	}
	if fc.Number.GetIsRestored() {
		tags = append(tags, image.WMRestored)
	}
	...
}
```

7 个字段直接 hardcode；优先级靠 append 顺序隐式表达；新增一种水印要同时改 `Number` 结构、`GenerateTags()` 和这里的 if 链。

### 1.2 Tag 常量目前是 `number` 包私有

```15:23:internal/number/constant.go
const (
	defaultTagUnrated      = "未审查"
	defaultTagChineseSubtitle = "字幕版"
	defaultTag4K              = "4K"
	defaultTagSpecialEdition            = "特别版"
	defaultTagRestored            = "修复版"
	defaultTag8K              = "8K"
	defaultTagVR              = "VR"
)
```

只有 `number.GenerateTags()` 和 `number` 包内测试能看到。如果 watermark handler 也想匹配这些字符串，要么包内下沉到共享包，要么裸字符串散布，后者坚决不做。

### 1.3 Pipeline 默认顺序

```3:16:internal/config/handler_config.go
var sysHandler = []string{
	"hd_cover",
	"image_transcoder",
	"poster_cropper",
	"watermark_maker",   // 位置 4
	"actor_spliter",
	"duration_fixer",
	"translator",
	"chinese_title_translate_optimizer",
	"number_title",
	"ai_tagger",
	"tag_padder",        // 位置 11，tag 在这里才从 Number 派生出来
	"tag_mapper",        // 位置 12，用户可配的 tag 别名 / 父级补全
}
```

关键事实：**现在的 `watermark_maker` 跑在 `tag_padder` 之前**，所以它不可能读 `Genres`，只能读 `Number`。改造必须同步把它挪到 `tag_padder` 之后。

### 1.4 Tag 来源目前有 3 处

| 来源 | 产出 tag | 是否影响水印 (改造后) |
|---|---|---|
| `tag_padder` from `Number.GenerateTags()` | `4K` / `8K` / `VR` / `字幕版` / `特别版` / `修复版` / `未审查` | ✅ 会命中 |
| `tag_padder` from numberID 前缀 | 例如 `ABC` | ❌ 不在 watermark table 里 |
| `ai_tagger` | `AI-xxx`（带前缀） | ❌ 不会撞到 watermark 关键字 |

所以改造后水印关键字不会被 AI tagger 误触发。**唯一风险源是 `tag_mapper`** —— 见 §4。

---

## 2. 改造目标

1. `watermark_handler` 只依赖 `fc.Meta.Genres`，不再读 `fc.Number.*` 的任何 getter。
2. 水印的优先级 / 顺序用一张 priority table 显式表达；新增一种水印 = 常量 + table 加一行；支持多 tag 共享同一水印（table 里写多行、相同 `wm` 值），匹配时按 watermark 去重。
3. Tag 常量下沉到 `internal/tag` 新包；`number` 与 `processor/handler` 都依赖它，互不再互相偷看私有常量。
4. 新增 `tag_dedup` handler：case-insensitive 去重 + 大写优先，作为**不依赖用户配置**的兜底清洁工，放在 pipeline 末端。
5. 水印匹配 case-insensitive，内部自带 set 去重，不依赖外部 handler。
6. 默认 pipeline 调序，保证 `tag_padder` → `watermark_maker` → `tag_mapper` → `tag_dedup` 的顺序。
7. 语义变化（见 §4）在 commit message / 本文档里显式记录。

---

## 3. 设计

### 3.1 新包 `internal/tag`

**位置**: `internal/tag/constants.go`（新建）。

**内容**（仅放共享字符串常量，不做任何解析 / 业务）:

```go
// Package tag 存放跨 handler / number 包共享的标签常量。
//
// 这些字符串既是 Genres 里的最终展示值, 也是下游 handler
// (当前是 watermark) 识别特定属性所依赖的契约, 所以必须
// 从任何单一产生者 (如 number) 里剥离出来独立维护。
package tag

const (
	Unrated         = "未审查"
	ChineseSubtitle = "字幕版"
	K4              = "4K"
	K8              = "8K"
	VR              = "VR"
	SpecialEdition  = "特别版"
	Restored        = "修复版"
)
```

命名纠结点：Go identifier 不能以数字开头，`4K` → `K4` 可以接受；也可以用 `Res4K` / `Tag4K` 风格。**倾向 `Res4K` / `Res8K` + `VR` / `ChineseSubtitle` / `Unrated` / `SpecialEdition` / `Restored`**，语义更自然：

```go
const (
	Unrated         = "未审查"
	ChineseSubtitle = "字幕版"
	Res4K           = "4K"
	Res8K           = "8K"
	VR              = "VR"
	SpecialEdition  = "特别版"
	Restored        = "修复版"
)
```

> ⚠️ 若 owner review 时更喜欢扁平命名（`Tag4K` 等），按 review 意见调整，后续代码里整体替换成本低。

**注意**：
- 这个包**只导出常量**，不导出 struct / function。任何想放解析逻辑的冲动都应拒绝，保持它永远是叶子包。
- 未来如果新增"高清修复" / "HDR" 等标签，也加到这里，而不是各 handler 私有常量。

### 3.2 `number` 包重构

删除 `internal/number/constant.go` 里的 `defaultTagXxx` 私有常量；`GenerateTags()` 改成引用 `tag.Xxx`：

```go
import "github.com/xxxsen/yamdc/internal/tag"

func (n *Number) GenerateTags() []string {
	rs := make([]string, 0, 5)
	if n.GetExternalFieldUnrated() {
		rs = append(rs, tag.Unrated)
	}
	if n.GetIsChineseSubtitle() {
		rs = append(rs, tag.ChineseSubtitle)
	}
	if n.GetIs4K() {
		rs = append(rs, tag.Res4K)
	}
	// ...
}
```

`number_test.go` 里 `TestGenerateSuffixTagsFileName` 使用了 `defaultTagXxx`，跟着改成 `tag.Xxx`。这一步不改行为，纯搬家。

`defaultSuffixXxx`（文件名后缀用）留在 `number` 包，**不**要跟 tag 常量混到 `internal/tag`——那是文件名解析细节，不是跨包契约。

### 3.3 `watermark_handler` 重写

核心结构：一张 priority table + 基于 tag set 的匹配。

```go
package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/image"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/tag"

	"github.com/xxxsen/common/logutil"
)

// watermarkRule 定义"命中某 tag 就打某水印"的映射,
// 切片的顺序就是水印的优先级 (靠前 = 优先画).
type watermarkRule struct {
	tag string
	wm  image.Watermark
}

// defaultWatermarkRules 是默认的 tag → watermark 优先级表.
//
// 顺序语义:
//   - 同时命中多种标签时, 按此切片顺序依次追加水印.
//   - 允许多条 rule 指向同一个 image.Watermark (N-to-1), 例如 4K / 8K
//     都可以指向假想的 image.WMHD; 匹配时按最终 watermark 去重, 同一
//     watermark 只会画一次, 位置由"第一条命中的 rule"决定.
//   - image.addWatermarkToImage 有 defaultMaxWaterMarkCount=6 的上限,
//     超出的部分会被截断, 所以把"最想突显"的放前面.
var defaultWatermarkRules = []watermarkRule{
	{tag.Res4K, image.WM4K},
	{tag.Res8K, image.WM8K},
	{tag.VR, image.WMVR},
	{tag.Unrated, image.WMUnrated},
	{tag.ChineseSubtitle, image.WMChineseSubtitle},
	{tag.SpecialEdition, image.WMSpecialEdition},
	{tag.Restored, image.WMRestored},
}

type watermark struct {
	storage store.IStorage
	rules   []watermarkRule
}

func (h *watermark) Handle(ctx context.Context, fc *model.FileContext) error {
	if fc.Meta == nil || fc.Meta.Poster == nil || len(fc.Meta.Poster.Key) == 0 {
		return nil
	}
	wms := h.matchWatermarks(fc.Meta.Genres)
	if len(wms) == 0 {
		logutil.GetLogger(ctx).Debug("no watermark tag found, skip watermark proc")
		return nil
	}
	key, err := store.AnonymousDataRewriteWithStorage(ctx, h.storage, fc.Meta.Poster.Key,
		func(_ context.Context, data []byte) ([]byte, error) {
			return image.AddWatermarkFromBytes(data, wms)
		})
	if err != nil {
		return fmt.Errorf("save watermarked image failed, err:%w", err)
	}
	fc.Meta.Poster.Key = key
	return nil
}

// matchWatermarks 按 priority table 顺序扫描 genres, 命中即取,
// 但**最终水印按 image.Watermark 值去重**, 保证任何一种水印最多
// 画一次. 一个 watermark 被多条 rule 同时命中时, 位置取决于第一
// 条命中的 rule 在 h.rules 中的顺序.
//
// 匹配 case-insensitive: "4K" / "4k" 都能命中 tag.Res4K.
// 内部直接用 strings.ToLower 组 set, 不依赖任何外部去重 handler
// 先跑 — 这样 watermark 对 pipeline 配置零假设, 即使用户完全
// 不配 tag_dedup 也能正确工作.
func (h *watermark) matchWatermarks(genres []string) []image.Watermark {
	if len(genres) == 0 {
		return nil
	}
	genreSet := make(map[string]struct{}, len(genres))
	for _, g := range genres {
		genreSet[strings.ToLower(g)] = struct{}{}
	}
	seen := make(map[image.Watermark]struct{}, len(h.rules))
	rs := make([]image.Watermark, 0, len(h.rules))
	for _, rule := range h.rules {
		if _, ok := genreSet[strings.ToLower(rule.tag)]; !ok {
			continue
		}
		if _, dup := seen[rule.wm]; dup {
			continue
		}
		seen[rule.wm] = struct{}{}
		rs = append(rs, rule.wm)
	}
	return rs
}

func init() {
	Register(HWatermakrMaker, func(_ any, deps appdeps.Runtime) (IHandler, error) {
		return &watermark{
			storage: deps.Storage,
			rules:   defaultWatermarkRules,
		}, nil
	})
}
```

**要点**:

- `rules` 存在 struct 上而不是直接用全局变量，是为了单测可注入自定义 table（见 §5.2）。
- **两级去重**：
  - 输入 `genreSet` 对 `Genres` 做 case-insensitive 去重，`["4K", "4k", "4K"]` 看起来只是一个 tag。
  - 输出 `seen` 对 `image.Watermark` 做去重，任何一种水印最多画一次，支持"多 tag 映射同一水印"（如将来加 `{tag.Res4K, image.WMHD}` + `{tag.Res8K, image.WMHD}`）。
- **顺序语义**：watermark 的画图顺序由 rule table 决定；多条 rule 指向同一 watermark 时，取第一条命中 rule 的位置。换句话说，"把想优先画的 watermark 对应的 rule 放在前面"是唯一需要记的规则，不需要理解去重细节。
- **大小写**：统一走 `strings.ToLower` 作 key。**watermark 对 pipeline 配置零假设**，不依赖任何外部 handler 先跑（`tag_padder` / `tag_dedup` / `tag_mapper` 都可以被用户删掉）。
- `fc.Meta == nil` 的 nil check 是顺手补的——现有代码没查但实际上 `fc.Meta` 理论上不会为 nil；保留与否 review 定。

### 3.4 新增 `tag_dedup` handler

**位置**: `internal/processor/handler/tag_dedup_handler.go`（新建）。
**常量**: `internal/processor/handler/constant.go` 加 `HTagDedup = "tag_dedup"`。

**行为**:
- 按 `strings.ToLower` 作为规范化 key 分组。
- 每组内选出"最大写"的代表：ASCII 大写字母计数最多的那个；**平票时优先保留首次出现的**（稳定语义，便于 debug）。
- 保留首次出现的顺序（按 lowercase key 第一次出现的下标排序），`Genres` 重排后结果确定且可重放。

**判定函数示意**:

```go
package handler

import (
	"context"
	"strings"

	"github.com/xxxsen/yamdc/internal/appdeps"
	"github.com/xxxsen/yamdc/internal/model"
)

type tagDedupHandler struct{}

// dedupPreferUpper 按 case-insensitive 去重, 冲突时保留大写字母数最多的变体.
//
// 规则:
//   1. key = strings.ToLower(tag), 同 key 视为"同一个 tag".
//   2. 同 key 冲突时, 取 ASCII 大写字母计数较多者 ("4K" score=1 胜 "4k" score=0,
//      "VR" score=2 胜 "Vr" score=1 胜 "vr" score=0).
//   3. 平票时, 保留首次出现的变体 (稳定, 不依赖 map 迭代顺序).
//   4. 输出顺序 = 各 lowercase key 首次出现的下标顺序, 保留原始语义.
//
// 注意: 对纯 CJK tag ("字幕版"), score 恒为 0, 不受影响; 纯重复也会被折成一条.
func dedupPreferUpper(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	type pick struct {
		value string
		score int
		order int
	}
	best := make(map[string]*pick, len(tags))
	order := make([]string, 0, len(tags))
	for i, t := range tags {
		key := strings.ToLower(t)
		score := countUpperASCII(t)
		if p, ok := best[key]; ok {
			if score > p.score {
				p.value = t
				p.score = score
			}
			continue
		}
		best[key] = &pick{value: t, score: score, order: i}
		order = append(order, key)
	}
	rs := make([]string, 0, len(order))
	for _, k := range order {
		rs = append(rs, best[k].value)
	}
	return rs
}

func countUpperASCII(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			n++
		}
	}
	return n
}

func (h *tagDedupHandler) Handle(_ context.Context, fc *model.FileContext) error {
	if fc.Meta == nil || len(fc.Meta.Genres) == 0 {
		return nil
	}
	fc.Meta.Genres = dedupPreferUpper(fc.Meta.Genres)
	return nil
}

func init() {
	Register(HTagDedup, ToCreator(&tagDedupHandler{}))
}
```

**要点 / 不变量**:

- **幂等**: `dedupPreferUpper(dedupPreferUpper(x)) == dedupPreferUpper(x)`。即使用户在 pipeline 里插两次也无副作用。
- **顺序稳定**: 不依赖 map iteration order，靠 `order` 切片显式维护。
- **纯函数**: `dedupPreferUpper` 是包级函数而非 struct 方法，便于单测；handler 只是薄封装。
- **边界**:
  - `countUpperASCII` 只数 ASCII 字母。Unicode 大小写（如 `Ä`/`ä`）本项目里 tag 不会出现，不浪费 rune scan 成本。
  - 纯 CJK tag 永远 score=0，行为退化成"先到先得"的普通去重，符合直觉。
  - 不对 `Genres` 做额外 trim / normalize（不去空格、不剥标点）——这是独立的策略，超出"去重"职责。

**为什么不直接放进 `tag_padder` 或 `tag_mapper`**:

- `tag_padder` 负责"生产" tag，`tag_mapper` 负责"映射改写" tag；去重是**独立的职责**（clean-up pass），分开后：
  - 用户可以在 YAML 里灵活调位，比如只保留 dedup 不要 mapper。
  - 单测 / 故障排查时更容易定位问题。
  - 符合项目里其他 handler 的"单一职责"风格。

### 3.5 Pipeline 顺序调整

`internal/config/handler_config.go`:

```go
var sysHandler = []string{
	"hd_cover",
	"image_transcoder",
	"poster_cropper",
	// watermark_maker 从这里 (位置 4) 挪走
	"actor_spliter",
	"duration_fixer",
	"translator",
	"chinese_title_translate_optimizer",
	"number_title",
	"ai_tagger",
	"tag_padder",
	"watermark_maker", // 从位置 4 挪到这里, 紧贴 tag_padder 之后
	"tag_mapper",
	"tag_dedup",       // 新增: 末端兜底清洁工, 不依赖 tag_mapper 是否被配置
}
```

**为什么是这个位置**:

- `watermark_maker` **必须在 `tag_padder` 之后**：否则 `Genres` 还没从 `Number` 派生出来。
- `watermark_maker` **必须在 `tag_mapper` 之前**：`tag_mapper` 支持用户配别名和父级补全（例如把 `"字幕版"` 改写成 `"中文字幕"`）。如果水印跑在 mapper 之后，一旦用户改写就再也匹配不上。
- `tag_dedup` 放在**最末端**：它的职责是保证"写入 DB / 返回给前端的 `Genres`"这个最终态干净。`tag_mapper` 是可选 handler（用户没配 JSON 就是 no-op），不能依赖它兜底；把 `tag_dedup` 放在所有会动 `Genres` 的 handler 之后，一次性扫尾。
- `ai_tagger` 跑在 `tag_padder` 前或后都 OK——它产出的 tag 都带 `AI-` 前缀，不会撞上水印关键字。保留现有顺序即可。
- **watermark_maker 不依赖 tag_dedup 先跑**：`matchWatermarks` 内部自带 `strings.ToLower` + set 去重，哪怕上游完全不做 dedup 也能正确匹配。`tag_dedup` 纯粹是"用户可见数据的质量保障"，不是水印正确性的前置条件。

**延伸：用户如果删掉默认 handler 会怎样**:

| 删掉 | 后果 |
|---|---|
| `tag_padder` | Number 字段不再派生 tag，水印打不上；用户自担责任（合理） |
| `tag_mapper` | 用户不走别名映射，正常 |
| `tag_dedup` | 最终 `Genres` 可能含大小写重复，水印仍正确（因为 watermark 自带去重）；用户自担数据质量 |
| `watermark_maker` | 不打水印，正常 |

设计上保证"删任何一个都不会让别的 handler 炸掉"，各自职责独立。

---

## 4. 语义变化

旧实现："有 Number 标志" → 打水印。
新实现："有对应 tag" → 打水印。

两者**在绝大多数情况下等价**（因为 tag 由 `tag_padder` 从 `Number` 派生），但存在**故意扩大**的边界：

1. **站点返回的 tag 也会生效**：如果某个 YAML 插件 / ai 引擎 / 未来的 handler 往 `Genres` 里塞了 `"4K"`，即使番号没带 `-4K` 后缀，封面也会被打 4K 水印。
2. **UI 手动加 tag 生效**：如果前端允许用户编辑 tag，加一个 `"VR"` 会触发 VR 水印。

**我的判断**：这是期望行为，"tag 是真相"是这次重构要明确表达的立场。但必须:

- 在 commit message 里写明"behavior change: watermark now triggered by Genres, not Number".
- 在 `watermark_handler.go` 的包注释里加一两行说明。
- 不需要 feature flag / 回滚开关——改动足够小，真回归也就是 revert PR。

**反方向边界**：如果某种情况下 `Number.GetIs4K() == true` 但 `tag_padder` 被禁用了（用户自定义 pipeline），改造后就不会打水印。这是**合理的**——用户既然禁了 tag_padder，就不应该期望依赖 tag 的下游 handler 还能工作。handler pipeline 本来就是"配置驱动"的，组合责任在用户。

---

## 5. 测试策略

### 5.1 删 / 改现有测试

`internal/processor/handler/watermark_handler_test.go` 里：

- `TestWatermarkHandlerNilPoster` / `TestWatermarkHandlerEmptyPosterKey`: 不依赖 Number 字段，保留。
- `TestWatermarkHandlerNoTags`: 逻辑不变，保留（只是"没 tag 就跳过"从 Number 空转成 Genres 空）。
- `TestWatermarkHandlerStorageError`: 改成设置 `fc.Meta.Genres = []string{tag.Unrated}`，不再 `SetExternalFieldUnrated`。
- `TestWatermarkHandlerWithValidImage`: 同上，用 `Genres` 替代 `number.Parse("ABC-123-C")`。
- `TestWatermarkHandlerAllTagTypes`: 改成 table-driven，每行直接给一个 tag 字符串。

改完后 `Number` 在 watermark 测试里**完全消失**，这本身就是"解耦成功"的证据。

### 5.2 `watermark_handler` 新增测试（至少覆盖正常 / 异常 / 边缘 3 类）

| 名称 | 目的 | 覆盖类型 |
|---|---|---|
| `TestMatchWatermarksOrder` | 所有 tag 命中时，返回顺序严格等于 `defaultWatermarkRules` | 正常 |
| `TestMatchWatermarksPartial` | 部分 tag 命中（如只有 `4K` + `字幕版`） | 正常 |
| `TestMatchWatermarksCaseInsensitive` | `"4k"` / `"vr"` 也能命中对应水印 | 正常 |
| `TestMatchWatermarksEmpty` | `Genres` 为 `nil` / 空切片 | 边缘 |
| `TestMatchWatermarksUnknownTag` | 只有非水印 tag（如 `"Cosplay"`），返回空 | 边缘 |
| `TestMatchWatermarksDedup` | 同一 tag 出现多次（如 `["4K", "4K"]`），结果只含一次对应 watermark | 边缘 |
| `TestMatchWatermarksMixedCaseDedup` | `["4K", "4k"]` 共同命中 `image.WM4K`，结果仍只出现一次 | 边缘 |
| `TestMatchWatermarksSharedWatermark` | 注入自定义 rules，两条 rule（不同 tag）指向**同一** `image.Watermark`；输入 `Genres` 同时含两个 tag，结果该水印只出现一次，且顺序 = 第一条命中 rule 的位置 | 正常 |
| `TestMatchWatermarksSharedWatermarkSingleHit` | 同上的 rules 下，只命中其中一个 tag，水印正常出现一次 | 正常 |
| `TestMatchWatermarksCustomRules` | 用 struct-level `rules` 注入只含 2 行的 table，验证可测性 | 正常 |
| `TestWatermarkHandlerNoMetaPoster` | `fc.Meta == nil` 或 `Poster == nil` 安全返回 | 异常 |

`matchWatermarks` 抽成方法就是为了单测能绕开 `storage` / `image` 依赖，纯跑字符串匹配逻辑。

### 5.3 `tag_dedup_handler` 新增测试

| 名称 | 目的 | 覆盖类型 |
|---|---|---|
| `TestDedupPreferUpperBasic` | `["4K", "4k"]` → `["4K"]`；`["vr", "Vr", "VR"]` → `["VR"]` | 正常 |
| `TestDedupPreferUpperMixedCJK` | `["字幕版", "字幕版"]` → `["字幕版"]`；`["4K", "字幕版", "4k"]` → `["4K", "字幕版"]`（保留首次出现顺序） | 正常 |
| `TestDedupPreferUpperStableTie` | `["Ab", "aB"]` 平票 → 保留首次出现的 `"Ab"` | 边缘 |
| `TestDedupPreferUpperIdempotent` | `dedupPreferUpper(dedupPreferUpper(x)) == dedupPreferUpper(x)` | 边缘 |
| `TestDedupPreferUpperEmpty` | `nil` / `[]string{}` 输入不 panic，返回空 | 边缘 |
| `TestDedupPreferUpperNoDuplicate` | 输入无重复时原样返回（顺序保留） | 正常 |
| `TestTagDedupHandlerNilMeta` | `fc.Meta == nil` 安全返回 | 异常 |
| `TestTagDedupHandlerEmptyGenres` | `Genres` 为 `nil` / `[]`，不做无谓工作 | 边缘 |
| `TestTagDedupHandlerViaFactory` | 通过 `CreateHandler(HTagDedup, ...)` 工厂创建，验证注册成功 | 正常 |

`dedupPreferUpper` 是包级纯函数，测试只跑字符串逻辑，速度快、易调试。

### 5.4 集成 / 回归

- `TestCreateAllRegisteredHandlers` (`handler_test.go` L55-79) 保留，**追加** `HTagDedup` 到 handler 名列表，确保新 handler 也能被工厂创建。
- `debugger_test.go` 里涉及的 handler 列表不包括 `HWatermakrMaker` / `HTagDedup`，无影响。
- `internal/number/number_test.go` 里用到了 `defaultTagXxx`（L156-162），跟着改成 `tag.Xxx`。
- `make test-coverage` 阈值 ≥ 95%，改动前后都应该能过；新增测试不应降低覆盖率。

---

## 6. 分步落地计划

建议 4 个 PR，每个都小、独立、可回滚。

### PR 1 — 引入 `internal/tag` 包（纯搬家，零行为变化）

- 新建 `internal/tag/constants.go`，7 个常量。
- `internal/number/constant.go` 删除 `defaultTagXxx`。
- `internal/number/model.go` `GenerateTags()` 引用 `tag.Xxx`。
- `internal/number/number_test.go` 更新引用。
- **验证**: `make backend-check` 通过，`go test ./internal/number/...` 所有 case 仍绿。
- Diff 行数预估: < 80 行。

### PR 2 — 新增 `tag_dedup` handler（纯新增，不改现有行为）

- 新建 `internal/processor/handler/tag_dedup_handler.go`（含 `dedupPreferUpper` 纯函数 + handler）。
- `internal/processor/handler/constant.go` 加 `HTagDedup = "tag_dedup"`。
- 新增 `tag_dedup_handler_test.go`（§5.3）。
- `handler_test.go` 的 `TestCreateAllRegisteredHandlers` 追加 `HTagDedup`。
- **这一步暂不改 `sysHandler`**，handler 注册但不进入默认 pipeline，零行为变化。
- **验证**: `make backend-check` 通过。
- Diff 行数预估: ~180 行（测试占大头）。

### PR 3 — 重写 `watermark_handler` + pipeline 调序（行为改变）

- `watermark_handler.go` 按 §3.3 重写，引入 priority table + case-insensitive 匹配。
- `config/handler_config.go` 按 §3.5 调整 `sysHandler` 顺序：
  - 把 `watermark_maker` 从位置 4 挪到 `tag_padder` 之后、`tag_mapper` 之前。
  - 把 `tag_dedup` 追加到 `sysHandler` 末尾，作为兜底清洁工。
- `watermark_handler_test.go` 按 §5.1 改；按 §5.2 新增。
- 在 commit message 里显式写明两处 behavior change（§4）:
  1. watermark 现在由 `Genres` 驱动，`Number` 字段不再参与。
  2. 默认 pipeline 末端新增 `tag_dedup` 节点，`Genres` 写入 DB 前会被 case-insensitive 去重（大写优先）。
- **验证**: `make ci-check` 全绿；手动跑一次 `yamdc run` 对包含 `-4K-C` 的样本片确认水印仍在；再构造一个 `Genres` 里含 `"4k"`（小写）的 case 确认水印也打得上，且最终入库的 tag 已归一化成 `"4K"`。
- Diff 行数预估: 250~300 行（新测试占大头）。

### PR 4 —（可选）清理

- `image.Watermark` 枚举 / `resMap` 注释里显式标注"由 watermark_handler 通过 tag 驱动消费"。
- `watermark_handler.go` 包注释里写明"语义：tag-driven, case-insensitive"。
- 更新 README / AGENTS.md 如果有提到"Number 字段驱动水印"的描述（需先 grep 确认）。
- Diff 行数预估: < 30 行。

**为什么 tag_dedup 要单独一个 PR（PR 2）而不跟 PR 3 合并**:
- PR 2 是纯新增、零行为变化（handler 注册但 `sysHandler` 没改），review 负担低，可以先合入稳一波。
- PR 3 合并时一旦回归，revert 只影响 watermark 语义和 `sysHandler` 末尾的 `tag_dedup` 挂载；`tag_dedup` 作为已注册 handler 留在代码里，任何用户都可以手动把它加进自己的 pipeline 而不依赖默认顺序。
- bisect 友好：如果后续某用户反馈"水印打不上"或"Genres 有重复"，能精确定位到 PR 3 还是 PR 2。

**不要把 4 个 PR 压成一个**——PR 1 / PR 2 都是零风险，PR 3 有行为变化需要谨慎 review，拆开后 bisect 友好。

---

## 7. 风险与回滚

| 风险 | 概率 | 处理 |
|---|---|---|
| `tag_mapper` 配置把水印 tag 改写掉（用户行为） | 低 | 文档明示水印必须跑在 mapper 之前；不做额外保护 |
| 用户自定义 pipeline 禁用了 `tag_padder`，水印失效 | 低 | 合理行为，不处理（见 §4 结尾） |
| 新 tag 拼写与水印常量不一致（如 `"4k"` vs `"4K"`） | 中 | 双保险: `tag_dedup` 归一化 + `watermark_handler` 忽略大小写匹配，任意一层生效即可兜住 |
| `tag_dedup` 把用户有意保留的小写别名吞掉（如用户故意想保留 `"4k"` 作展示） | 低 | `tag_dedup` 明确是"大写优先"的策略，作为默认行为；若用户要反悔，可从自己的 pipeline 配置里删掉这个 handler |
| 第三方 plugin 往 `Genres` 塞脏 tag 误触发水印 | 低 | 重构恰好把这种情况变成"按 tag 行事"，视为期望行为 |
| PR 2 合并后某用户反馈封面与旧行为不一致 | 低 | PR 2 可直接 revert；`internal/tag` 已独立成 PR 1，不受影响 |

---

## 8. 非目标（明确不做的事）

- **不**把 tag → watermark 映射做成 JSON 配置。本次只做 table 化，不做可配化。
- **不**引入自定义水印图片 upload / 注册机制（`image.Watermark` 枚举 + 内嵌资源不变）。
- **不**改 `image.addWatermarkToImage` 的 `defaultMaxWaterMarkCount = 6` / 布局算法。
- **不**动 `GenerateSuffix` / 文件名后缀派生（那条链路继续吃 `Number` 字段，与 tag 正交）。
- **不**把 `defaultSuffixXxx` 也挪到 `internal/tag`（那是解析细节，不是跨包契约）。

---

## 9. 验收清单

- [ ] `internal/tag/constants.go` 存在且仅含常量，没有函数 / 类型。
- [ ] `grep defaultTag internal/number` 无结果。
- [ ] `grep "fc.Number" internal/processor/handler/watermark_handler.go` 无结果。
- [ ] `internal/processor/handler/tag_dedup_handler.go` 存在，`dedupPreferUpper` 有包级测试。
- [ ] `HTagDedup` 已在 `handler_test.go` 的 `TestCreateAllRegisteredHandlers` 名单里。
- [ ] `sysHandler` 顺序: `tag_padder` → `watermark_maker` → `tag_mapper` → `tag_dedup`（末端）。
- [ ] `watermark_handler.matchWatermarks` 使用 `strings.ToLower` 做匹配。
- [ ] `make ci-check` 通过。
- [ ] `internal/` 覆盖率 ≥ 95%。
- [ ] Behavior change 在 PR 3 的 commit message / 本文档 §4 显式声明。
