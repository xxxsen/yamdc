package number

// 文件名后缀字面量。
//
// 这些 literal 字符串 (LEAK/U/UC 等) 是历史数据里的既成事实, 许多
// 用户的媒体库文件名都按这个形式落盘, 不能改; 只能把 Go 标识符改成
// 中性名 (SpecialEdition / Restored)。字面量与新语义的对应关系:
//
//	"LEAK"      <-> 特别版 / SpecialEdition  (非正式流通的发行版本)
//	"U" / "UC"  <-> 修复版 / Restored        (清晰度修复 / remaster)
const (
	defaultSuffixSpecialEdition  = "LEAK"
	defaultSuffixChineseSubtitle = "C"
	defaultSuffix4K              = "4K"
	defaultSuffix4KV2            = "2160P"
	defaultSuffix8K              = "8K"
	defaultSuffixVR              = "VR"
	defaultSuffixMultiCD         = "CD"
	defaultSuffixRestored1       = "U"
	defaultSuffixRestored2       = "UC"
)
