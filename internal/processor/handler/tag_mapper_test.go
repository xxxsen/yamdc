package handler

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
func createTestConfigFile(t *testing.T, config []*TagNode) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tag-mappings.json")

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filePath, data, 0644)
	require.NoError(t, err)

	return filePath
}

// 测试配置文件示例
func getTestConfig() []*TagNode {
	return []*TagNode{
		{
			Name: "cosplay",
			Alias: []string{
				"cos", "角色扮演",
			},
			Children: []*TagNode{
				{
					Name: "原神",
					Alias: []string{
						"Genshin", "⚪神",
					},
					Children: []*TagNode{
						{
							Name: "芭芭拉·佩奇",
							Alias: []string{
								"芭芭拉", "Barbara Pegg", "Barbara",
							},
						},
						{
							Name: "莫娜",
							Alias: []string{
								"Mona",
							},
						},
					},
				},
			},
		},
		{
			Name: "制服",
			Alias: []string{
				"uniform", "유니폼",
			},
			Children: []*TagNode{
				{
					Name: "JK制服",
					Alias: []string{
						"jk", "水手服",
					},
				},
				{
					Name: "护士服",
					Alias: []string{
						"nurse",
					},
				},
			},
		},
	}
}

func TestNewTagMapper_EmptyPath(t *testing.T) {
	mapper, err := NewTagMapper("")
	assert.Error(t, err)
	assert.Nil(t, mapper)
	assert.Contains(t, err.Error(), "empty")
}

func TestNewTagMapper_FileNotFound(t *testing.T) {
	mapper, err := NewTagMapper("/nonexistent/file.json")
	assert.Error(t, err)
	assert.Nil(t, mapper)
}

func TestNewTagMapper_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(filePath, []byte("invalid json content"), 0644)
	require.NoError(t, err)

	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Nil(t, mapper)
}

func TestNewTagMapper_Success(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)

	mapper, err := NewTagMapper(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

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

func TestProcessTags_EmptyMapper(t *testing.T) {
	// 创建一个空的mapper（没有配置文件）
	// 模拟禁用状态
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.json")
	// 创建一个空配置
	err := os.WriteFile(filePath, []byte("[]"), 0644)
	require.NoError(t, err)

	mapper, err := NewTagMapper(filePath)
	require.NoError(t, err)

	input := []string{"cos", "test"}
	output := mapper.ProcessTags(input)
	// 空配置应该直接返回原标签
	assert.ElementsMatch(t, input, output)
}

func TestProcessTags_EmptyInput(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	require.NoError(t, err)

	output := mapper.ProcessTags([]string{})
	assert.Empty(t, output)
}

func TestProcessTags_AliasMapping(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
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
	mapper, err := NewTagMapper(filePath)
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
	mapper, err := NewTagMapper(filePath)
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
	mapper, err := NewTagMapper(filePath)
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
	mapper, err := NewTagMapper(filePath)
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
	config := []*TagNode{
		{
			Name: "Level1",
			Alias: []string{
				"L1",
			},
			Children: []*TagNode{
				{
					Name: "Level2",
					Alias: []string{
						"L2",
					},
					Children: []*TagNode{
						{
							Name: "Level3",
							Alias: []string{
								"L3",
							},
							Children: []*TagNode{
								{
									Name: "Level4",
									Alias: []string{
										"L4",
									},
									Children: []*TagNode{
										{
											Name: "Level5",
											Alias: []string{
												"L5",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
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
	mapper, err := NewTagMapper(filePath)
	require.NoError(t, err)

	// 测试多个根级标签
	input := []string{"uniform", "jk"}
	output := mapper.ProcessTags(input)

	// 应该包含 制服 和 JK制服
	assert.Contains(t, output, "制服")
	assert.Contains(t, output, "JK制服")
}

// TestValidateUniqueness_DuplicateTags 测试标签重复的情况
func TestValidateUniqueness_DuplicateTags(t *testing.T) {
	// 同级标签重复
	config := []*TagNode{
		{Name: "Tag1"},
		{Name: "Tag1"}, // 重复
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tag: Tag1")
	assert.Nil(t, mapper)
}

// TestValidateUniqueness_DuplicateTagsInDifferentLevels 测试不同层级标签重复
func TestValidateUniqueness_DuplicateTagsInDifferentLevels(t *testing.T) {
	config := []*TagNode{
		{
			Name: "Parent",
			Children: []*TagNode{
				{Name: "Child"},
			},
		},
		{Name: "Child"}, // 与子标签重复
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tag: Child")
	assert.Nil(t, mapper)
}

// TestValidateUniqueness_DuplicateAlias 测试别名重复
func TestValidateUniqueness_DuplicateAlias(t *testing.T) {
	config := []*TagNode{
		{
			Name:  "Tag1",
			Alias: []string{"alias1", "alias2"},
		},
		{
			Name:  "Tag2",
			Alias: []string{"alias1"}, // alias1重复
		},
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate alias: alias1")
	assert.Nil(t, mapper)
}

// TestValidateUniqueness_AliasConflictWithTagName 测试别名与标签名冲突
func TestValidateUniqueness_AliasConflictWithTagName(t *testing.T) {
	config := []*TagNode{
		{Name: "Tag1"},
		{
			Name:  "Tag2",
			Alias: []string{"Tag1"}, // 别名与Tag1标签名冲突
		},
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alias conflicts with existing tag: Tag1")
	assert.Nil(t, mapper)
}

// TestValidateUniqueness_AliasConflictWithChildTagName 测试别名与子标签名冲突
func TestValidateUniqueness_AliasConflictWithChildTagName(t *testing.T) {
	config := []*TagNode{
		{
			Name: "Parent",
			Children: []*TagNode{
				{Name: "Child"},
			},
		},
		{
			Name:  "Tag2",
			Alias: []string{"Child"}, // 别名与子标签名冲突
		},
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alias conflicts with existing tag: Child")
	assert.Nil(t, mapper)
}

// TestValidateUniqueness_MultipleErrors 测试多个校验错误
func TestValidateUniqueness_MultipleErrors(t *testing.T) {
	config := []*TagNode{
		{
			Name:  "Tag1",
			Alias: []string{"alias1"},
		},
		{
			Name:  "Tag1",             // 重复标签
			Alias: []string{"alias1"}, // 重复别名
		},
		{
			Name:  "Tag2",
			Alias: []string{"Tag1"}, // 别名与标签名冲突
		},
	}
	filePath := createTestConfigFile(t, config)
	_, err := NewTagMapper(filePath)
	assert.Error(t, err)
	// 应该包含一个错误信息, 因为检测到第一个错误就会return了,不会继续执行
	assert.Contains(t, err.Error(), "duplicate tag: Tag1")
}

// TestValidateUniqueness_ValidConfiguration 测试有效配置不报错
func TestValidateUniqueness_ValidConfiguration(t *testing.T) {
	config := getTestConfig()
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

// TestValidateUniqueness_ComplexValidConfiguration 测试复杂有效配置
func TestValidateUniqueness_ComplexValidConfiguration(t *testing.T) {
	config := []*TagNode{
		{
			Name:  "Level1A",
			Alias: []string{"L1A", "一级A"},
			Children: []*TagNode{
				{
					Name:  "Level2A",
					Alias: []string{"L2A"},
					Children: []*TagNode{
						{Name: "Level3A", Alias: []string{"L3A"}},
					},
				},
				{
					Name:  "Level2B",
					Alias: []string{"L2B"},
				},
			},
		},
		{
			Name:  "Level1B",
			Alias: []string{"L1B", "一级B"},
			Children: []*TagNode{
				{Name: "Level2C", Alias: []string{"L2C"}},
			},
		},
	}
	filePath := createTestConfigFile(t, config)
	mapper, err := NewTagMapper(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

// TestRealTagsJsonFile 测试实际的 tags.json 文件
func TestRealTagsJsonFile(t *testing.T) {
	// 尝试加载项目中的 tags.json 文件
	filePath := "../.vscode/tags.json"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Skip("tags.json file not found, skipping test")
		return
	}

	mapper, err := NewTagMapper(filePath)
	// 应该成功加载，没有重复或冲突错误
	assert.NoError(t, err, "tags.json should not have any duplicate tags or aliases")
	assert.NotNil(t, mapper)

	// 测试一些简单的标签处理
	if mapper != nil {
		// 测试 Cosplay 标签
		result := mapper.ProcessTags([]string{"cos"})
		assert.Contains(t, result, "Cosplay")

		// 测试多层级标签
		result = mapper.ProcessTags([]string{"Genshin"})
		assert.Contains(t, result, "Cosplay")
		assert.Contains(t, result, "原神")
	}
}
