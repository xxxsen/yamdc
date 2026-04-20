package number

import (
	"errors"
	"fmt"
	"strings"
)

// VariantKind 描述一个 variant 在 UI 上的输入形式。
//
//   - VariantKindFlag    : 是/否的开关, 对应一个固定的后缀字面量 (例如 "-C"、"-4K")。
//   - VariantKindIndexed : 需要一个整数 index 的输入, 对应 "后缀 + 数字" 的字面量
//     (例如 "-CD1", "-CD2")。Index 取值必须落在 [Min, Max] 区间。
//
// 定义枚举是为了让前端能用不同的组件渲染 (checkbox vs. number input /
// selector), 后端也能对输入做类型校验。
type VariantKind string

const (
	VariantKindFlag    VariantKind = "flag"
	VariantKindIndexed VariantKind = "indexed"
)

// VariantDescriptor 描述一种可附加在 number 上的 variant 能力。它是
// `GET /api/number/variants` 的返回类型, 前端用这份 schema 去渲染
// "选择 variant" 的 UI, 并把用户的选择通过 `VariantSelection` 回写给
// 后端, 后端据此拼出 number+后缀。
//
// 字段约定:
//   - ID: 稳定的 string key, 前端/后端/测试之间的锚点, 永远不随 label 变更。
//   - Suffix: 落到文件名里的字面量 (不带前导 '-')。必须和历史命名一致。
//   - Label: UI 上的短名 (例如 "中字", "4K"), 用于按钮/chip 展示。
//   - Description: 长说明, 鼠标悬停或下拉展开时展示。
//   - Kind: 输入形式, 见 VariantKind。
//   - Min / Max: 仅当 Kind == VariantKindIndexed 时有意义, 表示 index 合法区间。
//
// 设计取舍:
//   - 故意不把 "4K" / "2160P" 两种写法都暴露出去。它们在 number.Parse
//     里都会被识别为 is4k, 但生成文件名只用 "4K"; 暴露两个变体只会让
//     用户选择时产生困惑。需要识别 "2160P" 历史文件名时仍由 Parse 负责。
type VariantDescriptor struct {
	ID          string      `json:"id"`
	Suffix      string      `json:"suffix"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Kind        VariantKind `json:"kind"`
	Min         int         `json:"min,omitempty"`
	Max         int         `json:"max,omitempty"`
}

// 稳定的 ID 常量。它们会被前端硬编码使用, 一旦定下不能改。
const (
	VariantIDChineseSubtitle = "chinese_subtitle"
	VariantID4K              = "resolution_4k"
	VariantID8K              = "resolution_8k"
	VariantIDVR              = "vr"
	VariantIDSpecialEdition  = "special_edition"
	VariantIDRestored        = "restored"
	VariantIDMultiCD         = "multi_cd"
)

// defaultMultiCDMax 是 UI 上 CD 下拉框提供的最大值。真实世界的多盘一般不会
// 超过 2-3 张, 给 10 已经远超需要, 防止用户遇到 "10 张 CD 的合集" 类边缘数据。
const defaultMultiCDMax = 10

// DefaultVariantDescriptors 返回前端应当展示的 variant 列表, 顺序也是建议的
// 渲染顺序 (和 Number.GenerateSuffix 里的拼装顺序保持一致, 这样 UI 上看到的
// 顺序和最终落到文件名里的顺序一致, 降低心智负担)。
//
// 返回值是每次重新分配的副本, 所以调用方修改切片内容不会污染全局状态。
func DefaultVariantDescriptors() []VariantDescriptor {
	return []VariantDescriptor{
		{
			ID:          VariantID4K,
			Suffix:      defaultSuffix4K,
			Label:       "4K",
			Description: "4K 分辨率",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantID8K,
			Suffix:      defaultSuffix8K,
			Label:       "8K",
			Description: "8K 分辨率",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantIDVR,
			Suffix:      defaultSuffixVR,
			Label:       "VR",
			Description: "VR 视角",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantIDChineseSubtitle,
			Suffix:      defaultSuffixChineseSubtitle,
			Label:       "中字",
			Description: "含中文字幕",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantIDSpecialEdition,
			Suffix:      defaultSuffixSpecialEdition,
			Label:       "特别版",
			Description: "特别版 / 流出版 (LEAK)",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantIDRestored,
			Suffix:      defaultSuffixRestored2,
			Label:       "修复版",
			Description: "修复 / 重制版 (UC)",
			Kind:        VariantKindFlag,
		},
		{
			ID:          VariantIDMultiCD,
			Suffix:      defaultSuffixMultiCD,
			Label:       "多盘",
			Description: "多张光盘 / 分段视频 (例如 CD1, CD2)",
			Kind:        VariantKindIndexed,
			Min:         1,
			Max:         defaultMultiCDMax,
		},
	}
}

// VariantSelection 是用户对某个 VariantDescriptor 的一次选择:
//   - 对 Kind == VariantKindFlag 的 descriptor, Index 被忽略, 出现即视为"勾选";
//   - 对 Kind == VariantKindIndexed 的 descriptor, Index 必须落在描述符的
//     [Min, Max] 区间内。
type VariantSelection struct {
	ID    string `json:"id"`
	Index int    `json:"index,omitempty"`
}

// Variant 相关的错误都以 sentinel 形式暴露, 调用方可以用 errors.Is 分流到
// 具体的错误码 / 错误提示; 同时错误信息里会带上具体 ID, 方便排障。
var (
	ErrVariantEmptyBase       = errors.New("variant: empty base number")
	ErrVariantUnknownID       = errors.New("variant: unknown id")
	ErrVariantDuplicate       = errors.New("variant: duplicate selection")
	ErrVariantIndexOutOfRange = errors.New("variant: index out of range")
)

// ApplyVariantSelections 把用户选中的 variant 合并到 base number 之上, 返回
// 最终的 "number + 后缀" 字符串。内部复用 Number.GenerateSuffix, 保证
// 顺序和历史落盘一致, 前端显示顺序和文件名顺序一致。
//
// 行为:
//   - base 会被 TrimSpace + ToUpper, 和 Parse 的规范化保持一致。
//   - 同一个 ID 多次出现会返回 ErrVariantDuplicate (出错优先于静默去重,
//     方便前端暴露 bug, 而不是把重复提交悄悄接住)。
//   - 未知 ID 返回 ErrVariantUnknownID。
//   - indexed kind 的 Index 超出 [Min, Max] 返回 ErrVariantIndexOutOfRange。
//
// 没有任何 selection 时, 函数仍会做 base 的规范化 (TrimSpace / ToUpper),
// 不会返回错误 — 这样调用方可以无脑走 ApplyVariantSelections, 用一条路径
// 覆盖 "无 variant" 和 "有 variant" 两种情况。
func ApplyVariantSelections(base string, selections []VariantSelection) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(base))
	if normalized == "" {
		return "", ErrVariantEmptyBase
	}

	descriptors := DefaultVariantDescriptors()
	byID := make(map[string]VariantDescriptor, len(descriptors))
	for _, d := range descriptors {
		byID[d.ID] = d
	}

	info := &Number{numberID: normalized}
	seen := make(map[string]struct{}, len(selections))
	for _, sel := range selections {
		if _, dup := seen[sel.ID]; dup {
			return "", fmt.Errorf("id=%s: %w", sel.ID, ErrVariantDuplicate)
		}
		seen[sel.ID] = struct{}{}

		desc, ok := byID[sel.ID]
		if !ok {
			return "", fmt.Errorf("id=%s: %w", sel.ID, ErrVariantUnknownID)
		}
		if err := applyVariantOnNumber(info, desc, sel); err != nil {
			return "", err
		}
	}

	return info.GenerateFileName(), nil
}

func applyVariantOnNumber(info *Number, desc VariantDescriptor, sel VariantSelection) error {
	switch desc.ID {
	case VariantIDChineseSubtitle:
		info.isChineseSubtitle = true
	case VariantID4K:
		info.is4k = true
	case VariantID8K:
		info.is8k = true
	case VariantIDVR:
		info.isVr = true
	case VariantIDSpecialEdition:
		info.isSpecialEdition = true
	case VariantIDRestored:
		info.isRestored = true
	case VariantIDMultiCD:
		if sel.Index < desc.Min || sel.Index > desc.Max {
			return fmt.Errorf("id=%s index=%d range=[%d,%d]: %w",
				desc.ID, sel.Index, desc.Min, desc.Max, ErrVariantIndexOutOfRange)
		}
		info.isMultiCD = true
		info.multiCDIndex = sel.Index
	default:
		// defensive: descriptor 表和 switch 不同步时直接报错。
		return fmt.Errorf("id=%s: %w", desc.ID, ErrVariantUnknownID)
	}
	return nil
}
