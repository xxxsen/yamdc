package config

// sysHandler 是默认的 handler pipeline 顺序.
//
// Tag 流水线的位置约束 (见 td/023):
//   - tag_padder 从 Number 字段派生出基础 tag.
//   - watermark_maker 消费 Genres 打水印; 必须在 tag_padder 之后
//     (否则 tag 还没有), 必须在 tag_mapper 之前 (否则用户配置的
//     别名映射会让水印 tag 失效).
//   - tag_mapper 按用户 JSON 配置做别名 / 父级补全 (可选 handler).
//   - tag_dedup 是末端兜底的清洁工, case-insensitive 去重 + 大写优先,
//     不依赖 tag_mapper 是否被配置, 保证写入 DB / 返回前端的 Genres
//     永远干净.
var sysHandler = []string{
	"hd_cover",
	"image_transcoder",
	"poster_cropper",
	"actor_spliter",
	"duration_fixer",
	"translator",
	"chinese_title_translate_optimizer",
	"number_title",
	"ai_tagger",
	"tag_padder",
	"tag_mapper",
	"watermark_maker",
	"tag_dedup",
}
