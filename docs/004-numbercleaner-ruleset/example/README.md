# 示例说明

本目录提供番号规则集系统的最小示例。

包含：

1. `manifest.yaml`
   - 演示规则包如何声明入口目录

2. `001-base.yaml`
   - 演示 `version` 与 `options`

3. `002-normalizers.yaml`
   - 演示基础 normalizer 规则

4. `006-matchers.yaml`
   - 演示 matcher 的 `normalize_template`、`category`、`uncensor`

5. `override.yaml`
   - 演示 override 层如何按规则名覆盖已有规则

6. `config.json`
   - 演示主程序如何声明规则包来源

这些示例只展示规则系统语义，不绑定具体站点或完整生产规则集。

