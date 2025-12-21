package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TagMapper 标签映射器，用于处理标签的别名映射和父级补全
type TagMapper struct {
	enabled         bool
	aliasToStandard map[string]string   // 别名到标准标签的映射
	tagToParent     map[string]string   // 标签到父标签的映射
	tagToPath       map[string][]string // 标签到完整路径的缓存
}

// NewTagMapper 创建新的标签映射器
func NewTagMapper(enable bool, filePath string) (*TagMapper, error) {
	mapper := &TagMapper{
		enabled:         enable,
		aliasToStandard: make(map[string]string),
		tagToParent:     make(map[string]string),
		tagToPath:       make(map[string][]string),
	}

	if !enable {
		return mapper, nil
	}

	if filePath == "" {
		return mapper, fmt.Errorf("tag mapping file path is empty")
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return mapper, fmt.Errorf("tag mapping file not found: %s", filePath)
	}

	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return mapper, fmt.Errorf("failed to read tag mapping file: %w", err)
	}

	// 解析 JSON
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return mapper, fmt.Errorf("failed to parse tag mapping file: %w", err)
	}

	// 递归解析标签树
	mapper.parseTagTree(config, "")

	// 构建路径缓存
	mapper.buildPathCache()

	return mapper, nil
}

// parseTagTree 递归解析标签树
func (tm *TagMapper) parseTagTree(node map[string]interface{}, parent string) {
	for key, value := range node {
		// 跳过 _alias 字段，它在处理标准标签时会被处理
		if key == "_alias" {
			continue
		}

		// 当前键是标准标签
		standardTag := key

		// 如果有父标签，记录父子关系
		if parent != "" {
			tm.tagToParent[standardTag] = parent
		}

		// 处理值的不同类型
		switch v := value.(type) {
		case map[string]interface{}:
			// 值是对象，可能包含 _alias 和子标签
			// 先处理当前标签的别名
			if aliases, ok := v["_alias"]; ok {
				if aliasArray, ok := aliases.([]interface{}); ok {
					for _, alias := range aliasArray {
						if aliasStr, ok := alias.(string); ok {
							tm.aliasToStandard[aliasStr] = standardTag
						}
					}
				}
			}

			// 递归处理子标签
			tm.parseTagTree(v, standardTag)

		case []interface{}:
			// 值是数组，表示这是简化别名配置
			for _, alias := range v {
				if aliasStr, ok := alias.(string); ok {
					tm.aliasToStandard[aliasStr] = standardTag
				}
			}
		}
	}
}

// buildPathCache 构建路径缓存，为每个标签计算完整路径
func (tm *TagMapper) buildPathCache() {
	// 为每个标签构建从根到该标签的完整路径
	for tag := range tm.tagToParent {
		tm.getOrBuildPath(tag)
	}

	// 为没有父标签的根标签也建立路径（只包含自己）
	for tag := range tm.aliasToStandard {
		standardTag := tm.aliasToStandard[tag]
		if _, exists := tm.tagToPath[standardTag]; !exists {
			tm.tagToPath[standardTag] = []string{standardTag}
		}
	}
}

// getOrBuildPath 获取或构建标签的完整路径
func (tm *TagMapper) getOrBuildPath(tag string) []string {
	// 如果已经缓存，直接返回
	if path, exists := tm.tagToPath[tag]; exists {
		return path
	}

	// 构建路径
	path := []string{}

	// 向上追溯父标签
	current := tag
	for {
		path = append([]string{current}, path...) // 在前面插入

		parent, hasParent := tm.tagToParent[current]
		if !hasParent {
			break
		}
		current = parent
	}

	// 缓存路径
	tm.tagToPath[tag] = path
	return path
}

// ProcessTags 处理标签列表，执行别名映射和父级补全
func (tm *TagMapper) ProcessTags(tags []string) []string {
	if !tm.enabled {
		return tags
	}

	if len(tags) == 0 {
		return tags
	}

	// 使用 map 作为集合来去重
	resultSet := make(map[string]bool)

	for _, tag := range tags {
		// 标签规范化：去除首尾空格
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		// 查找是否是别名
		standardTag := tag
		if mapped, isAlias := tm.aliasToStandard[tag]; isAlias {
			standardTag = mapped
		}

		// 查找是否有完整路径
		if path, hasPath := tm.tagToPath[standardTag]; hasPath {
			// 添加完整路径中的所有标签
			for _, t := range path {
				resultSet[t] = true
			}
		} else {
			// 没有路径信息，只添加标准标签本身
			resultSet[standardTag] = true
		}
	}

	// 转换为切片
	result := make([]string, 0, len(resultSet))
	for tag := range resultSet {
		result = append(result, tag)
	}

	return result
}

// IsEnabled 返回映射器是否启用
func (tm *TagMapper) IsEnabled() bool {
	return tm.enabled
}
