# 示例说明

本目录提供搜索插件 bundle 的最小示例。

包含：

1. `manifest.yaml`
   - 演示 bundle 元信息
   - 演示 `chains.<chain> + name` 组链配置
   - 演示默认链和分类链
   - 演示同一个逻辑插件名在默认链和分类链中分别配置

2. `config.json`
   - 演示主程序如何声明 plugin bundle source
   - 演示 GitHub remote source 与本地 source 的组合方式
   - 演示本地相对路径会基于 `data_dir` 解析

3. `plugin-test/`
   - 预留给 `plugin-test --plugin=... --output=json` 这类 CI 校验命令
   - 当前命令直接校验 bundle 目录，不再依赖单独的 case 文件

这些示例只展示 bundle 与配置层语义，不包含具体站点插件逻辑。
