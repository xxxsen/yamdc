package config

var sysUncensorRule = []string{
	`^\d+[-|_]\d+$`,
	`^N\d+$`,
	`^K\d+$`,
	`^KB\d+$`,
	`^C\d+-KI\d+$`,
	`^1PON.*$`,
	`^CARIB.*$`,
	`^SM3D2DBD.*$`,
	`^SMDV.*$`,
	`^SKY.*$`,
	`^HEY.*$`,
	`^FC2.*$`,
	`^MKD.*$`,
	`^MKBD.*$`,
	`^H4610.*$`,
	`^H0930.*$`,
	`^MD[-|_].*$`,
	`^SMD[-|_].*$`,
	`^SSDV[-|_].*$`,
	`^CCDV[-|_].*$`,
	`^LLDV[-|_].*$`,
	`^DRC[-|_].*$`,
	`^MXX[-|_].*$`,
	`^DSAM[-|_].*$`,
}

var sysRewriteRule = []NumberRewriteRule{}

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
