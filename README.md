yamdc
===

由于原先的MovieDataCapture作者把工具闭源了, 只能自己写一个了。

## 使用方式

推荐使用docker运行。 在非linux环境下运行, 部分特性无法启用。

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

**NOTE: 程序依赖go-face进行人脸识别, 以用于识别图片中的人脸并进行截图, 这个库需要有对应的模型文件, 程序启动的时候, 会检测模型文件是否存在, 如果不存在, 则会自动下载模型文件到`数据目录`下**

### 手动编译

程序编译需要go环境, 版本需要>=**1.21**, 请自行安装。

#### 依赖相关

linux下编译, 需要安装相关的依赖, 可以使用下面的命令进行安装

```shell
# Ubuntu
sudo apt-get install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg-turbo8-dev gfortran
# Debian
sudo apt-get install libdlib-dev libblas-dev libatlas-base-dev liblapack-dev libjpeg62-turbo-dev gfortran
# 其他
# 我也不知道其他发行版对应的依赖名是啥...
```

**NOTE: 如果不是编译linux下的可执行文件则可以跳过安装依赖部分, 当然, 缺少依赖会导致后面的人脸识别特性无法开启。**

#### 编译&运行

```shell
CGO_LDFLAGS="-static" CGO_ENABLED=1 go build -a -tags netgo -ldflags '-w' -o yamdc ./
```

编译完成后, 会在目录下生成对应的可执行文件(windows用户需要重命名下, 给可执行文件增加`.exe` 后缀。)

之后执行下面命令运行即可。

```shell
# --config指定配置文件位置, 详细配置参考后续章节。
./yamdc --config=./config.json
```

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
|-LEAK|-|为封面添加特定水印| 

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

开启的方式如下

```json
{
    "scan_dir": "...",
    "ai_engine": {
        "name": "gemini", //当前仅支持gemini, 不填则不开启
        "args": {
            "model": "gemini-2.0-flash", //按需填写, 仅测试2.0-flash, 其他的没测试
            "key": "fill with your key here" //从这里获取 https://aistudio.google.com/app/apikey
        }
    }
    //other config...
}
```