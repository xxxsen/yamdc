package number

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultVariantDescriptors 锚定默认 variant 列表的关键不变量, 防止后续
// 改动意外删掉 / 重排 / 改动 ID 或 Suffix (这些字段会被前端持久化使用)。
func TestDefaultVariantDescriptors(t *testing.T) {
	descriptors := DefaultVariantDescriptors()
	require.NotEmpty(t, descriptors, "default descriptors should not be empty")

	// 每个 ID 全局唯一 (前端依赖此不变量做 selection 的 key)。
	idSet := make(map[string]struct{}, len(descriptors))
	for _, d := range descriptors {
		assert.NotEmpty(t, d.ID, "descriptor must have non-empty id: %+v", d)
		assert.NotEmpty(t, d.Suffix, "descriptor must have non-empty suffix: %+v", d)
		assert.NotEmpty(t, d.Label, "descriptor must have non-empty label: %+v", d)
		_, dup := idSet[d.ID]
		assert.False(t, dup, "duplicated descriptor id: %s", d.ID)
		idSet[d.ID] = struct{}{}
		assert.Contains(t, []VariantKind{VariantKindFlag, VariantKindIndexed}, d.Kind,
			"unknown kind %q for id %s", d.Kind, d.ID)
		if d.Kind == VariantKindIndexed {
			assert.GreaterOrEqual(t, d.Max, d.Min,
				"indexed descriptor %s: max (%d) < min (%d)", d.ID, d.Max, d.Min)
			assert.GreaterOrEqual(t, d.Min, 1,
				"indexed descriptor %s: min (%d) should be >= 1", d.ID, d.Min)
		}
	}

	// 每次调用必须返回独立切片, 调用方修改不能污染全局状态。
	first := DefaultVariantDescriptors()
	first[0].Label = "mutated"
	second := DefaultVariantDescriptors()
	assert.NotEqual(t, "mutated", second[0].Label,
		"DefaultVariantDescriptors must return a fresh slice each call")

	// 关键 ID / Suffix 对应关系断言, 一旦改动即意味着向老文件名的不兼容变更。
	type idSuffix struct {
		id     string
		suffix string
	}
	expected := []idSuffix{
		{VariantID4K, "4K"},
		{VariantID8K, "8K"},
		{VariantIDVR, "VR"},
		{VariantIDChineseSubtitle, "C"},
		{VariantIDSpecialEdition, "LEAK"},
		{VariantIDRestored, "UC"},
		{VariantIDMultiCD, "CD"},
	}
	got := make(map[string]string, len(descriptors))
	for _, d := range descriptors {
		got[d.ID] = d.Suffix
	}
	for _, want := range expected {
		assert.Equal(t, want.suffix, got[want.id], "id=%s suffix mismatch", want.id)
	}

	// Group 互斥关系锚定: 前端会按 group 字面量判相等, 所以这里把关键
	// 分组的成员关系写死, 防止后续改名导致前端 "4K / 8K 又能同时勾选" 这种
	// 静默回归。
	groupMembers := make(map[string][]string)
	for _, d := range descriptors {
		if d.Group == "" {
			continue
		}
		groupMembers[d.Group] = append(groupMembers[d.Group], d.ID)
	}
	assert.ElementsMatch(t,
		[]string{VariantID4K, VariantID8K},
		groupMembers[VariantGroupResolution],
		"resolution group should contain exactly 4K and 8K")
	assert.ElementsMatch(t,
		[]string{VariantIDSpecialEdition, VariantIDRestored},
		groupMembers[VariantGroupEdition],
		"edition group should contain exactly LEAK and UC")
	// VR / 中字 / 多盘 应当保持独立 (空 Group), 组内互斥不应波及它们。
	for _, d := range descriptors {
		switch d.ID {
		case VariantIDVR, VariantIDChineseSubtitle, VariantIDMultiCD:
			assert.Empty(t, d.Group,
				"variant %s should be independent (no group)", d.ID)
		}
	}
}

// TestApplyVariantSelections 覆盖 "apply" 的核心路径: 无 variant / flag / indexed /
// 多个组合 / 出错场景。尤其验证顺序: Generator 的顺序是固定的 (4K/8K/VR/C/LEAK/UC/CD),
// 即使用户随意传入 selection, 输出也必须按该顺序拼装, 和落盘文件名一致。
func TestApplyVariantSelections(t *testing.T) {
	t.Run("no selection normalizes base", func(t *testing.T) {
		out, err := ApplyVariantSelections("  pxvr-406  ", nil)
		require.NoError(t, err)
		assert.Equal(t, "PXVR-406", out)
	})

	t.Run("empty base fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("   ", nil)
		assert.ErrorIs(t, err, ErrVariantEmptyBase)
	})

	t.Run("flag variants combine in fixed order regardless of input order", func(t *testing.T) {
		out, err := ApplyVariantSelections("PXVR-406", []VariantSelection{
			{ID: VariantIDChineseSubtitle},
			{ID: VariantID4K},
		})
		require.NoError(t, err)
		// Expected order: base -4K -C (4K before C per GenerateSuffix).
		assert.Equal(t, "PXVR-406-4K-C", out)
	})

	t.Run("indexed CD variant", func(t *testing.T) {
		out, err := ApplyVariantSelections("PXVR-406", []VariantSelection{
			{ID: VariantIDMultiCD, Index: 2},
		})
		require.NoError(t, err)
		assert.Equal(t, "PXVR-406-CD2", out)
	})

	t.Run("full stack respects group mutex", func(t *testing.T) {
		// 4K / 8K 同属 resolution 组, LEAK / UC 同属 edition 组, 组内互斥,
		// 所以一次合法的 "full stack" 每个组最多取一个。
		out, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantIDVR},
			{ID: VariantID8K},
			{ID: VariantIDMultiCD, Index: 1},
			{ID: VariantIDChineseSubtitle},
			{ID: VariantIDSpecialEdition},
		})
		require.NoError(t, err)
		// Order per GenerateSuffix: 8K, VR, C, LEAK, CD1.
		assert.Equal(t, "ABC-001-8K-VR-C-LEAK-CD1", out)
	})

	t.Run("resolution group conflict (4K + 8K) fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantID4K},
			{ID: VariantID8K},
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrVariantGroupConflict)
		// 错误文案里带上 group 和冲突双方, 方便前端 / 排障直接定位。
		assert.Contains(t, err.Error(), "group=resolution")
		assert.Contains(t, err.Error(), "existing=resolution_4k")
		assert.Contains(t, err.Error(), "conflict=resolution_8k")
	})

	t.Run("edition group conflict (LEAK + UC) fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantIDSpecialEdition},
			{ID: VariantIDRestored},
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrVariantGroupConflict)
		assert.Contains(t, err.Error(), "group=edition")
	})

	t.Run("independent variants can coexist across groups", func(t *testing.T) {
		// VR / 中字 / CD 都是独立 variant (Group 为空), 4K / LEAK 各自占
		// 一个分组但不冲突 — 合法组合。
		out, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantID4K},
			{ID: VariantIDSpecialEdition},
			{ID: VariantIDVR},
			{ID: VariantIDChineseSubtitle},
			{ID: VariantIDMultiCD, Index: 2},
		})
		require.NoError(t, err)
		assert.Equal(t, "ABC-001-4K-VR-C-LEAK-CD2", out)
	})

	t.Run("duplicate id fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantID4K},
			{ID: VariantID4K},
		})
		assert.ErrorIs(t, err, ErrVariantDuplicate)
	})

	t.Run("unknown id fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: "definitely-not-a-variant"},
		})
		assert.ErrorIs(t, err, ErrVariantUnknownID)
	})

	t.Run("indexed out of range fails", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantIDMultiCD, Index: 0},
		})
		assert.ErrorIs(t, err, ErrVariantIndexOutOfRange)

		_, err = ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantIDMultiCD, Index: defaultMultiCDMax + 1},
		})
		assert.ErrorIs(t, err, ErrVariantIndexOutOfRange)
	})

	t.Run("output round-trips through Parse", func(t *testing.T) {
		// 输出能被自己的 Parse 再解析出等价的 variant, 保证我们和历史解析器
		// 关于 "variant 怎么长" 这件事没有漂移。
		out, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: VariantIDMultiCD, Index: 2},
			{ID: VariantIDChineseSubtitle},
			{ID: VariantID4K},
		})
		require.NoError(t, err)

		n, err := Parse(out)
		require.NoError(t, err)
		assert.Equal(t, "ABC-001", n.GetNumberID())
		assert.True(t, n.GetIs4K())
		assert.True(t, n.GetIsChineseSubtitle())
		assert.True(t, n.GetIsMultiCD())
		assert.Equal(t, 2, n.GetMultiCDIndex())
	})

	t.Run("wraps sentinel with id context", func(t *testing.T) {
		_, err := ApplyVariantSelections("ABC-001", []VariantSelection{
			{ID: "unknown"},
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVariantUnknownID))
		assert.Contains(t, err.Error(), "id=unknown")
	})
}
