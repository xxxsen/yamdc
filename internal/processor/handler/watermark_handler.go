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

// defaultWatermarkRules 是默认的 tag -> watermark 优先级表.
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
	{tag.Uncensored, image.WMUncensored},
	{tag.ChineseSubtitle, image.WMChineseSubtitle},
	{tag.Leak, image.WMLeak},
	{tag.Hack, image.WMHack},
}

// watermark handler 按 MovieMeta.Genres 驱动水印绘制, 不再读 Number
// 字段. 对 pipeline 上下游 handler 无硬依赖: 内部自带 case-insensitive
// 匹配 + 水印级去重, 即使用户删掉 tag_dedup / tag_padder 等 handler,
// 本 handler 仍能按当前 Genres 做出正确决策.
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
// 最终水印按 image.Watermark 值去重, 保证任何一种水印最多画一次.
// 一个 watermark 被多条 rule 同时命中时, 位置取决于第一条命中的
// rule 在 h.rules 中的顺序.
//
// 匹配 case-insensitive: "4K" / "4k" 都能命中 tag.Res4K. 内部直接
// 用 strings.ToLower 组 set, 不依赖任何外部去重 handler 先跑.
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
