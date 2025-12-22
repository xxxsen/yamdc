package config

var sysPlugins = []string{
	"javbus",
	"javhoo",
	"airav",
	"javdb",
	"jav321",
	"javlibrary",
	"caribpr",
	"18av",
	"njav",
	"missav",
	"freejavbt",
	"tktube",
	"avsox",
}

var sysCategoryPlugins = []CategoryPlugin{
	//如果存在分配配置, 那么当番号被识别为特定分类的场景下, 将会使用分类插件直接查询
	{Name: "FC2", Plugins: []string{"fc2", "18av", "njav", "freejavbt", "tktube", "avsox", "fc2ppvdb"}},
	{Name: "JVR", Plugins: []string{"jvrporn"}},
	{Name: "COSPURI", Plugins: []string{"cospuri"}},
	{Name: "MD", Plugins: []string{"madouqu"}},
	{Name: "MANYVIDS", Plugins: []string{"manyvids"}},
}
