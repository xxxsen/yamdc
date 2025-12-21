package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建测试用的配置文件
func createTestConfigFile(t *testing.T, config map[string]interface{}) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tag-mappings.json")

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filePath, data, 0644)
	require.NoError(t, err)

	return filePath
}

// 测试配置文件示例
func getTestConfig() map[string]interface{} {
	return map[string]interface{}{
		"cosplay": map[string]interface{}{
			"_alias": []interface{}{"cos", "角色扮演"},
			"原神": map[string]interface{}{
				"_alias": []interface{}{"Genshin", "⚪神"},
				"芭芭拉·佩奇": []interface{}{"芭芭拉", "Barbara Pegg", "Barbara"},
				"莫娜":     []interface{}{"Mona"},
			},
		},
		"制服": map[string]interface{}{
			"_alias": []interface{}{"uniform", "유니폼"},
			"JK制服":   []interface{}{"jk", "水手服"},
			"护士服":    []interface{}{"nurse"},
		},
	}
}

func TestNewTagMapper_Disabled(t *testing.T) {
	mapper, err := NewTagMapper(false, "")
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
	assert.False(t, mapper.IsEnabled())
}

func TestNewTagMapper_FileNotFound(t *testing.T) {
	mapper, err := NewTagMapper(true, "/nonexistent/file.json")
	assert.Error(t, err)
	assert.NotNil(t, mapper)
}

func TestNewTagMapper_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(filePath, []byte("invalid json content"), 0644)
	require.NoError(t, err)

	mapper, err := NewTagMapper(true, filePath)
	assert.Error(t, err)
	assert.NotNil(t, mapper)
}

func TestNewTagMapper_Success(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)

	mapper, err := NewTagMapper(true, filePath)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
	assert.True(t, mapper.IsEnabled())

	// 验证别名映射是否正确构建
	assert.Equal(t, "cosplay", mapper.aliasToStandard["cos"])
	assert.Equal(t, "cosplay", mapper.aliasToStandard["角色扮演"])
	assert.Equal(t, "原神", mapper.aliasToStandard["Genshin"])
	assert.Equal(t, "芭芭拉·佩奇", mapper.aliasToStandard["Barbara"])
	assert.Equal(t, "莫娜", mapper.aliasToStandard["Mona"])

	// 验证父子关系是否正确构建
	assert.Equal(t, "cosplay", mapper.tagToParent["原神"])
	assert.Equal(t, "原神", mapper.tagToParent["芭芭拉·佩奇"])
	assert.Equal(t, "原神", mapper.tagToParent["莫娜"])
	assert.Equal(t, "制服", mapper.tagToParent["JK制服"])
}

func TestProcessTags_Disabled(t *testing.T) {
	mapper, err := NewTagMapper(false, "")
	require.NoError(t, err)

	input := []string{"cos", "test"}
	output := mapper.ProcessTags(input)
	assert.Equal(t, input, output)
}

func TestProcessTags_EmptyInput(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	output := mapper.ProcessTags([]string{})
	assert.Empty(t, output)
}

func TestProcessTags_AliasMapping(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "单个别名映射 - cos",
			input:    []string{"cos"},
			expected: []string{"cosplay"},
		},
		{
			name:     "单个别名映射 - 角色扮演",
			input:    []string{"角色扮演"},
			expected: []string{"cosplay"},
		},
		{
			name:     "子标签别名 - Genshin",
			input:    []string{"Genshin"},
			expected: []string{"cosplay", "原神"},
		},
		{
			name:     "孙标签别名 - Barbara",
			input:    []string{"Barbara"},
			expected: []string{"cosplay", "原神", "芭芭拉·佩奇"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := mapper.ProcessTags(tt.input)
			sort.Strings(output)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestProcessTags_ParentCompletion(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "标准标签 - cosplay",
			input:    []string{"cosplay"},
			expected: []string{"cosplay"},
		},
		{
			name:     "标准子标签 - 原神",
			input:    []string{"原神"},
			expected: []string{"cosplay", "原神"},
		},
		{
			name:     "标准孙标签 - 莫娜",
			input:    []string{"莫娜"},
			expected: []string{"cosplay", "原神", "莫娜"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := mapper.ProcessTags(tt.input)
			sort.Strings(output)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestProcessTags_MultipleTagsWithDedup(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "多标签去重 - 角色扮演 + Mona",
			input:    []string{"角色扮演", "Mona"},
			expected: []string{"cosplay", "原神", "莫娜"},
		},
		{
			name:     "重复标签 - cos + cosplay",
			input:    []string{"cos", "cosplay"},
			expected: []string{"cosplay"},
		},
		{
			name:     "父子标签去重 - Genshin + Barbara + 芭芭拉",
			input:    []string{"Genshin", "Barbara", "芭芭拉"},
			expected: []string{"cosplay", "原神", "芭芭拉·佩奇"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := mapper.ProcessTags(tt.input)
			sort.Strings(output)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestProcessTags_UnknownTag(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "未知标签保留",
			input:    []string{"unknown_tag"},
			expected: []string{"unknown_tag"},
		},
		{
			name:     "混合已知和未知标签",
			input:    []string{"cos", "unknown_tag"},
			expected: []string{"cosplay", "unknown_tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := mapper.ProcessTags(tt.input)
			sort.Strings(output)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestProcessTags_EmptyStringFiltering(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	input := []string{"cos", "", "  ", "Mona"}
	output := mapper.ProcessTags(input)

	// 空字符串应该被过滤
	for _, tag := range output {
		assert.NotEmpty(t, tag)
	}
}

func TestProcessTags_ComplexHierarchy(t *testing.T) {
	// 测试更深层次的层级结构
	config := map[string]interface{}{
		"Level1": map[string]interface{}{
			"_alias": []interface{}{"L1"},
			"Level2": map[string]interface{}{
				"_alias": []interface{}{"L2"},
				"Level3": map[string]interface{}{
					"_alias": []interface{}{"L3"},
					"Level4": map[string]interface{}{
						"_alias": []interface{}{"L4"},
						"Level5": []interface{}{"L5"},
					},
				},
			},
		},
	}

	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	// 测试最深层别名
	output := mapper.ProcessTags([]string{"L5"})
	expected := []string{"Level1", "Level2", "Level3", "Level4", "Level5"}
	sort.Strings(output)
	sort.Strings(expected)
	assert.Equal(t, expected, output)
}

func TestProcessTags_MultipleRootTags(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(true, filePath)
	require.NoError(t, err)

	// 测试多个根级标签
	input := []string{"uniform", "jk"}
	output := mapper.ProcessTags(input)

	// 应该包含 制服 和 JK制服
	assert.Contains(t, output, "制服")
	assert.Contains(t, output, "JK制服")
}
