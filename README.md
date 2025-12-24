yamdc
===

由于原先的MovieDataCapture作者把工具闭源了, 只能自己写一个了。

**默默用着就行，不需要宣传/推广，甚至也不需要star。**

## 使用方式

推荐使用docker运行。 在非linux环境下运行, 部分特性无法启用。

### 本地构建与运行

使用 go install 安装：

```bash
go install github.com/xxxsen/yamdc/cmd/yamdc@latest
```

安装后可直接运行：

```bash
yamdc --config=./config.json
yamdc run --config=./config.json
```

或使用源码构建：

```bash
make build
```

构建后可使用以下任意方式运行：

```bash
./yamdc --config=./config.json
./yamdc run --config=./config.json
```

运行单元测试：

```bash
make test
```

### docker运行

使用docker运行, 对应的`docker-compose.yml`参考下面文件

//docker-compose.yml

```yaml
version: "3.1"
services:
  yamdc:
    image: xxxsen/yamdc:latest
    container_name: yamdc
    user: "1000:1000" #指定uid/gid, 根据需要修改
    volumes:
      - /data/scrape/scandir:/scandir
      - /data/scrape/savedir:/savedir
      - /data/scrape/datadir:/datadir
      - /data/config:/config
    command: --config=/config/config.json
```

程序的配置文件如下

//config.json

```json
{
    "scan_dir": "/scandir",
    "save_dir": "/savedir",
    "data_dir": "/datadir",
    "naming": "{YEAR}/{NUMBER}"
}
```

需要挂载扫描目录(/scandir), 存储目录(/savedir), 数据目录(/datadir)和配置目录(/config), 这几个目录在自己的配置文件中指定。

配置完成后, 使用`docker compose up` 进行刮削, 刮削完成的电影会被存储到`/data/scrape/savedir`下。

## 基础配置

```json
{
    "scan_dir": "/dir/to/scan",
    "save_dir": "/dir/to/save/scraped/data",
    "data_dir": "/dir/to/save/models/and/cache",
    "naming": "naming rule, specify naming rule you want, example: {YEAR}/{NUMBER}"
}
```

|配置项|说明|
|---|---|
|scan_dir|扫描目录, 程序会扫描该目录并对其中的影片进行刮削|
|save_dir|保存目录, 刮削成功的电影会被移动到该目录, 并按`naming`指定的命名规则进行命名|
|data_dir|数据目录, 存储中间文件或者模型文件的|
|naming|命名规则, 可用的命名标签如下:{DATE}, {YEAR}, {MONTH}, {NUMBER}, {ACTOR}, {TITLE}, {TITLE_TRANSLATED}|

**NOTE: naming方式, 虽然提供了ACTOR/TITLE/TITLE_TRANSLATED, 但是并不推荐使用(可能会因为包含特殊字符或者长度超限制导致创建目录失败)。**

工具并不会对番号进行清洗(各种奇奇怪怪的下载站都有自己的命名方式, 无脑清洗可能会导致得到预期外的番号), 用户自己需要对文件进行重命名。

当前支持给番号添加特定来后缀来实现`添加额外分类`, `添加特定水印`等能力。

支持的后缀列表及说明(不同的后缀没有顺序限制, 可以同时存在多种后缀):

|后缀|举例|说明|
|---|---|---|
|-CD{Number}|-CD1|多CD场景下, 指定当前影片对应的CD ID, 起始CD为1|
|-C|-|添加`字幕`到分类中并为封面添加水印|
|-4K|-|添加`4K`到分类中并为封面添加水印|
|-8K|-|添加8K水印|
|-LEAK|-|为封面添加特定水印| 
|-VR|-|添加vr水印|
|-UC, -U|-|添加破解水印|

## 其他配置
### 标签自动映射和父级标签自动补全
功能：当当检测到某个标签（或其别名）时，自动完成两项核心操作：
1. 将别名标签映射为标准标签
2. 自动向上递归添加所有父级标准标签

开启该功能：
```json
{
  "scan_dir": "...",
  "save_dir": "...",
  "data_dir": "...",
  "naming": "...",

  "handler_config": {
    "tag_mapper": {
      "disabled": false,
      "args": {
        "file_path": "/path/to/your/tagconfig/tags.json"
      }
    }
  }
}
```
同时需要创建一个`tags.json`文件,tags.json 采用层级嵌套结构，核心字段说明如下：

| 字段/配置形式            | 作用 | 示例|
|--------------------|-|-|
| `alias` 数组（仅对象内有效） |配置当前标准标签的别名，检测到别名时自动映射为当前标准标签|"cosplay" 的 _alias: ["cos", "角色扮演"]|
| `children`         |定义子级标准标签，形成标签层级关系，子标签命中时自动携带父标签|"cosplay" 下嵌套 "原神" 对象|
|`name`|定义当前标准标签的名称，用于匹配和映射|"cosplay" 是标准标签|

参考配置示例：

```json
[
  {
    "name": "cosplay",
    "alias": ["cos", "角色扮演"],
    "children": [
      {
        "name": "原神",
        "alias": ["Genshin", "⚪神"],
        "children": [
          {
            "name": "芭芭拉·佩奇",
            "alias": ["芭芭拉", "Barbara Pegg", "Barbara"]
          },
          {
            "name": "莫娜",
            "alias": ["Mona"]
          }
        ]
      }
    ]
  }
]

```



效果：

| 输入标签             | 预期输出标签                      | 说明                                 |
| ---------------- | --------------------------- | ---------------------------------- |
| "cos"            | ["cosplay"]                 | 匹配 cosplay 的别名，映射后无父级，直接输出         |
| "Genshin"        | ["cosplay", "原神"]           | 匹配原神的别名，自动补全父标签 cosplay            |
| "Barbara"        | ["cosplay", "原神", "芭芭拉・佩奇"] | 匹配芭芭拉·佩奇的别名，补全父标签原神 + 祖父标签 cosplay |
| ["角色扮演", "Mona"] | ["cosplay", "原神", "莫娜"]     | 多标签输入，去重后补全对应父级                    |
| "莫娜"             | ["cosplay", "原神", "莫娜"]     | 直接输入标准子标签，补全所有父级                   |




## 其他

### 性能问题

上面的docker-compose.yml的例子将扫描、存储、数据目录分别挂载在3个不同的目录下, 这在docker中会导致golang的os.Rename执行失败, 导致使用低效率的复制方式, 为了避免这个问题, 可以考虑将`/data/scrape`直接挂载到一个目录中, 来避免跨设备复制问题。

```yml
version: "3.1"
    ...
    volumes:
      - /data/scrape:/scrape 
```

**NOTE: 假定/data/scrape下已经有scandir, savedir, datadir 3个目录**

对应的配置文件修改

```json
{
    ...
    "scan_dir": "/scrape/scandir",
    "save_dir": "/scrape/savedir",
    "data_dir": "/scrape/datadir",
    ...
}
```

### 网络问题

众所周知, 国内的网络在访问某些国外的站点会有问题, 如果程序运行过程中出现各种超时/请求失败问题, 可以考虑配置下代理, 参考如下配置:

```json
{
    "scan_dir": "...",
    "network_config": {  //与scan_dir, save_dir, data_dir 这些配置在同级
        "proxy": "socks5://1.2.3.4:1080", //设置socks5代理, 仅支持http/socks5
        "timeout": 60     //设置超时时间, 单位为秒
    }
}
```

### AI能力

目前支持使用AI来提供**标签提取**, **文本翻译**的能力。

- 标签提取: 使用当前已有的标题、简介额外提取5个标签
- 文本翻译: 用于替换谷歌翻译

开启的方式如下:

```json
{
    "scan_dir": "...",
    "ai_engine": {
        "name": "gemini", 
        "args": {
            "model": "gemini-2.0-flash", //按需填写, 仅测试2.0-flash, 其他的没测试
            "key": "fill with your key here" //从这里获取 https://aistudio.google.com/app/apikey
        }
    }
    //other config...
}
```

或者使用本地的 Ollama 服务：

```json
{
    "scan_dir": "...",
    "ai_engine": {
        "name": "ollama",
        "args": {
            "host": "https://ollama.abc.com", //Ollama API 地址，替换成你自建的地址
            "model": "gemma2:2b"        //替换为本地已有的模型名称
        }
    }
    //other config...
}
```

### cloudflare绕过

部分网站会开启cloudflare的反爬虫能力, 目前支持使用[`byparr`](https://github.com/ThePhaseless/Byparr)进行绕过, 如果已经部署了相关的服务, 可以在配置种开启下面的选项来支持。

```json
{
    "scan_dir": "...",
    "flare_solverr_config": {
        "enable": true,
        "host": "http://127.0.0.1:8191", //替换成具体的地址
        "domains": {
            "abc.com": true  //这里填写要使用flare_solverr的域名。启动的时候, 会打印插件当前的域名列表, 如果某个域名需要绕过cloudflare则加到这里即可。
        }
    }
}
```
