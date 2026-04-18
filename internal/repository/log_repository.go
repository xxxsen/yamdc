package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LogType 枚举. 每个值必须和 migration 003 里 log_type 列注释里列出的名字对齐,
// 否则查询会落空。新增值时记得同步前端 list API 的 log_type 白名单,
// 以及 DB 注释。
const (
	LogTypeScrapeJob        = "scrape_job"
	LogTypeMediaLibrarySync = "media_library_sync"
)

// LogOrderAsc / LogOrderDesc 是 List 参数里 Order 字段的合法取值。
// 保持 string 是为了让前端 / web handler 直接传字串过来, 不需要额外映射。
const (
	LogOrderAsc  = "asc"
	LogOrderDesc = "desc"
)

// LogEntry 是 yamdc_unified_log_tab 的行模型, 字段一一对应迁移脚本里的列。
// Msg 存的是 JSON 字符串, 具体 schema 由 LogType 决定, 解析放到调用方
// (job / medialib 各自知道自己的 payload 格式, repo 层不做 decode 以免
// 引入向上层业务的依赖)。
type LogEntry struct {
	ID        int64  `json:"id"`
	LogType   string `json:"log_type"`
	TaskID    string `json:"task_id"`
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	CreatedAt int64  `json:"created_at"`
}

// LogListFilter 是 List 的过滤参数。所有字段都是可选 (除 LogType 外):
//
//   - LogType 为 "" 时返回空结果, 这是刻意设计: 前端不应当在一个弹窗里
//     混合不同来源的日志, 强制传 LogType 避免误用。
//   - TaskID 为 "" 时不按任务过滤 (返回整个 log_type 的日志), 用于
//     sync 日志这种按时间轴展示的场景。
//   - Limit <= 0 使用默认 500; 上限 1000 (防御性, 避免前端传异常值
//     一次性把内存打爆)。
//   - Order 支持 "asc"/"desc", 其它值视为 "asc" (scrape 日志默认按时间
//     正序展示更自然)。
type LogListFilter struct {
	LogType string
	TaskID  string
	Limit   int
	Order   string
}

// logListDefaultLimit / logListMaxLimit 把 "给 SQL LIMIT 传多少" 单独抽出常量,
// 方便调参, 也让 List 的判断逻辑一眼看懂。
const (
	logListDefaultLimit = 500
	logListMaxLimit     = 1000
)

type LogRepository struct {
	db *sql.DB
}

func NewLogRepository(db *sql.DB) *LogRepository {
	return &LogRepository{db: db}
}

// Append 往 yamdc_unified_log_tab 插一条日志。created_at 用服务端当前毫秒
// 时间, 不开放给调用方是因为让所有日志都走一套时钟, 避免 "测试里注入时间,
// 线上时间漂" 这种调试陷阱。
//
// msg 必须是完整的 JSON 字符串 (payload 包一层), 由调用方 marshal;
// repo 层不参与 JSON 编码是刻意的: 不同 log_type 的 payload 结构不一样,
// 在业务包里 marshal 比在 repo 层做 generic 反射干净。
func (r *LogRepository) Append(ctx context.Context, logType, taskID, level, msg string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO yamdc_unified_log_tab (log_type, task_id, level, msg, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, logType, taskID, level, msg, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("insert log failed: %w", err)
	}
	return nil
}

// List 返回命中过滤条件的日志行。空 LogType 直接走短路返回空切片, 避免退化
// 成全表扫 (调用方通常是 UI 弹窗, 总是知道自己在看哪类日志)。
func (r *LogRepository) List(ctx context.Context, filter LogListFilter) ([]LogEntry, error) {
	if filter.LogType == "" {
		return []LogEntry{}, nil
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = logListDefaultLimit
	}
	if limit > logListMaxLimit {
		limit = logListMaxLimit
	}
	order := "ASC"
	if strings.EqualFold(filter.Order, LogOrderDesc) {
		order = "DESC"
	}
	var (
		query strings.Builder
		args  = make([]any, 0, 3)
	)
	query.WriteString(`SELECT id, log_type, task_id, level, msg, created_at FROM yamdc_unified_log_tab WHERE log_type = ?`)
	args = append(args, filter.LogType)
	if filter.TaskID != "" {
		query.WriteString(` AND task_id = ?`)
		args = append(args, filter.TaskID)
	}
	// 用 id 做 tie-breaker, 保证同一 created_at 下的多条日志顺序稳定
	// (自增 id 等价于到达顺序)。
	fmt.Fprintf(&query, ` ORDER BY created_at %s, id %s LIMIT ?`, order, order)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list logs failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	entries := make([]LogEntry, 0, limit)
	for rows.Next() {
		var entry LogEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.LogType,
			&entry.TaskID,
			&entry.Level,
			&entry.Msg,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan log failed: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate logs failed: %w", err)
	}
	return entries, nil
}

// DeleteByTask 删除某个任务的所有日志, 目前仅用于 job 硬删除时连带清理。
// retention 路径不用这个, 走 DeleteOlderThan 更高效 (单条 DELETE
// 扫索引范围)。
func (r *LogRepository) DeleteByTask(ctx context.Context, logType, taskID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM yamdc_unified_log_tab WHERE log_type = ? AND task_id = ?`,
		logType, taskID,
	)
	if err != nil {
		return fmt.Errorf("delete logs by task failed: %w", err)
	}
	return nil
}

// DeleteOlderThan 是 retention 清理主入口, 按 created_at 裁掉超期日志。
// 走 idx_yamdc_unified_log_type_created_at 索引的时候 SQLite 能精确扫裁剪
// 区间, 即便表规模到百万也只付出裁剪区间的 IO。调用方决定 cutoff 策略
// (目前统一 7 天), 这里只负责执行。
func (r *LogRepository) DeleteOlderThan(ctx context.Context, cutoffMs int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM yamdc_unified_log_tab WHERE created_at < ?`, cutoffMs)
	if err != nil {
		return fmt.Errorf("cleanup logs failed: %w", err)
	}
	return nil
}
