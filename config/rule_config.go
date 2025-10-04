package config

var sysRuleConfig = RuleConfig{
	NumberRewriter: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_rewriter.toml",
	},
	NumberCategorier: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_categorier.toml",
	},
	NumberUncensorTester: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_uncensor_tester.toml",
	},
}
