# Pebble Cache Store 设计文档

## 一、文档目的

本文档描述当前 cache store 从 SQLite 迁移到 Pebble 后的正式设计、持久化语义与实现边界。

本设计用于说明：

1. cache 数据在 Pebble 中的 key/value 编码方式
2. 过期清理如何避免误删新值
3. 服务 crash 时的持久化与一致性预期
4. 当前方案不承诺解决的问题

## 二、设计目标

本方案的目标是：

1. 提供本地持久化 cache store
2. 支持按 key 读写二进制 value
3. 支持每条记录独立过期时间
4. 支持后台批量清理过期数据
5. 在服务 crash 后尽量保留已成功提交的 cache 写入
6. 保持 cache 可重建属性，不把 cache store 当作主数据源

## 三、非目标

本方案不做：

1. 不提供跨进程并发写入同一个 Pebble 目录的能力
2. 不提供分布式复制或多副本容灾
3. 不把 cache 写入作为业务事务的一部分
4. 不保证 `Commit` 返回前的写入在 crash 后一定可见
5. 不修复底层磁盘、文件系统、虚拟化层违反 `fsync` 语义导致的问题

## 四、存储位置

Pebble cache 默认存储在：

```text
${data_dir}/cache/pebble
```

路径由以下函数生成：

```text
internal/store/pebble_storage.go
PebblePathForDataDir(dataDir string)
```

启动时通过 bootstrap 构建 cache store：

```text
internal/bootstrap/infra/storage.go
```

## 五、Key 空间设计

Pebble 中使用两个前缀区分数据记录与过期索引。

```text
d<raw-key>                  -> data record
e<expire-at-sec><raw-key>   -> expire index
```

字段说明：

1. `d` 表示真实 cache 数据
2. `e` 表示按过期时间排序的索引
3. `expire-at-sec` 使用 8 字节 big-endian Unix 秒级时间戳
4. big-endian 编码保证过期索引可以按字节序自然排序

## 六、Value 编码

data record 的 value 格式：

```text
<expire-at-sec><raw-value>
```

字段说明：

1. 前 8 字节为 Unix 秒级过期时间
2. 后续字节为调用方写入的原始 value
3. 读取时先解码过期时间，再判断是否已过期

设计理由：

1. 读单个 key 时不需要额外查 expire index
2. cleanup 能通过 data record 的过期时间判断 expire index 是否仍然指向当前版本
3. 覆盖写同一个 key 时，即使旧 expire index 残留，也不会误删新 value
4. cache 过期不需要亚秒级精度，秒级时间戳与历史 SQLite cache 行为保持一致

## 七、写入流程

写入 `PutData(ctx, key, value, expire)` 时：

1. 归一化 expire
2. 生成新的 `expireAt`
3. 查询当前 data record 的旧 `expireAt`
4. 如果存在旧记录，删除旧 expire index
5. 写入新的 data record
6. 写入新的 expire index
7. 使用 `batch.Commit(pebble.Sync)` 提交

同一个 batch 中包含 data record 与 expire index 的变更，避免一条记录被拆成多个独立提交。

## 八、读取流程

读取 `GetData(ctx, key)` 时：

1. 按 `d<raw-key>` 查询 data record
2. 解码 `expireAt` 与 value
3. 如果未过期，返回 value
4. 如果已过期，尝试删除 data record 与对应 expire index，并返回过期错误

`IsDataExist(ctx, key)` 使用同样的过期判断逻辑，但只返回存在性。

## 九、过期清理

`CleanupExpired(ctx)` 使用 expire index 做有界扫描：

1. 只扫描 `e` 前缀范围
2. 按 expire index 的字节序从早到晚遍历
3. 遇到 `expireAt > now` 后停止
4. 对每条过期索引，查询当前 data record 的 `expireAt`
5. 只有当前 data record 的 `expireAt` 与 expire index 一致时，才删除 data record
6. 无论 data record 是否匹配，都会删除已过期的 expire index
7. 每批最多提交 `1024` 个删除操作

这个比对机制用于处理覆盖写竞态：

```text
t1: key=k 写入 value=old, expireAt=100
t2: key=k 被覆盖为 value=new, expireAt=200
t3: 旧 expire index e100k 仍然存在
t4: cleanup 扫到 e100k，但 data record 中 expireAt=200
t5: cleanup 只删除 e100k，不删除 d+k
```

因此旧索引最多造成额外清理成本，不会误删新值。

## 十、Crash 语义

当前实现使用：

```text
pebble.Open(path, &pebble.Options{})
batch.Commit(pebble.Sync)
```

语义说明：

1. 默认 `Options` 没有禁用 WAL，因此 Pebble 会保留 crash recovery 能力
2. 所有产品代码路径的写入和删除最终都通过 `Commit(pebble.Sync)` 提交
3. `Sync=true` 表示提交时要求同步到磁盘，成功返回后的写入具备单次写入持久性预期
4. batch 内部的 `Set/Delete(..., pebble.NoSync)` 只是把操作加入 batch，最终持久化语义由 `Batch.Commit(pebble.Sync)` 决定

服务 crash 场景下：

1. `Commit(pebble.Sync)` 已成功返回：预期 crash 后仍可恢复
2. crash 发生在 `Commit` 返回前：该次 batch 可能生效，也可能不生效，调用方不能假设成功
3. crash 发生在读到过期记录并尝试删除时：删除可能丢失，但记录仍会在后续读取或 cleanup 中再次被识别为过期
4. crash 发生在 cleanup 批量删除过程中：已提交批次会保留，未提交批次会在下次 cleanup 重试

## 十一、数据损坏边界

当前设计不主动引入已知的数据损坏风险。

防护点：

1. 未禁用 WAL
2. 使用 `pebble.Sync` 提交写入
3. data record 与 expire index 在同一 batch 内提交
4. 读取 value 时会校验 record header 长度
5. Unix 秒级时间戳编解码会检查 `uint64 -> int64` 溢出

仍可能失败的情况：

1. 底层磁盘损坏
2. 文件系统或虚拟化层没有兑现同步写语义
3. 进程被并发启动，多个实例尝试打开同一个 Pebble 目录
4. 人工删除或篡改 Pebble 文件
5. Pebble 自身或依赖版本存在缺陷

这些情况下应把 cache 目录视为可删除重建的数据，而不是主业务状态。

## 十二、当前实现位置

核心实现：

```text
internal/store/pebble_storage.go
```

覆盖测试：

```text
internal/store/pebble_storage_test.go
```

定期 cleanup job：

```text
internal/store/cleanup_job.go
internal/bootstrap/actions_app.go
```

## 十三、运维建议

1. 不要把 `${data_dir}/cache/pebble` 放在临时目录或容易被清理的目录
2. 不要让多个 yamdc 进程共享同一个 cache Pebble 目录
3. 如果 Pebble 打开失败且确认只是 cache 损坏，可以停止服务后删除 `${data_dir}/cache/pebble` 让系统重建
4. 如果后续把 cache store 用于不可重建数据，需要重新评估备份、迁移和恢复策略
