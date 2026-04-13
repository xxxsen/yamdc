# 搜索插件 Bundle 系统设计

## 一、文档目的

本文档描述搜索插件 bundle 的正式设计与当前实现边界。

本设计用于将插件配置从主程序仓库中剥离出去，使主程序仅保留：

1. YAML 插件 runtime
2. bundle 加载与缓存能力
3. 搜索链运行时重建能力

## 二、设计目标

本方案的目标是：

1. 主程序本地不内置具体插件配置
2. 插件通过 bundle 独立分发和加载
3. 支持本地目录 bundle 与远程 zip bundle
4. 支持多个 bundle 同时加载
5. 支持按 category 组装搜索链
6. 支持同名插件按优先级决议
7. 支持远程缓存 zip，运行时直接读取 zip
8. 保持安全边界，只允许加载 YAML 插件，不执行远程代码

说明：

1. 当前主程序 runtime 已不再依赖 builtin YAML 插件注册
2. 插件链默认由外部 bundle 提供
3. 仓库内可能仍保留测试或开发辅助资源，但不作为运行时默认插件来源

## 三、非目标

本方案不做：

1. 不执行远程 Go/JS/Lua 等任意逻辑
2. 不支持远程自定义函数
3. 不在 bundle manifest 中承载插件逻辑
4. 不把 plugin bundle 作为主程序唯一的代码扩展机制

## 四、总体架构

系统分为两层。

### 4.1 通用数据加载层

位置：

```text
internal/bundle/manager.go
```

职责：

1. 从本地目录或远程 zip 读取 bundle 数据
2. 处理远程 zip 缓存与更新检测
3. 使用统一的 `fs.FS + base` 暴露 bundle 内容
4. 在数据可用或数据更新成功后调用外部 callback

这一层不关心业务对象是什么。

### 4.2 业务解析层

位置：

```text
internal/searcher/plugin/bundle/
internal/numbercleaner/bundle.go
```

职责：

1. 读取 manifest
2. 根据 `entry` 读取业务 YAML
3. 执行业务校验
4. 构建业务对象
5. 在 callback 中完成注册、替换或重建

## 五、Bundle 结构

本地目录与远程 zip 使用同一结构：

```text
<bundle-root>/
  manifest.yaml
  <entry>/
    *.yaml
```

不支持：

1. 本地目录无 manifest 的隐式加载
2. 把目录下全部 YAML 自动视为插件

补充：

1. 本地 source 在 `source_type` 为空或为 `local` 时，会先按 `data_dir` 解析相对路径
2. 远程 source 当前只支持 GitHub repository URL

## 六、Manifest 结构

```yaml
version: 1
name: official-plugins
desc: official searcher plugin bundle
bundle_version: 2026.04.06
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
  source_a:
    - name: alpha
      priority: 200
```

字段语义：

1. `version`: 当前固定为 `1`
2. `name`: bundle 名称，用于日志与冲突定位
3. `desc`: bundle 描述
4. `bundle_version`: bundle 自身版本
5. `entry`: 插件目录入口
6. `chains`: 搜索链配置，key 为链名，value 为插件列表

## 七、链路模型

系统有两类链：

1. `all`
2. 特定 `category`

规则：

1. `all` 是默认链名
2. 其他链名就是业务上的 category 名
3. 未命中特定 chain 时，只走 `all`
4. 命中特定 chain 时：
   1. 若该 chain 有配置，则只走该 chain
   2. 若该 chain 没有配置，则回退到 `all`

说明：

1. bundle 解析阶段不会把 `all` 预先拼入其他链
2. 真正的 fallback 在运行时搜索器中处理
3. 所有 bundle 合并后若没有 `all` 链，会打印 warning

## 八、唯一键与冲突规则

### 8.1 单个 manifest 内

唯一键为：

```text
(chain, name)
```

规则：

1. 同一个 manifest 内不允许重复 `(chain, name)`
2. `entry` 目录中插件 YAML 的 `name` 必须唯一
3. `configuration.name` 必须能映射到 bundle 内的插件 YAML

### 8.2 多 bundle 合并

对同一个 `(chain, name)`，按以下顺序决议：

1. `priority ASC`
2. `name ASC`
3. bundle 加载顺序

规则：

1. 排序后的第一个插件生效
2. 后续候选插件忽略
3. 若出现同名且同优先级冲突，保留第一个并记录 warning 日志

实现补充：

1. 运行时注册层会把 `(category, name)` 映射成内部唯一键
2. `all` 链仍直接使用原始插件名
3. 特定 category 链中的同名插件会使用 category-scoped 的内部运行时键
4. 这样可以保证同一个逻辑插件名在不同 category 下拥有不同实现时不会互相覆盖

## 九、远程缓存模型

远程 bundle 采用：

1. 下载 zip 到本地缓存
2. 不解压
3. 直接从 zip 中读取 `manifest + entry`
4. 校验成功后才替换 active zip

这与规则集使用的远程缓存模型保持一致。

当前实现约束：

1. 远程地址必须是 GitHub repository URL
2. 远程更新流程为：查询最新 tag -> 下载 codeload zip -> 校验 -> 激活
3. 后台同步失败时，会打印 warning 日志，并继续保留当前已激活版本

## 十、运行时生命周期

通用 manager 使用外部 callback 模型。

核心接口：

```go
type OnDataReadyFunc func(context.Context, *BundleData) error

type Manager struct { ... }

func NewManager(...) (*Manager, error)
func (m *Manager) Start(ctx context.Context) error
```

语义：

1. callback 必须在构造时传入
2. `Start(ctx)` 负责首次加载与后续自动更新
3. 首次加载失败时，`Start(ctx)` 返回错误
4. 若更新下载成功但 callback 执行失败，则不激活新数据
5. 后台更新失败不会中断已有运行时状态，只会记录日志

## 十一、当前实现结构

### 11.1 通用 bundle 层

```text
internal/bundle/manager.go
```

负责：

1. 本地目录读取
2. 远程 zip 缓存
3. GitHub 最新 tag 查询
4. zip 直接读取
5. callback 生命周期

### 11.2 搜索插件 bundle 层

```text
internal/searcher/plugin/bundle/
  manifest.go
  loader.go
  bundle_manager.go
```

负责：

1. manifest 结构定义与校验
2. 插件 YAML 加载
3. bundle 级校验
4. 多 bundle 合并与链路决议

### 11.3 Number Cleaner 层

```text
internal/numbercleaner/bundle.go
```

负责：

1. 将 bundle 数据解析为 `RuleSet`
2. 在 callback 中构建或重建 cleaner

## 十二、搜索插件运行时热更新

当前实现已经支持 server 路径上的插件 runtime 重建。

### 12.1 已实现

1. plugin bundle 更新后重新注册 YAML 插件
2. 热更新时旧 bundle 注册项会被移除，之后所有数据都以新 bundle 为准
3. 重新构建默认搜索链与分类搜索链
4. 通过运行时壳对象替换搜索链
5. debugger 的链路视图同步更新

相关实现：

```text
internal/searcher/category_searcher.go
internal/searcher/debugger.go
cmd/yamdc/bootstrap.go
```

### 12.2 当前边界

1. `server` 路径支持 runtime 重建
2. `run` 模式仍然是一轮静态执行，不需要热更新

## 十三、配置入口

主程序配置通过：

```json
{
  "searcher_plugin": {
    "sources": [
      {
        "source_type": "remote",
        "location": "https://github.com/yourname/your-yamdc-plugin-repo"
      }
    ]
  }
}
```

当前约束：

1. `config` 包只在 `cmd` 层使用
2. 业务包不直接依赖 `internal/config`
3. `cmd` 层负责将配置转换为各模块自己的内部结构
4. 本地 source 的相对路径基于 `data_dir` 解析

## 十四、强校验规则

bundle 加载时执行以下校验：

1. `manifest.version` 必填且兼容
2. `manifest.name` 必填
3. `manifest.entry` 必填
4. `priority` 必须在 `1..1000`
5. chain 名为空时自动归一成 `all`
6. 同一 manifest 内 `(chain, name)` 不允许重复
7. `entry` 目录内插件 `name` 必须唯一
8. `chains.*.name` 必须存在于插件集合中
9. 插件 YAML 自身必须通过 runtime schema 校验

## 十五、与配置层的边界

当前原则是：

1. `internal/config` 只在 `cmd` 层使用
2. 其他业务包使用各自内部定义的轻量结构

这条边界已应用于：

1. plugin bundle source 定义
2. handler debugger 配置
3. plugin runtime 重建路径

## 十六、结论

当前远程插件 bundle 系统已经具备可用实现，包含：

1. 本地目录 bundle
2. GitHub tag zip bundle
3. zip 缓存
4. 多 bundle 合并
5. category 链路决议
6. server 路径的插件 runtime 重建

因此该设计已可作为正式文档归档。
