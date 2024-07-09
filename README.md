yamdc
===

由于原先的MovieDataCapture作者把工具闭源了, 只能自己写一个了。

## 使用方式

使用docker进行部署, 对应的`docker-compose.yml`参考下面文件

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

**NOTE: 程序依赖go-face进行人脸识别, 以用于识别图片中的人脸并进行截图, 这个库需要有对应的模型文件, 可以通过项目目录下的`scripts/download_models.sh`进行下载, 程序会将模型下载到`models`目录, 之后将该目录移动到数据目录(`/datadir`)下即可**

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
|naming|命名规则, 可用的命名标签如下:{DATE}, {YEAR}, {MONTH}, {NUMBER}, {ACTOR}|


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
