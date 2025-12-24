package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TagNode 标签节点结构
type TagNode struct {
	Name     string     `json:"name"`
	Alias    []string   `json:"alias"`
	Children []*TagNode `json:"children"`
}

// TagMapper 标签映射器，用于处理标签的别名映射和父级补全
type TagMapper struct {
	aliasToStandard map[string]string   // 别名到标准标签的映射
	tagToParent     map[string]string   // 标签到父标签的映射
	tagToPath       map[string][]string // 标签到完整路径的缓存
}

// NewTagMapper 创建新的标签映射器
func NewTagMapper(filePath string) (*TagMapper, error) {
	mapper := &TagMapper{
		aliasToStandard: make(map[string]string),
		tagToParent:     make(map[string]string),
		tagToPath:       make(map[string][]string),
	}

	if filePath == "" {
		return nil, fmt.Errorf("tag mapping file path is empty")
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tag mapping file not found: %s", filePath)
	}

	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tag mapping file: %w", err)
	}

	// 尝试解析新格式（数组）
	var nodes []*TagNode
	err = json.Unmarshal(data, &nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tag mapping file as array: %w", err)
	}

	// 解析标签树
	if err = mapper.parseTagNodes(nodes, ""); err != nil {
		return nil, err
	}
	// 构建路径缓存
	mapper.buildPathCache()

	return mapper, nil
}

// parseTagNodes 解析新格式的标签树(数组形式)
func (tm *TagMapper) parseTagNodes(nodes []*TagNode, parent string) error {
	for _, node := range nodes {
		if node.Name == "" {
			continue
		}

		// 检查标签是否已存在(重复检测)
		if _, exists := tm.tagToParent[node.Name]; exists {
			return fmt.Errorf("duplicate tag: %s", node.Name)
		}

		// 检查标签名是否与已有别名冲突
		if _, exists := tm.aliasToStandard[node.Name]; exists {
			return fmt.Errorf("tag conflicts with existing alias: %s", node.Name)
		}

		// 记录父子关系
		tm.tagToParent[node.Name] = parent

		// 记录别名映射
		for _, alias := range node.Alias {
			if alias == "" {
				continue
			}
			// 检查别名是否已存在
			if _, exists := tm.aliasToStandard[alias]; exists {
				return fmt.Errorf("duplicate alias: %s", alias)
			}
			// 检查别名是否与已有标签名冲突
			if _, exists := tm.tagToParent[alias]; exists {
				return fmt.Errorf("alias conflicts with existing tag: %s", alias)
			}
			tm.aliasToStandard[alias] = node.Name
		}

		// 递归处理子节点
		if len(node.Children) > 0 {
			if err := tm.parseTagNodes(node.Children, node.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildPathCache 构建路径缓存,为每个标签计算完整路径
func (tm *TagMapper) buildPathCache() {
	// 为每个标签构建从根到该标签的完整路径
	for tag := range tm.tagToParent {
		tm.getOrBuildPath(tag)
	}
}

// getOrBuildPath 获取或构建标签的完整路径
func (tm *TagMapper) getOrBuildPath(tag string) []string {
	// 如果已经缓存，直接返回
	if path, exists := tm.tagToPath[tag]; exists {
		return path
	}

	// 构建路径
	path := make([]string, 0)

	// 向上追溯父标签
	current := tag
	for {
		path = append([]string{current}, path...) // 在前面插入

		parent, hasParent := tm.tagToParent[current]
		if !hasParent || parent == "" {
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
		if path, hasPath := tm.tagToPath[standardTag]; hasPath && len(path) > 0 {
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
