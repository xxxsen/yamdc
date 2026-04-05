# 示例说明

本目录提供番号规则集系统的分场景示例。

包含：

1. `basic-ruleset/`
   - 最小规则集示例
   - 展示 `options`、`normalizers`、`matchers`

2. `advanced-ruleset/`
   - 较复杂规则集示例
   - 展示 `rewrite_rules`、`suffix_rules`、`noise_rules`、`matchers`、`post_processors`
   - 展示 `require_boundary`、`score`、`category`、`uncensor`

3. `override-bundle/`
   - 演示规则包结构
   - 演示 `manifest.yaml + entry`
   - 演示 override 层如何覆盖已有规则
   - 演示主程序如何声明 ruleset bundle source

这些示例只展示规则系统语义，不绑定完整生产规则集。
