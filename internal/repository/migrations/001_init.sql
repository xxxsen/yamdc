CREATE TABLE IF NOT EXISTS yamdc_job_tab (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_uid TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    file_ext TEXT NOT NULL,
    conflict_key TEXT NOT NULL DEFAULT '',
    rel_path TEXT NOT NULL UNIQUE,
    abs_path TEXT NOT NULL,
    number TEXT NOT NULL,
    raw_number TEXT NOT NULL DEFAULT '',
    cleaned_number TEXT NOT NULL DEFAULT '',
    number_source TEXT NOT NULL DEFAULT 'raw',
    number_clean_status TEXT NOT NULL DEFAULT '',
    number_clean_confidence TEXT NOT NULL DEFAULT '',
    number_clean_warnings TEXT NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    error_msg TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    scrape_started_at INTEGER NOT NULL DEFAULT 0,
    scrape_finished_at INTEGER NOT NULL DEFAULT 0,
    reviewed_at INTEGER NOT NULL DEFAULT 0,
    imported_at INTEGER NOT NULL DEFAULT 0,
    deleted_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_yamdc_job_status ON yamdc_job_tab(status);
CREATE INDEX IF NOT EXISTS idx_yamdc_job_updated_at ON yamdc_job_tab(updated_at);
CREATE INDEX IF NOT EXISTS idx_yamdc_job_conflict_key_active ON yamdc_job_tab(conflict_key) WHERE deleted_at = 0 AND status != 'done' AND conflict_key != '';

-- 注: 历史上这里还会 CREATE TABLE yamdc_log_tab (scrape job 专用日志表),
-- 1.4 已经把日志表合到 yamdc_unified_log_tab, 定义搬到 003 migration,
-- 所以 001 不再创建 yamdc_log_tab。对早就装过 001 / 002 的老库, 003 会
-- 额外跑一条 "DROP TABLE IF EXISTS yamdc_log_tab" 把遗留表清掉。
CREATE TABLE IF NOT EXISTS yamdc_scrape_data_tab (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL UNIQUE,
    source TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    raw_data TEXT NOT NULL DEFAULT '',
    review_data TEXT NOT NULL DEFAULT '',
    final_data TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS yamdc_media_library_tab (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rel_path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0,
    poster_path TEXT NOT NULL DEFAULT '',
    cover_path TEXT NOT NULL DEFAULT '',
    item_json TEXT NOT NULL DEFAULT '',
    detail_json TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_yamdc_media_library_updated_at ON yamdc_media_library_tab(updated_at);

CREATE TABLE IF NOT EXISTS yamdc_task_state_tab (
    task_key TEXT PRIMARY KEY,
    status TEXT NOT NULL DEFAULT '',
    total INTEGER NOT NULL DEFAULT 0,
    processed INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    conflict_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    current TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    started_at INTEGER NOT NULL DEFAULT 0,
    finished_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);
