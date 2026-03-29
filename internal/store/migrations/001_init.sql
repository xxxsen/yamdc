CREATE TABLE IF NOT EXISTS cache_tab (
    key TEXT PRIMARY KEY,
    value BLOB,
    expire_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_expireat ON cache_tab(expire_at);
