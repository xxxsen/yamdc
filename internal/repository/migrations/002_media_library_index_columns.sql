-- 1.4: 给 yamdc_media_library_tab 补一组便于过滤/排序的专用列,
-- 让 ListItems 可以把 keyword/year/size/sort 下推到 SQL, 不再回到 Go
-- 里一把 SELECT + 全表 json.Unmarshal + 逐行过滤。
--
-- 写入路径 (upsertDetail) 会从 Item 里填这些列;
-- 旧数据通过下面的 UPDATE 用 SQLite JSON1 的 json_extract 从 item_json 回填,
-- 回填不准的行 (例如 release_date 格式非 YYYY-...) 会等下一次媒体库 sync 修正。
ALTER TABLE yamdc_media_library_tab ADD COLUMN number TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN name TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN release_year TEXT NOT NULL DEFAULT '';

ALTER TABLE yamdc_media_library_tab ADD COLUMN total_size INTEGER NOT NULL DEFAULT 0;

-- 回填: json_extract 在 item_json 为空/非 JSON 时返回 NULL, COALESCE 兜底到零值。
-- release_year 只信以 4 位数字打头的 release_date (大多数爬虫数据都是 YYYY-MM-DD),
-- 其余情况置空, 等 sync 时 Go 的 releaseYear() 再从任意位置提取 4 位数字。
UPDATE yamdc_media_library_tab
SET number = COALESCE(json_extract(item_json, '$.number'), ''),
    name = COALESCE(json_extract(item_json, '$.name'), ''),
    release_year = CASE
        WHEN substr(COALESCE(json_extract(item_json, '$.release_date'), ''), 1, 4)
             GLOB '[0-9][0-9][0-9][0-9]'
        THEN substr(COALESCE(json_extract(item_json, '$.release_date'), ''), 1, 4)
        ELSE ''
    END,
    total_size = COALESCE(json_extract(item_json, '$.total_size'), 0)
WHERE item_json != '';

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_number ON yamdc_media_library_tab(number);

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_release_year ON yamdc_media_library_tab(release_year);

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_total_size ON yamdc_media_library_tab(total_size);
