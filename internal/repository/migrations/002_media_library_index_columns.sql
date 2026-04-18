-- 1.4: 给 yamdc_media_library_tab 补一组便于过滤/排序的专用列,
-- 让 ListItems 可以把 keyword/year/size/sort 下推到 SQL, 不再回到 Go
-- 里一把 SELECT + 全表 json.Unmarshal + 逐行过滤。
--
-- 写入路径 (upsertDetail) 会从 Item 里填这些列, 旧数据这里不做 SQL 回填,
-- 升级后需要用户手动触发一次 "同步媒体库" 让 upsertDetail 把这些列写正确,
-- 否则历史行的 number / name / release_year / total_size 仍是默认零值,
-- ListItems 的 keyword / year / size 过滤会漏命中它们。
ALTER TABLE yamdc_media_library_tab ADD COLUMN number TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN name TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN release_year TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN total_size INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_number ON yamdc_media_library_tab(number);

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_release_year ON yamdc_media_library_tab(release_year);

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_total_size ON yamdc_media_library_tab(total_size);
