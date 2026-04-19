// Package tag 存放跨 handler / number 包共享的标签常量。
//
// 这些字符串既是 MovieMeta.Genres 里写给用户看的最终展示值, 也是
// 下游 handler (目前是 watermark) 识别特定属性所依赖的契约, 所以
// 必须从任何单一产生者 (如 number) 里剥离出来独立维护, 避免
// magic string 在多个包里各自复制。
//
// 本包刻意只导出字符串常量, 不放任何函数 / 类型, 保持叶子包属性。
// 如果需要对 tag 做归一化 / 去重等工具函数, 放到调用方自己的包里去,
// 不要往这里塞。
package tag

// 常量名刻意使用电影发行行业通用术语 (Unrated / SpecialEdition /
// Restored), 与 MPAA / 院线发行常见分类对齐。展示值保持既有中文
// 字符串以保证向后兼容: 已入库的 MovieMeta.Genres、用户 tag_mapper
// 配置、watermark 规则都能继续命中。
const (
	Unrated         = "未审查"
	ChineseSubtitle = "字幕版"
	Res4K           = "4K"
	Res8K           = "8K"
	VR              = "VR"
	SpecialEdition  = "特别版"
	Restored        = "修复版"
)
