package config

var sysRuleConfig = RuleConfig{
	NumberRewriter: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/number_rewriter.yaml",
	},
	NumberCategorier: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/number_categorier.yaml",
	},
	NumberUncensorTester: LinkConfig{
		Type: "network",
		Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/number_uncensor_tester.yaml",
	},
}
