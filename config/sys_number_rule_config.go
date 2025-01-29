package config

var sysUncensorRule = []string{
	`(?i)^\d+[-|_]\d+$`,
	`(?i)^N\d+$`,
	`(?i)^K\d+$`,
	`(?i)^KB\d+$`,
	`(?i)^C\d+-KI\d+$`,
	`(?i)^1PON.*$`,
	`(?i)^CARIB.*$`,
	`(?i)^SM3D2DBD.*$`,
	`(?i)^SMDV.*$`,
	`(?i)^SKY.*$`,
	`(?i)^HEY.*$`,
	`(?i)^FC2.*$`,
	`(?i)^MKD.*$`,
	`(?i)^MKBD.*$`,
	`(?i)^H4610.*$`,
	`(?i)^H0930.*$`,
	`(?i)^MD[-|_].*$`,
	`(?i)^SMD[-|_].*$`,
	`(?i)^SSDV[-|_].*$`,
	`(?i)^CCDV[-|_].*$`,
	`(?i)^LLDV[-|_].*$`,
	`(?i)^DRC[-|_].*$`,
	`(?i)^MXX[-|_].*$`,
	`(?i)^DSAM[-|_].*$`,
}

var sysRewriteRule = []NumberRewriteRule{ //rewrite 逻辑在number.Parse之前, 所以数据可能存在小写的情况, 需要特殊处理
	{
		Remark:  "format fc2",
		Rule:    `(?i)^fc2[-|_]?(ppv)?[-|_](\d+)([-|_].*)?$`, //需要处理后面的-C-CD1之类的字符串, 用正则出来起来真的麻烦...
		Rewrite: `FC2-PPV-$2$3`,
	},
	{
		Remark:  "format number like '234abc-123' to 'abc-123'",
		Rule:    `^\d+([a-zA-Z]+[-|_]\d+)([-|_].*)?$`,
		Rewrite: `$1$2`,
	},
}

var sysCategoryRule = []NumberCategoryRule{
	{
		Rules: []string{
			"^FC2.*$",
		},
		Category: "FC2",
	},
}

var sysNumberRule = NumberRule{
	NumberUncensorRules: sysUncensorRule,
	NumberRewriteRules:  sysRewriteRule,
	NumberCategoryRule:  sysCategoryRule,
}
