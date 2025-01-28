package config

var sysUncensorRule = []string{}

var sysRewriteRule = []NumberRewriteRule{}

var sysCategoryRule = []NumberCategoryRule{}

var sysNumberRule = NumberRule{
	NumberUncensorRules: sysUncensorRule,
	NumberRewriteRules:  sysRewriteRule,
	NumberCategoryRule:  sysCategoryRule,
}
