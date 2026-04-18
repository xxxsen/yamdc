-- 1.4: 统一日志表 + 配套的 app 级 kv 存储。
--
-- 合表动机:
--   (1) 后续再加新的日志来源 (例如媒体库同步事件、配置变更审计) 不需要再开新表,
--       schema 统一成 {log_type, task_id, level, msg, created_at}, 前端也只要
--       一套 list API;
--   (2) retention / 清理逻辑只写一份 (DELETE WHERE created_at < ?), 避免每张表
--       都手搓一遍 cleanup;
--   (3) 现有 scrape 日志的 "stage / message / detail" 统一压进 msg (JSON),
--       表结构不再被具体 log_type 绑死。
--
-- 新表叫 yamdc_unified_log_tab 而不是 yamdc_log_tab 是刻意的:
--   老分支里的 yamdc_log_tab 在 001 migration 里已经长成 scrape 专用 schema,
--   同名 "先 DROP 再 CREATE" 的做法读起来像一个 "每次启动都会丢数据" 的陷阱
--   (虽然 user_version 保证不会), 容易让后来翻代码的人误判。换个名字彻底
--   避开这层认知负担, 代价只是多一条 DROP IF EXISTS 清理遗留。
--
-- 字段约定:
--   log_type: 来源枚举, 目前 'scrape_job' | 'media_library_sync', 以后可扩;
--   task_id:  归属任务的标识, TEXT 方便混 int/str:
--             - scrape_job:         CAST(job_id AS TEXT)
--             - media_library_sync: sync 运行一次对应一个 run_id 原文
--             为 '' 表示全局日志, 目前还没用到;
--   level:    'info' | 'warn' | 'error', 对齐前端展示的三档, 够用;
--   msg:      永远存 JSON (payload 包一层), 由 log_type 决定内部 schema:
--             - scrape_job          {"stage","message","detail"}
--             - media_library_sync  {"rel_path","message"}
--             新增 log_type 自由扩字段、不改表;
--   created_at: unix ms, 按它做时间索引 + retention DELETE。命名上对齐项目
--             里其他表 (yamdc_scrape_data_tab.created_at / yamdc_media_library_tab.
--             created_at), 避免 ctime 这种 unix 风格缩写和其余字段格格不入。
--
-- 老数据处理:
--   直接 DROP 原 yamdc_log_tab 表, 不迁移。retention 是 7 天, 老日志搬过来
--   也会很快被裁; 写 json_object() INSERT SELECT 的迁移 SQL 反而多一个出错面,
--   权衡之后选 "旧日志一刀切", 用户从干净的新表起步。
--
-- yamdc_kv_tab 是配套的 app 级 key-value 存储, 目前存 media library sync dirty
-- flag, 后续 app 级 boolean/counter 都可以复用这张表。
DROP INDEX IF EXISTS idx_yamdc_log_job_id_created_at;
DROP TABLE IF EXISTS yamdc_log_tab;

CREATE TABLE IF NOT EXISTS yamdc_unified_log_tab (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    log_type   TEXT NOT NULL,
    task_id    TEXT NOT NULL DEFAULT '',
    level      TEXT NOT NULL,
    msg        TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_yamdc_unified_log_type_task_created_at
    ON yamdc_unified_log_tab(log_type, task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_yamdc_unified_log_type_created_at
    ON yamdc_unified_log_tab(log_type, created_at);

CREATE TABLE IF NOT EXISTS yamdc_kv_tab (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);
