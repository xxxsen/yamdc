yamdc
===

一个面向影片资料整理、元数据补全与媒体库管理的抓取工具。

## 当前实现概览

当前仓库是前后端分离模式：

- Go 服务端（命令：`yamdc run` / `yamdc server`）
- Next.js WebUI（目录：`web/`）

其中：

- `run`：执行一次完整扫描与抓取流程（单次任务）
- `server`：启动 HTTP API 服务，供 WebUI 操作扫描、抓取、Review、入库、媒体库管理

## 使用方式

推荐使用 Docker 运行。非 Linux 环境下，部分特性可能不可用。

### 本地构建与运行

使用 `go install` 安装：

```bash
go install github.com/xxxsen/yamdc/cmd/yamdc@latest
```

安装后可直接运行：

```bash
# 单次抓取模式
yamdc run --config=./config.json

# 服务端模式（WebUI 需要）
yamdc server --config=./config.json
```

或使用源码构建：

```bash
make build
```

构建后可使用以下任意方式运行：

```bash
./yamdc run --config=./config.json
./yamdc server --config=./config.json
```

运行单元测试：

```bash
make test
```

### Docker 运行（服务端 + WebUI）

仓库内已经提供可直接参考的编排文件：`docker/docker-compose.yml`。

该编排是三容器模式：

- `yamdc-backend`：Go 服务端（`server --config=/config/config.json`）
- `yamdc-web`：Next.js 前端（通过 `NEXT_PUBLIC_API_BASE_URL` 指向后端）
- `yamdc-gateway`：Nginx 网关（统一对外端口）

核心配置点：

```yaml
services:
  yamdc-backend:
    command: ["server", "--config", "/config/config.json"]

  yamdc-web:
    environment:
      NEXT_PUBLIC_API_BASE_URL: http://yamdc-backend:8080
```

对应 `config.json`（服务端模式需包含 `library_dir`）：

```json
{
  "scan_dir": "/scandir",
  "save_dir": "/savedir",
  "library_dir": "/librarydir",
  "data_dir": "/datadir",
  "naming": "{YEAR}/{MOVIEID}"
}
```

说明：

- `YAMDC_SERVER_ADDR` 是后端服务监听地址环境变量（默认 `:8080`，可不显式配置）。
- `NEXT_PUBLIC_API_BASE_URL` 是前端构建/运行时访问后端 API 的地址；在 Docker 内通常应指向后端服务名（如 `http://yamdc-backend:8080`）。

启动：

```bash
cd docker
docker compose up -d --build
```

## WebUI

本地启动方式：

```bash
go build -o ./yamdc ./cmd/yamdc
YAMDC_SERVER_ADDR=:8080 ./yamdc server --config=./config.json
```

```bash
cd web
npm install
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080 npm run dev
```

启动后访问（常用页面）：

- `http://127.0.0.1:3000/processing`
- `http://127.0.0.1:3000/review`
- `http://127.0.0.1:3000/library`
- `http://127.0.0.1:3000/media-library`
- `http://127.0.0.1:3000/debug`

WebUI 主流程：

1. 扫描 `scan_dir`
2. 生成待处理任务
3. 手动触发单任务抓取
4. 在 Review 页面修正元数据
5. 点击入库，写入 `save_dir`
6. 通过媒体库页面管理入库结果

## 基础配置

### 最小配置（`run` 模式）

```json
{
  "scan_dir": "/dir/to/scan",
  "save_dir": "/dir/to/save/scraped/data",
  "data_dir": "/dir/to/save/models/and/cache",
  "naming": "{YEAR}/{MOVIEID}"
}
```

### 服务端配置（`server` 模式）

```json
{
  "scan_dir": "/dir/to/scan",
  "save_dir": "/dir/to/save/scraped/data",
  "library_dir": "/dir/to/library",
  "data_dir": "/dir/to/save/models/and/cache",
  "naming": "{YEAR}/{MOVIEID}"
}
```

| 配置项 | 说明 |
|---|---|
| scan_dir | 扫描目录，程序会扫描该目录并对其中影片进行抓取 |
| save_dir | 保存目录，抓取成功后按 `naming` 规则命名 |
| library_dir | 媒体库目录（仅 `server` 模式必填） |
| data_dir | 数据目录，存储中间文件、缓存、模型 |
| naming | 命名规则，可用标签：`{DATE}`, `{YEAR}`, `{MONTH}`, `{MOVIEID}`, `{NUMBER}`, `{ACTOR}`, `{TITLE}`, `{TITLE_TRANSLATED}` |

> NOTE: `MOVIEID` 是文档主推写法，`NUMBER` 作为兼容别名保留，两者值相同。`ACTOR/TITLE/TITLE_TRANSLATED` 可能包含特殊字符或长度超限，不推荐直接用于目录名。

## 可选 Repo 配置

`plugin repo` 和 `script repo` 现在都是可选项。

- 未配置 `searcher_plugin_config.sources`：
  - 服务会正常启动
  - 不会加载插件 bundle
  - 启动日志会提醒你配置自己的 plugin repo
- 未配置 `movieid_ruleset_config`：
  - 服务会正常启动
  - 会退化为 `PassthroughCleaner`
  - 启动日志会提醒你配置自己的 script repo

如果你希望启用自定义搜索插件和影片 ID 清洗规则，需要在 `config.json` 中显式配置它们。

### 配置自己的 Script Repo

`script repo` 用来提供影片 ID 清洗规则。支持：

- 本地目录
- 远程 GitHub 仓库（按 tag 下载 bundle）

本地目录示例：

```json
{
  "movieid_ruleset_config": {
    "source_type": "local",
    "location": "/path/to/your-script-repo"
  }
}
```

远程仓库示例：

```json
{
  "movieid_ruleset_config": {
    "source_type": "remote",
    "location": "https://github.com/yourname/your-yamdc-script-repo"
  }
}
```

最小目录结构：

```text
your-yamdc-script-repo/
  manifest.yaml
  ruleset/
    001-base.yaml
    002-matchers.yaml
```

`manifest.yaml` 示例：

```yaml
entry: ruleset
```

规则文件格式可参考仓库内：

- [docs/004-movieid-ruleset/design.md](/home/sen/work/yamdc/docs/004-movieid-ruleset/design.md)
- [docs/004-movieid-ruleset/example/README.md](/home/sen/work/yamdc/docs/004-movieid-ruleset/example/README.md)

### 配置自己的 Plugin Repo

`plugin repo` 用来提供搜索插件 bundle。支持：

- 本地目录
- 远程 GitHub 仓库（按 tag 下载 bundle）

本地目录示例：

```json
{
  "searcher_plugin_config": {
    "sources": [
      {
        "source_type": "local",
        "location": "/path/to/your-plugin-repo"
      }
    ]
  }
}
```

远程仓库示例：

```json
{
  "searcher_plugin_config": {
    "sources": [
      {
        "source_type": "remote",
        "location": "https://github.com/yourname/your-yamdc-plugin-repo"
      }
    ]
  }
}
```

最小目录结构：

```text
your-yamdc-plugin-repo/
  manifest.yaml
  plugins/
    alpha.yaml
```

`manifest.yaml` 示例：

```yaml
version: 1
name: my-plugin-bundle
entry: plugins
chains:
  all:
    - name: alpha
      priority: 100
```

插件 YAML 格式和 bundle 规则可参考：

- [docs/003-searcher-plugin-bundle/design.md](/home/sen/work/yamdc/docs/003-searcher-plugin-bundle/design.md)
- [docs/002-searcher-yaml-plugin-system/design.md](/home/sen/work/yamdc/docs/002-searcher-yaml-plugin-system/design.md)
- [docs/002-searcher-yaml-plugin-system/example/README.md](/home/sen/work/yamdc/docs/002-searcher-yaml-plugin-system/example/README.md)

## 文件名后缀扩展能力

工具不会强制清洗影片 ID（不同来源命名差异较大），建议用户按需重命名。

支持通过影片 ID 后缀实现“额外分类/封面水印”等能力（后缀可组合，顺序不限）：

| 后缀 | 举例 | 说明 |
|---|---|---|
| `-CD{Number}` | `-CD1` | 多 CD 场景下指定当前影片对应 CD ID（从 1 开始） |
| `-C` | `-` | 标记为含字幕轨版本，添加相应分类并为封面附加水印 |
| `-4K` | `-` | 添加“4K”分类并为封面附加水印 |
| `-8K` | `-` | 添加 8K 水印 |
| `-VR` | `-` | 添加 VR 水印 |

## 其他配置

### 标签自动映射和父级标签自动补全

功能：当检测到某个标签（或其别名）时，自动完成：

1. 别名映射到标准标签
2. 递归补全父级标准标签

开启方式（`handler_config.tag_mapper`）：

```json
{
  "handler_config": {
    "tag_mapper": {
      "disable": false,
      "args": {
        "file_path": "/path/to/your/tagconfig/tags.json"
      }
    }
  }
}
```

完整说明与示例见：`docs/001-标签自动映射与父级标签自动补全.md`。

### 网络问题

如果访问海外站点出现超时/请求失败，可设置代理：

```json
{
  "network_config": {
    "proxy": "socks5://1.2.3.4:1080",
    "timeout": 60
  }
}
```

### AI 能力

目前支持使用 AI 进行：

- 标签提取（基于标题、简介提取额外标签）
- 文本翻译（替换谷歌翻译）

`gemini` 示例：

```json
{
  "ai_engine": {
    "name": "gemini",
    "args": {
      "model": "gemini-2.0-flash",
      "key": "fill with your key here"
    }
  }
}
```

`ollama` 示例：

```json
{
  "ai_engine": {
    "name": "ollama",
    "args": {
      "host": "https://ollama.abc.com",
      "model": "gemma2:2b"
    }
  }
}
```

### Cloudflare 绕过

部分站点开启了 Cloudflare 反爬，可通过 `byparr` 配置：

```json
{
  "flare_solverr_config": {
    "enable": true,
    "host": "http://127.0.0.1:8191"
  }
}
```
