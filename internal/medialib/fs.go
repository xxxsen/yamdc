package medialib

import "errors"

// 媒体库磁盘层常量与错误定义。
//
// 具体实现按职责分布在:
//   - fs_scan.go     目录扫描 / Detail 读取 / artwork 启发式
//   - fs_variant.go  variant 组装、主 variant 选择、文件附着
//   - fs_nfo.go      NFO ⇄ Meta 映射, plot / actor 规范化
//   - fs_artwork.go  封面 / fanart / poster 读写, poster crop
//   - fs_mutate.go   写操作 / 删除 / 路径解析 / 字符串工具

// 共享错误。
//
//nolint:gochecknoglobals // domain-level static errors
var (
	errLibraryItemNotDir            = errors.New("library item is not a directory")
	errLibraryDetailRequired        = errors.New("library detail is required")
	errLibraryVariantNotFound       = errors.New("library variant not found")
	errCoverNotFound                = errors.New("cover not found")
	errCropRectOutOfBounds          = errors.New("crop rectangle out of bounds")
	errOnlyExtrafanartDeletable     = errors.New("only extrafanart files can be deleted")
	errInvalidLibraryPath           = errors.New("invalid library path")
	errExtrafanartFilenameExhausted = errors.New("unable to allocate extrafanart filename")
)

// videoExts 是被认为是"视频"的扩展名集合, 影响 variant 判定与 file kind 展示。
//
//nolint:gochecknoglobals // domain-level static set
var videoExts = map[string]struct{}{
	".avi": {}, ".flv": {}, ".m2ts": {}, ".m4v": {}, ".mkv": {}, ".mov": {}, ".mp4": {}, ".mpe": {},
	".mpeg": {}, ".mpg": {}, ".mts": {}, ".rm": {}, ".rmvb": {}, ".strm": {}, ".ts": {}, ".wmv": {},
}

// imageExts 是被认为是"图片"的扩展名集合, 用于 artwork 检测与 variant 图片关联。
//
//nolint:gochecknoglobals // domain-level static set
var imageExts = map[string]struct{}{
	".avif": {}, ".bmp": {}, ".gif": {}, ".jpeg": {}, ".jpg": {}, ".png": {}, ".webp": {},
}
