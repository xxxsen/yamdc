package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestTagMappingConfig 创建测试用的标签映射配置文件
func createTestTagMappingConfig(t *testing.T) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tag-mappings.json")

	// 使用新格式(数组)
	config := []*TagNode{
		{
			Name:  "cosplay",
			Alias: []string{"cos", "角色扮演"},
			Children: []*TagNode{
				{
					Name:  "原神",
					Alias: []string{"Genshin", "⚪神"},
					Children: []*TagNode{
						{
							Name:  "芭芭拉·佩奇",
							Alias: []string{"芭芭拉", "Barbara Pegg", "Barbara"},
						},
						{
							Name:  "莫娜",
							Alias: []string{"Mona"},
						},
					},
				},
			},
		},
		{
			Name:  "制服",
			Alias: []string{"uniform", "유니폼"},
			Children: []*TagNode{
				{
					Name:  "JK制服",
					Alias: []string{"jk", "水手服"},
				},
				{
					Name:  "护士服",
					Alias: []string{"nurse"},
				},
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filePath, data, 0644)
	require.NoError(t, err)

	return filePath
}

func TestTagMappingHandler_Disabled(t *testing.T) {
	// 创建禁用状态的处理器
	handler, err := createTagMappingHandler(map[string]interface{}{
		"enable": false,
	})
	require.NoError(t, err)

	num, err := number.Parse("test-001")
	require.NoError(t, err)

	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Genres: []string{"cos", "test"},
		},
	}

	err = handler.Handle(context.Background(), fc)
	assert.NoError(t, err)
	// 禁用状态下，标签不应该被修改
	assert.Equal(t, []string{"cos", "test"}, fc.Meta.Genres)
}

func TestTagMappingHandler_EmptyTags(t *testing.T) {
	filePath := createTestTagMappingConfig(t)
	handler, err := createTagMappingHandler(map[string]interface{}{
		"enable":    true,
		"file_path": filePath,
	})
	require.NoError(t, err)

	num, err := number.Parse("test-001")
	require.NoError(t, err)

	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Genres: []string{},
		},
	}

	err = handler.Handle(context.Background(), fc)
	assert.NoError(t, err)
	assert.Empty(t, fc.Meta.Genres)
}

func TestTagMappingHandler_AliasMapping(t *testing.T) {
	filePath := createTestTagMappingConfig(t)
	handler, err := createTagMappingHandler(map[string]interface{}{
		"file_path": filePath,
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "单个别名 - cos",
			input:    []string{"cos"},
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
		{
			name:     "多标签去重",
			input:    []string{"角色扮演", "Mona"},
			expected: []string{"cosplay", "原神", "莫娜"},
		},
		{
			name:     "未知标签保留",
			input:    []string{"unknown_tag"},
			expected: []string{"unknown_tag"},
		},
		{
			name:     "混合标签",
			input:    []string{"cos", "unknown_tag", "jk"},
			expected: []string{"cosplay", "unknown_tag", "制服", "JK制服"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, err := number.Parse("test-001")
			require.NoError(t, err)

			fc := &model.FileContext{
				Number: num,
				Meta: &model.MovieMeta{
					Genres: tt.input,
				},
			}

			err = handler.Handle(context.Background(), fc)
			assert.NoError(t, err)

			// 排序后比较
			sort.Strings(fc.Meta.Genres)
			sort.Strings(tt.expected)
			assert.Equal(t, tt.expected, fc.Meta.Genres)
		})
	}
}

func TestTagMappingHandler_FileNotFound(t *testing.T) {
	// 配置文件不存在时，应该创建禁用状态的处理器
	handler, err := createTagMappingHandler(map[string]interface{}{
		"enable":    true,
		"file_path": "/nonexistent/file.json",
	})
	require.NoError(t, err)

	num, err := number.Parse("test-001")
	require.NoError(t, err)

	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Genres: []string{"cos"},
		},
	}

	err = handler.Handle(context.Background(), fc)
	assert.NoError(t, err)
	// 文件不存在时降级为禁用状态，标签不变
	assert.Equal(t, []string{"cos"}, fc.Meta.Genres)
}

func TestTagMappingHandler_NoConfig(t *testing.T) {
	// 测试没有配置参数时的行为
	handler, err := createTagMappingHandler(map[string]interface{}{})
	require.NoError(t, err)

	num, err := number.Parse("test-001")
	require.NoError(t, err)

	fc := &model.FileContext{
		Number: num,
		Meta: &model.MovieMeta{
			Genres: []string{"test"},
		},
	}

	err = handler.Handle(context.Background(), fc)
	assert.NoError(t, err)
	// 无配置时应该禁用，标签不变
	assert.Equal(t, []string{"test"}, fc.Meta.Genres)
}
